package plan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/tap"

	"github.com/yarlson/snap/internal/model"
)

// mockExecutor records all calls and returns canned responses.
type mockExecutor struct {
	mu    sync.Mutex
	calls []executorCall
	err   error // if set, all Run calls return this error
}

type executorCall struct {
	modelType model.Type
	args      []string
}

func (m *mockExecutor) Run(_ context.Context, w io.Writer, mt model.Type, args ...string) error {
	m.mu.Lock()
	m.calls = append(m.calls, executorCall{modelType: mt, args: args})
	m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	// Write a canned response so the output can be verified.
	fmt.Fprintln(w, "LLM response")
	return nil
}

func (m *mockExecutor) getCalls() []executorCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]executorCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// promptMatchExecutor returns per-prompt errors based on a substring match.
type promptMatchExecutor struct {
	failOn map[string]error // substring → error
}

func (m *promptMatchExecutor) Run(_ context.Context, w io.Writer, _ model.Type, args ...string) error {
	prompt := args[len(args)-1]
	for substr, err := range m.failOn {
		if strings.Contains(prompt, substr) {
			return err
		}
	}

	fmt.Fprintln(w, "LLM response")
	return nil
}

// testTasksMD is a minimal TASKS.md with 3 tasks for testing the full pipeline.
const testTasksMD = `# TASKS: Test

## G. Task List

| # | File | Name | Epic | Outcome | Risk | Size |
|---|------|------|------|---------|------|------|
| 0 | TASK0.md | Setup | Epic 1 | Base setup | Low | S |
| 1 | TASK1.md | Feature A | Epic 2 | Feature A works | Medium | M |
| 2 | TASK2.md | Feature B | Epic 3 | Feature B works | Low | S |

## H. Dependencies
`

// setupTasksDir creates a temp dir with a TASKS.md file for full-pipeline tests.
// Returns the path to the tasks dir.
func setupTasksDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(testTasksMD), 0o600))
	return dir
}

// --- Phase 1 tests ---

func TestPlanner_Phase1_UserMessageThenDone(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("I want OAuth2 auth\n/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First call: requirements prompt (no -c)
	require.GreaterOrEqual(t, len(calls), 2)
	assert.NotContains(t, calls[0].args, "-c")

	// Second call: user message with -c
	assert.Contains(t, calls[1].args, "-c")
	assert.Contains(t, calls[1].args[len(calls[1].args)-1], "I want OAuth2 auth")

	// Output should contain phase headers
	output := out.String()
	assert.Contains(t, output, "Gathering requirements")
	assert.Contains(t, output, "snap plan>")
}

func TestPlanner_Phase1_DoneImmediately(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First call: requirements prompt
	require.GreaterOrEqual(t, len(calls), 1)
	assert.NotContains(t, calls[0].args, "-c")

	// Phase 2 pipeline: 1 (PRD) + 2 (parallel TECH+DESIGN) + 4 (task splitting chain) + 3 (task files) = 10
	// Total: 1 (requirements prompt) + 10 = 11
	assert.Equal(t, 11, len(calls))
}

func TestPlanner_Phase1_DoneUppercase(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/DONE\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	// Should have completed both phases (1 requirements + 10 generation)
	calls := exec.getCalls()
	assert.Equal(t, 11, len(calls))
}

func TestPlanner_Phase1_EOF(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	// No /done, just EOF
	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("some requirements\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	// Should have run user message + Phase 2
	calls := exec.getCalls()
	// 1 (requirements) + 1 (user msg) + 10 (generation) = 12
	assert.Equal(t, 12, len(calls))
}

func TestPlanner_Phase1_ContextCancel(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("some input\n")),
	)

	err := p.Run(ctx)
	require.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "Planning aborted")
}

// --- Phase 2 tests ---

