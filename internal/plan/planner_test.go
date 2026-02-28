package plan

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// --- Phase 1 tests ---

func TestPlanner_Phase1_UserMessageThenDone(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// First call: requirements prompt
	require.GreaterOrEqual(t, len(calls), 1)
	assert.NotContains(t, calls[0].args, "-c")

	// Phase 2 pipeline: 1 (PRD) + 2 (parallel TECH+DESIGN) + 4 (task splitting chain) = 7
	// Total: 1 (requirements prompt) + 7 = 8
	assert.Equal(t, 8, len(calls))
}

func TestPlanner_Phase1_DoneUppercase(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/DONE\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	// Should have completed both phases (1 requirements + 7 generation)
	calls := exec.getCalls()
	assert.Equal(t, 8, len(calls))
}

func TestPlanner_Phase1_EOF(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	// No /done, just EOF
	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("some requirements\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	// Should have run user message + Phase 2
	calls := exec.getCalls()
	// 1 (requirements) + 1 (user msg) + 7 (generation) = 9
	assert.Equal(t, 9, len(calls))
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Requirements prompt + 7 generation calls (1 PRD + 2 parallel + 4 task splitting) = 8.
	require.Equal(t, 8, len(calls))

	// All Phase 2 calls should use model.Thinking.
	for i := 1; i < 8; i++ {
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
}

func TestPlanner_Phase2_StepHeaders_NewPipeline(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Step 1/6")
	assert.Contains(t, output, "Step 2/6")
	assert.Contains(t, output, "Step 3/6")
	assert.Contains(t, output, "Step 4/6")
	assert.Contains(t, output, "Step 5/6")
	assert.Contains(t, output, "Step 6/6")
	assert.Contains(t, output, "Generate PRD")
	assert.Contains(t, output, "Generate technology plan + design spec")
	assert.Contains(t, output, "Create task list")
	assert.Contains(t, output, "Assess tasks")
	assert.Contains(t, output, "Refine tasks")
	assert.Contains(t, output, "Generate task summary")
}

func TestPlanner_Phase2_StepCompletions(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	// Steps 1, 3, 4, 5, 6 produce "Step complete"; step 2 produces sub-step names.
	assert.Equal(t, 5, strings.Count(output, "Step complete"))
}

func TestPlanner_Phase2_ParallelDocs(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements) + 1 (PRD) + 2 (parallel TECH+DESIGN) + 4 (task splitting) = 8
	require.Equal(t, 8, len(calls))

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
	assert.Contains(t, err.Error(), "step 2/6 failed")

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
	assert.Contains(t, err.Error(), "step 2/6 failed")

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
	assert.Contains(t, output, "Planning aborted at step 2/6")
}

func TestPlanner_Phase2_SubStepTimings(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithBrief("brief.md", "I want OAuth2 with Google"),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Only Phase 2: 1 (PRD) + 2 (parallel) + 4 (task splitting) = 7
	require.Equal(t, 7, len(calls))

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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("I want auth\nwith JWT sessions\n/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 2 (user messages) + 7 (generation) = 10
	assert.Equal(t, 10, len(calls))

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

	output := out.String()
	assert.Contains(t, output, "Planning session")
	assert.Contains(t, output, "Gathering requirements")
	assert.Contains(t, output, "Generating planning documents")
	assert.Contains(t, output, "Planning complete")
}

func TestPlanner_WithBrief_SkipsPhase1(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithBrief("requirements.md", "I want OAuth2 with Google"),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Only Phase 2: 1 (PRD) + 2 (parallel) + 4 (task splitting) = 7
	assert.Equal(t, 7, len(calls))

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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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
	// 1 (requirements prompt) + 1 (user msg "hello") + 7 (generation) = 9
	assert.Equal(t, 9, len(calls))
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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
	// 1 (requirements prompt) + 0 (no user msgs) + 7 (generation) = 8
	assert.Equal(t, 8, len(calls))
}

func TestPlanner_Interactive_DoneCaseInsensitive(t *testing.T) {
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
		time.Sleep(100 * time.Millisecond)
		emitLine(in, "/DONE")
	}()

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// 1 (requirements prompt) + 7 (generation) = 8
	assert.Equal(t, 8, len(calls))
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

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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
	// 1 (requirements) + 2 (user msgs) + 7 (generation) = 10
	assert.Equal(t, 10, len(calls))
	// User messages have -c
	assert.Contains(t, calls[1].args, "-c")
	assert.Equal(t, "msg1", calls[1].args[len(calls[1].args)-1])
	assert.Contains(t, calls[2].args, "-c")
	assert.Equal(t, "msg2", calls[2].args[len(calls[2].args)-1])
}

func TestPlanner_Phase2_PreambleInPrompts(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Requirements prompt + 7 generation calls = 8 total.
	require.Equal(t, 8, len(calls))

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
}

// --- Task splitting chain tests ---

func TestPlanner_Phase2_TaskSplittingChain(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	require.Equal(t, 8, len(calls))

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

func TestPlanner_Phase2_StepHeaders_SixSteps(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()

	// Verify all 6 step headers with correct names.
	steps := []struct {
		header string
		name   string
	}{
		{"Step 1/6", "Generate PRD"},
		{"Step 2/6", "Generate technology plan + design spec"},
		{"Step 3/6", "Create task list"},
		{"Step 4/6", "Assess tasks"},
		{"Step 5/6", "Refine tasks"},
		{"Step 6/6", "Generate task summary"},
	}

	for _, s := range steps {
		assert.Contains(t, output, s.header, "output should contain %s", s.header)
		assert.Contains(t, output, s.name, "output should contain step name %q", s.name)
	}
}

func TestPlanner_Phase2_PreambleInTaskSplitting(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	require.Equal(t, 8, len(calls))

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
	// Should abort at step 4/6 (assess tasks), which is the next step after the cancel fires.
	assert.Contains(t, output, "Planning aborted at step 4/6")
}

func TestPlanner_Interactive_WithResume(t *testing.T) {
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	exec := &mockExecutor{}
	var buf bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
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