func TestPlanner_Phase2_Pipeline(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Requirements + 10 generation calls = 11.
	// (1 PRD + 2 parallel + 4 task splitting + 3 task files)
	require.Equal(t, 11, len(calls))

	// All Phase 2 calls should use model.Thinking.
	for i := 1; i < len(calls); i++ {
		assert.Equal(t, model.Thinking, calls[i].modelType, "call %d should use Thinking model", i)
	}

	// Call 1 (PRD): has -c (continues Phase 1 conversation).
	assert.Contains(t, calls[1].args, "-c", "PRD step should have -c")

	// Calls 2,3 (parallel TECH+DESIGN): no -c (independent processes).
	assert.NotContains(t, calls[2].args, "-c", "parallel call should not have -c")
	assert.NotContains(t, calls[3].args, "-c", "parallel call should not have -c")

	// Call 4 (create tasks): no -c (fresh conversation).
	assert.NotContains(t, calls[4].args, "-c", "create tasks should not have -c")

	// Calls 5,6,7 (assess, merge, generate): have -c (continue task splitting conversation).
	assert.Contains(t, calls[5].args, "-c", "assess tasks should have -c")
	assert.Contains(t, calls[6].args, "-c", "merge tasks should have -c")
	assert.Contains(t, calls[7].args, "-c", "generate task summary should have -c")

	// Calls 8,9,10 (task file generation): no -c (independent sub-agents).
	for i := 8; i < 11; i++ {
		assert.NotContains(t, calls[i].args, "-c", "task file call %d should not have -c", i)
	}
}

func TestPlanner_Phase2_StepHeaders_NewPipeline(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Step 1/7")
	assert.Contains(t, output, "Step 2/7")
	assert.Contains(t, output, "Step 3/7")
	assert.Contains(t, output, "Step 4/7")
	assert.Contains(t, output, "Step 5/7")
	assert.Contains(t, output, "Step 6/7")
	assert.Contains(t, output, "Step 7/7")
	assert.Contains(t, output, "Generate PRD")
	assert.Contains(t, output, "Generate technology plan + design spec")
	assert.Contains(t, output, "Create task list")
	assert.Contains(t, output, "Assess tasks")
	assert.Contains(t, output, "Refine tasks")
	assert.Contains(t, output, "Generate task summary")
	assert.Contains(t, output, "Generate task files")
}

func TestPlanner_Phase2_StepCompletions(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	// Steps 1, 3, 4, 5, 6 produce "Step complete"; step 2 produces sub-step names;
	// step 7 produces "N task files generated".
	assert.Equal(t, 5, strings.Count(output, "Step complete"))
	assert.Contains(t, output, "task files generated")
}

func TestPlanner_Phase2_ParallelDocs(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements) + 1 (PRD) + 2 (parallel TECH+DESIGN) + 4 (task splitting) + 3 (task files) = 11
	require.Equal(t, 11, len(calls))

	// The two parallel calls (indices 2,3) should lack -c and use model.Thinking.
	for _, i := range []int{2, 3} {
		assert.NotContains(t, calls[i].args, "-c", "parallel call %d should not have -c", i)
		assert.Equal(t, model.Thinking, calls[i].modelType, "parallel call %d should use Thinking", i)
	}

	// Sub-step completion lines should appear in output.
	output := out.String()
	assert.Contains(t, output, "Technology plan")
	assert.Contains(t, output, "Design spec")
}

func TestPlanner_Phase2_ParallelDocOneFails(t *testing.T) {
	exec := &promptMatchExecutor{
		failOn: map[string]error{
			"DESIGN.md": fmt.Errorf("design generation failed"),
		},
	}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 2/7 failed")

	output := out.String()
	// One sub-step succeeded, one failed.
	assert.Contains(t, output, "Technology plan")
	assert.Contains(t, output, "Design spec")
}

func TestPlanner_Phase2_ParallelDocBothFail(t *testing.T) {
	exec := &promptMatchExecutor{
		failOn: map[string]error{
			"TECHNOLOGY.md": fmt.Errorf("tech failed"),
			"DESIGN.md":     fmt.Errorf("design failed"),
		},
	}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 2/7 failed")

	output := out.String()
	// Both failures reported in output.
	assert.Contains(t, output, "Technology plan")
	assert.Contains(t, output, "Design spec")
}

func TestPlanner_Phase2_ContextCancel_BeforeParallel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 2 calls (requirements + PRD), so context is cancelled before parallel step starts.
	cancellingExec := &cancellingMockExecutor{
		cancelAfter: 2,
		cancel:      cancel,
	}

	var out bytes.Buffer

	p := NewPlanner(cancellingExec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(ctx)
	require.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "Planning aborted at step 2/7")
}

func TestPlanner_Phase2_SubStepTimings(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	// Sub-step completion lines should contain durations (e.g., "0s" for fast mock).
	// StepComplete renders: " ✓ <name>  <duration>"
	// With the mock executor the duration will be very short ("0s").
	assert.Contains(t, output, "Technology plan")
	assert.Contains(t, output, "Design spec")
	assert.Contains(t, output, "0s")
}

func TestPlanner_Phase2_FromMode_Parallel(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithBrief("brief.md", "I want OAuth2 with Google"),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Only Phase 2: 1 (PRD) + 2 (parallel) + 4 (task splitting) + 3 (task files) = 10
	require.Equal(t, 10, len(calls))

	// PRD (call 0): no -c (--from mode, fresh conversation).
	assert.NotContains(t, calls[0].args, "-c", "PRD in --from mode should not have -c")

	// Parallel calls (1,2): no -c.
	assert.NotContains(t, calls[1].args, "-c", "parallel call should not have -c")
	assert.NotContains(t, calls[2].args, "-c", "parallel call should not have -c")

	// Create tasks (call 3): no -c (fresh conversation).
	assert.NotContains(t, calls[3].args, "-c", "create tasks should not have -c")

	// Assess, merge, generate (calls 4,5,6): have -c.
	assert.Contains(t, calls[4].args, "-c", "assess tasks should have -c")
	assert.Contains(t, calls[5].args, "-c", "merge tasks should have -c")
	assert.Contains(t, calls[6].args, "-c", "generate task summary should have -c")

	// Task file calls (7,8,9): no -c.
	for i := 7; i < 10; i++ {
		assert.NotContains(t, calls[i].args, "-c", "task file call %d should not have -c", i)
	}

	// PRD prompt should contain the brief.
	firstPrompt := calls[0].args[len(calls[0].args)-1]
	assert.Contains(t, firstPrompt, "I want OAuth2 with Google")

	output := out.String()
	assert.Contains(t, output, "using brief.md as input")
	assert.Contains(t, output, "Planning complete")
}

func TestPlanner_Phase2_ContextCancelMidStep(t *testing.T) {
	// Cancel after requirements prompt + first generation step (PRD).
	ctx, cancel := context.WithCancel(context.Background())

	cancellingExec := &cancellingMockExecutor{
		cancelAfter: 2, // cancel after 2nd call (requirements + PRD)
		cancel:      cancel,
	}

	var out bytes.Buffer

	p := NewPlanner(cancellingExec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(ctx)
	require.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "Planning aborted at step")
}

// cancellingMockExecutor cancels after N calls.
type cancellingMockExecutor struct {
	mu          sync.Mutex
	callCount   int
	cancelAfter int
	cancel      context.CancelFunc
}

func (m *cancellingMockExecutor) Run(_ context.Context, w io.Writer, _ model.Type, _ ...string) error {
	m.mu.Lock()
	m.callCount++
	count := m.callCount
	m.mu.Unlock()

	fmt.Fprintln(w, "LLM response")

	if count >= m.cancelAfter {
		m.cancel()
	}
	return nil
}

// --- Combined tests ---

func TestPlanner_FullPipeline(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("I want auth\nwith JWT sessions\n/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 2 (user messages) + 10 (generation) = 13
	assert.Equal(t, 13, len(calls))

	// Requirements prompt (no -c)
	assert.NotContains(t, calls[0].args, "-c")

	// User messages (with -c)
	for i := 1; i <= 2; i++ {
		assert.Contains(t, calls[i].args, "-c")
	}

	// PRD (call 3): -c
	assert.Contains(t, calls[3].args, "-c")
	// Parallel calls (4,5): no -c
	assert.NotContains(t, calls[4].args, "-c")
	assert.NotContains(t, calls[5].args, "-c")
	// Create tasks (call 6): no -c (fresh conversation)
	assert.NotContains(t, calls[6].args, "-c")
	// Assess, merge, generate (calls 7,8,9): -c
	assert.Contains(t, calls[7].args, "-c")
	assert.Contains(t, calls[8].args, "-c")
	assert.Contains(t, calls[9].args, "-c")
	// Task file calls (10,11,12): no -c
	for i := 10; i < 13; i++ {
		assert.NotContains(t, calls[i].args, "-c", "task file call %d should not have -c", i)
	}

	output := out.String()
	assert.Contains(t, output, "Planning session")
	assert.Contains(t, output, "Gathering requirements")
	assert.Contains(t, output, "Generating planning documents")
	assert.Contains(t, output, "Planning complete")
}

func TestPlanner_WithBrief_SkipsPhase1(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithBrief("requirements.md", "I want OAuth2 with Google"),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Only Phase 2: 1 (PRD) + 2 (parallel) + 4 (task splitting) + 3 (task files) = 10
	assert.Equal(t, 10, len(calls))

	// First gen step (PRD) should NOT have -c (fresh conversation start).
	assert.NotContains(t, calls[0].args, "-c")

	// Parallel calls (1,2): no -c.
	assert.NotContains(t, calls[1].args, "-c")
	assert.NotContains(t, calls[2].args, "-c")

	// Create tasks (call 3): no -c (fresh conversation).
	assert.NotContains(t, calls[3].args, "-c")

	// Assess, merge, generate (calls 4,5,6): have -c.
	assert.Contains(t, calls[4].args, "-c")
	assert.Contains(t, calls[5].args, "-c")
	assert.Contains(t, calls[6].args, "-c")

	// Task file calls (7,8,9): no -c.
	for i := 7; i < 10; i++ {
		assert.NotContains(t, calls[i].args, "-c", "task file call %d should not have -c", i)
	}

	// PRD prompt should contain the brief.
	firstPrompt := calls[0].args[len(calls[0].args)-1]
	assert.Contains(t, firstPrompt, "I want OAuth2 with Google")

	output := out.String()
	assert.Contains(t, output, "using requirements.md as input")
	assert.Contains(t, output, "Planning complete")
}

func TestPlanner_WithBrief_NoPhase1Output(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithBrief("brief.md", "some brief"),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	// Should NOT contain Phase 1 artifacts
	assert.NotContains(t, output, "Gathering requirements")
	assert.NotContains(t, output, "snap plan>")
}

func TestPlanner_ExecutorError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("provider failed")}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider failed")
}

// --- Resume tests ---

func TestPlanner_WithResume_FirstCallHasCFlag(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("refine tasks\n/done\n")),
		WithResume(true),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First call (requirements prompt) should have -c because of resume.
	require.GreaterOrEqual(t, len(calls), 1)
	assert.Contains(t, calls[0].args, "-c", "resume mode: first call should have -c")

	output := out.String()
	assert.Contains(t, output, "Resuming planning")
}

func TestPlanner_WithoutResume_FirstCallNoCFlag(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
		WithResume(false),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First call (requirements prompt) should NOT have -c (fresh start).
	require.GreaterOrEqual(t, len(calls), 1)
	assert.NotContains(t, calls[0].args, "-c", "fresh mode: first call should not have -c")

	output := out.String()
	assert.NotContains(t, output, "Resuming planning")
}

// --- AfterFirstMessage callback tests ---

func TestPlanner_AfterFirstMessage_CalledOnSuccess(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	var callbackCalled bool
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
		WithAfterFirstMessage(func() error {
			callbackCalled = true
			return nil
		}),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, callbackCalled, "afterFirstMessage should be called after first successful message")
}

func TestPlanner_AfterFirstMessage_NotCalledOnFailure(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("provider failed")}
	var out bytes.Buffer
	var callbackCalled bool

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
		WithAfterFirstMessage(func() error {
			callbackCalled = true
			return nil
		}),
	)

	err := p.Run(context.Background())
	require.Error(t, err)
	assert.False(t, callbackCalled, "afterFirstMessage should NOT be called when first message fails")
}

func TestPlanner_AfterFirstMessage_CalledOnceOnly(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	callCount := 0
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("msg1\nmsg2\n/done\n")),
		WithAfterFirstMessage(func() error {
			callCount++
			return nil
		}),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "afterFirstMessage should be called exactly once")
}

func TestPlanner_AfterFirstMessage_WithBrief(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	var callbackCalled bool
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithBrief("brief.md", "some brief"),
		WithAfterFirstMessage(func() error {
			callbackCalled = true
			return nil
		}),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, callbackCalled, "afterFirstMessage should be called in --from mode too")
}

// --- Interactive (tap) path tests ---
//
// These tests use tap.SetTermIO (global state) so they must NOT use t.Parallel().

// emitString types each rune as a keypress via the mock readable.
func emitString(in *tap.MockReadable, s string) {
	for _, ch := range s {
		str := string(ch)
		in.EmitKeypress(str, tap.Key{Name: str, Rune: ch})
	}
}

// emitLine types a string followed by Enter.
func emitLine(in *tap.MockReadable, s string) {
	emitString(in, s)
	in.EmitKeypress("", tap.Key{Name: "return"})
}

func TestPlanner_Interactive_UserMessageThenDone(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "hello")
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "/done")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 1 (user msg "hello") + 10 (generation) = 12
	assert.Equal(t, 12, len(calls))
	// Requirements prompt (no -c)
	assert.NotContains(t, calls[0].args, "-c")
	// User message with -c, last arg is "hello"
	assert.Contains(t, calls[1].args, "-c")
	assert.Equal(t, "hello", calls[1].args[len(calls[1].args)-1])
}

func TestPlanner_Interactive_DoneImmediately(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "/done")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 0 (no user msgs) + 10 (generation) = 11
	assert.Equal(t, 11, len(calls))
}

func TestPlanner_Interactive_DoneCaseInsensitive(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(100 * time.Millisecond)
		emitLine(in, "/DONE")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 10 (generation) = 11
	assert.Equal(t, 11, len(calls))
}

func TestPlanner_Interactive_CtrlC_Aborts(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		in.EmitKeypress("\x03", tap.Key{Name: "c", Ctrl: true})
	}()

	err := p.Run(context.Background())
	require.ErrorIs(t, err, context.Canceled)

	output := buf.String()
	assert.Contains(t, output, "Planning aborted")
}

func TestPlanner_Interactive_Escape_Aborts(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		in.EmitKeypress("", tap.Key{Name: "escape"})
	}()

	err := p.Run(context.Background())
	require.ErrorIs(t, err, context.Canceled)

	output := buf.String()
	assert.Contains(t, output, "Planning aborted")
}

func TestPlanner_Interactive_ContextCancel(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := p.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)

	output := buf.String()
	assert.Contains(t, output, "Planning aborted")
}

func TestPlanner_Interactive_MultipleMessages(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&buf),
		WithInteractive(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "msg1")
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "msg2")
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "/done")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements) + 2 (user msgs) + 10 (generation) = 13
	assert.Equal(t, 13, len(calls))
	// User messages have -c
	assert.Contains(t, calls[1].args, "-c")
	assert.Equal(t, "msg1", calls[1].args[len(calls[1].args)-1])
	assert.Contains(t, calls[2].args, "-c")
	assert.Equal(t, "msg2", calls[2].args[len(calls[2].args)-1])
}

func TestPlanner_Phase2_PreambleInPrompts(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Requirements prompt + 10 generation calls = 11 total.
	require.Equal(t, 11, len(calls))

	// PRD (1), parallel TECH (2), parallel DESIGN (3): all have preamble.
	for _, i := range []int{1, 2, 3} {
		prompt := calls[i].args[len(calls[i].args)-1]
		assert.Contains(t, prompt, "simplest solution",
			"Phase 2 call %d prompt should contain preamble text", i)
	}

	// Create tasks (4): has preamble (via RenderCreateTasksPrompt).
	prompt4 := calls[4].args[len(calls[4].args)-1]
	assert.Contains(t, prompt4, "simplest solution", "create tasks prompt should contain preamble")

	// Assess (5) and merge (6): no preamble (operate on conversation context from step 3).
	prompt5 := calls[5].args[len(calls[5].args)-1]
	assert.NotContains(t, prompt5, "simplest solution", "assess tasks prompt should not contain preamble")
	prompt6 := calls[6].args[len(calls[6].args)-1]
	assert.NotContains(t, prompt6, "simplest solution", "merge tasks prompt should not contain preamble")

	// Generate task summary (7): has preamble (via RenderGenerateTaskSummaryPrompt).
	prompt7 := calls[7].args[len(calls[7].args)-1]
	assert.Contains(t, prompt7, "simplest solution", "generate task summary prompt should contain preamble")

	// Task file prompts (8,9,10): have preamble (via RenderGenerateTaskFilePrompt).
	for i := 8; i < 11; i++ {
		prompt := calls[i].args[len(calls[i].args)-1]
		assert.Contains(t, prompt, "simplest solution",
			"task file call %d prompt should contain preamble text", i)
	}
}

// --- Task splitting chain tests ---

func TestPlanner_Phase2_TaskSplittingChain(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	require.Equal(t, 11, len(calls))

	// Task splitting chain: calls 4-7 (after requirements + PRD + 2 parallel).
	// Step 3 (create tasks, call 4): no -c.
	assert.NotContains(t, calls[4].args, "-c", "create tasks (step 3) should not have -c")

	// Step 4 (assess tasks, call 5): has -c.
	assert.Contains(t, calls[5].args, "-c", "assess tasks (step 4) should have -c")

	// Step 5 (merge tasks, call 6): has -c.
	assert.Contains(t, calls[6].args, "-c", "merge tasks (step 5) should have -c")

	// Step 6 (generate task summary, call 7): has -c.
	assert.Contains(t, calls[7].args, "-c", "generate task summary (step 6) should have -c")

	// All 4 task splitting calls use model.Thinking.
	for i := 4; i <= 7; i++ {
		assert.Equal(t, model.Thinking, calls[i].modelType,
			"task splitting call %d should use Thinking model", i)
	}
}

func TestPlanner_Phase2_StepHeaders_SevenSteps(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()

	// Verify all 7 step headers with correct names.
	steps := []struct {
		header string
		name   string
	}{
		{"Step 1/7", "Generate PRD"},
		{"Step 2/7", "Generate technology plan + design spec"},
		{"Step 3/7", "Create task list"},
		{"Step 4/7", "Assess tasks"},
		{"Step 5/7", "Refine tasks"},
		{"Step 6/7", "Generate task summary"},
		{"Step 7/7", "Generate task files"},
	}

	for _, s := range steps {
		assert.Contains(t, output, s.header, "output should contain %s", s.header)
		assert.Contains(t, output, s.name, "output should contain step name %q", s.name)
	}
}

func TestPlanner_Phase2_PreambleInTaskSplitting(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	require.Equal(t, 11, len(calls))

	// Step 3 (create tasks, call 4): has preamble.
	prompt4 := calls[4].args[len(calls[4].args)-1]
	assert.Contains(t, prompt4, "simplest solution", "step 3 (create tasks) should contain preamble")

	// Step 4 (assess tasks, call 5): no preamble.
	prompt5 := calls[5].args[len(calls[5].args)-1]
	assert.NotContains(t, prompt5, "simplest solution", "step 4 (assess tasks) should not contain preamble")

	// Step 5 (merge tasks, call 6): no preamble.
	prompt6 := calls[6].args[len(calls[6].args)-1]
	assert.NotContains(t, prompt6, "simplest solution", "step 5 (merge tasks) should not contain preamble")

	// Step 6 (generate task summary, call 7): has preamble.
	prompt7 := calls[7].args[len(calls[7].args)-1]
	assert.Contains(t, prompt7, "simplest solution", "step 6 (generate task summary) should contain preamble")
}

func TestPlanner_Phase2_ContextCancel_DuringTaskSplitting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 5 calls: requirements + PRD + 2 parallel + 1st task splitting step (create).
	cancellingExec := &cancellingMockExecutor{
		cancelAfter: 5,
		cancel:      cancel,
	}

	var out bytes.Buffer

	p := NewPlanner(cancellingExec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(ctx)
	require.Error(t, err)

	output := out.String()
	// Should abort at step 4/7 (assess tasks), which is the next step after the cancel fires.
	assert.Contains(t, output, "Planning aborted at step 4/7")
}

func TestPlanner_Interactive_WithResume(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&buf),
		WithInteractive(true),
		WithResume(true),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "refine")
		time.Sleep(200 * time.Millisecond)
		emitLine(in, "/done")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First executor call (requirements prompt) has -c (resume mode)
	require.GreaterOrEqual(t, len(calls), 1)
	assert.Contains(t, calls[0].args, "-c", "resume mode: first call should have -c")

	output := buf.String()
	assert.Contains(t, output, "Resuming planning")
}

// --- Step 7 (batched task file generation) tests ---

func TestPlanner_Phase2_BatchedTaskFiles(t *testing.T) {
	// 8 tasks in TASKS.md → 2 batches (5 + 3) for step 7.
	bigTasksMD := `# TASKS

## G. Task List

| # | File | Name | Epic | Outcome | Risk | Size |
|---|------|------|------|---------|------|------|
| 0 | TASK0.md | Task zero | E1 | Outcome 0 | Low | S |
| 1 | TASK1.md | Task one | E1 | Outcome 1 | Low | S |
| 2 | TASK2.md | Task two | E2 | Outcome 2 | Med | M |
| 3 | TASK3.md | Task three | E2 | Outcome 3 | Med | M |
| 4 | TASK4.md | Task four | E3 | Outcome 4 | Low | S |
| 5 | TASK5.md | Task five | E3 | Outcome 5 | Low | S |
| 6 | TASK6.md | Task six | E4 | Outcome 6 | Med | M |
| 7 | TASK7.md | Task seven | E4 | Outcome 7 | High | L |

## H. Dependencies
`
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(bigTasksMD), 0o600))

	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", dir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements) + 1 (PRD) + 2 (parallel) + 4 (task splitting) + 8 (task files) = 16
	assert.Equal(t, 16, len(calls))

	output := out.String()
	// With 8 tasks (>5), batch progress should appear.
	assert.Contains(t, output, "Batch 1/2")
	assert.Contains(t, output, "Batch 2/2")
	assert.Contains(t, output, "Step 7/7")
}

func TestPlanner_Phase2_BatchedTaskFiles_SmallBatch(t *testing.T) {
	// 3 tasks → single batch, simple completion line.
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t) // 3 tasks

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	// ≤5 tasks → no "Batch N/M", just "N task files generated"
	assert.NotContains(t, output, "Batch")
	assert.Contains(t, output, "3 task files generated")
}

func TestPlanner_Phase2_BatchedTaskFiles_PartialFailure(t *testing.T) {
	// Use promptMatchExecutor to fail specific task file prompts.
	exec := &promptMatchExecutor{
		failOn: map[string]error{
			"TASK1.md": fmt.Errorf("claude command failed: exit status 1"),
		},
	}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t) // 3 tasks

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 7/7")
	assert.Contains(t, err.Error(), "TASK1.md")

	output := out.String()
	assert.Contains(t, output, "TASK1.md")
}

func TestPlanner_Phase2_BatchedTaskFiles_PartialFailure_LargeBatch(t *testing.T) {
	// 8 tasks (>5) → 2 batches. One task fails in batch 1.
	// Verify: both batches run, failures collected, error reported with all failures.
	bigTasksMD := `# TASKS

## G. Task List

| # | File | Name | Epic | Outcome | Risk | Size |
|---|------|------|------|---------|------|------|
| 0 | TASK0.md | Task zero | E1 | Outcome 0 | Low | S |
| 1 | TASK1.md | Task one | E1 | Outcome 1 | Low | S |
| 2 | TASK2.md | Task two | E2 | Outcome 2 | Med | M |
| 3 | TASK3.md | Task three | E2 | Outcome 3 | Med | M |
| 4 | TASK4.md | Task four | E3 | Outcome 4 | Low | S |
| 5 | TASK5.md | Task five | E3 | Outcome 5 | Low | S |
| 6 | TASK6.md | Task six | E4 | Outcome 6 | Med | M |
| 7 | TASK7.md | Task seven | E4 | Outcome 7 | High | L |

## H. Dependencies
`
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(bigTasksMD), 0o600))

	// Fail TASK1.md (in first batch) but not others.
	exec := &promptMatchExecutor{
		failOn: map[string]error{
			"TASK1.md": fmt.Errorf("claude command failed: exit status 1"),
		},
	}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", dir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.Error(t, err)

	// Error should report the failure with step and task count.
	assert.Contains(t, err.Error(), "step 7/7")
	assert.Contains(t, err.Error(), "TASK1.md")
	assert.Contains(t, err.Error(), "task file(s) failed")

	output := out.String()
	// Batch 1 should show as failed.
	assert.Contains(t, output, "Batch 1/2")
	// Batch 2 should have run (either succeeded or is mentioned).
	assert.Contains(t, output, "Batch 2/2")
}

func TestPlanner_Phase2_ContextCancel_DuringBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 8 calls: 1 req + 1 PRD + 2 parallel + 4 task splitting = 8.
	// This fires just before step 7 starts its batch.
	cancellingExec := &cancellingMockExecutor{
		cancelAfter: 8,
		cancel:      cancel,
	}

	var out bytes.Buffer
	tasksDir := setupTasksDir(t)

	p := NewPlanner(cancellingExec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(ctx)
	require.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "Planning aborted")
	assert.Contains(t, output, "step 7/7")
}

func TestPlanner_Phase2_PreambleInTaskFilePrompts(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer
	tasksDir := setupTasksDir(t) // 3 tasks

	p := NewPlanner(exec, "auth", tasksDir,
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	require.Equal(t, 11, len(calls))

	// Task file prompts are calls 8, 9, 10 (indices after requirements + PRD + 2 parallel + 4 splitting).
	for i := 8; i < 11; i++ {
		prompt := calls[i].args[len(calls[i].args)-1]
		assert.Contains(t, prompt, "simplest solution",
			"task file call %d should contain preamble", i)
		assert.Contains(t, prompt, "TASK",
			"task file call %d should reference TASK in prompt", i)
	}
}
