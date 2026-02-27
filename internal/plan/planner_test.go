package plan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	// Phase 2 should still run (4 generation steps)
	// Total: 1 (requirements prompt) + 4 (generation steps) = 5
	assert.Equal(t, 5, len(calls))
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

	// Should have completed both phases (1 requirements + 4 generation)
	calls := exec.getCalls()
	assert.Equal(t, 5, len(calls))
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
	// 1 (requirements) + 1 (user msg) + 4 (generation) = 6
	assert.Equal(t, 6, len(calls))
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

func TestPlanner_Phase2_FourSteps(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	calls := exec.getCalls()
	// Requirements prompt + 4 generation steps = 5 total calls.
	assert.Equal(t, 5, len(calls))

	// All Phase 2 calls should use -c flag and model.Thinking
	for i := 1; i < 5; i++ {
		assert.Contains(t, calls[i].args, "-c", "step %d should have -c flag", i)
		assert.Equal(t, model.Thinking, calls[i].modelType, "step %d should use Thinking model", i)
	}
}

func TestPlanner_Phase2_StepHeaders(t *testing.T) {
	exec := &mockExecutor{}
	var out bytes.Buffer

	p := NewPlanner(exec, "auth", ".snap/sessions/auth/tasks",
		WithOutput(&out),
		WithInput(strings.NewReader("/done\n")),
	)

	err := p.Run(context.Background())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Step 1/4")
	assert.Contains(t, output, "Step 2/4")
	assert.Contains(t, output, "Step 3/4")
	assert.Contains(t, output, "Step 4/4")
	assert.Contains(t, output, "Generate PRD")
	assert.Contains(t, output, "Generate technology plan")
	assert.Contains(t, output, "Generate design spec")
	assert.Contains(t, output, "Split into tasks")
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
	// Step complete markers should appear (4 times)
	assert.Equal(t, 4, strings.Count(output, "Step complete"))
}

func TestPlanner_Phase2_ContextCancelMidStep(t *testing.T) {
	// Override: cancel after requirements prompt + first generation step
	ctx, cancel := context.WithCancel(context.Background())

	cancellingExec := &cancellingMockExecutor{
		cancelAfter: 2, // cancel after 2nd call (requirements + first gen step)
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
	// 1 (requirements prompt) + 2 (user messages) + 4 (generation) = 7
	assert.Equal(t, 7, len(calls))

	// Requirements prompt (no -c)
	assert.NotContains(t, calls[0].args, "-c")

	// User messages (with -c)
	for i := 1; i <= 2; i++ {
		assert.Contains(t, calls[i].args, "-c")
	}

	// Generation steps (with -c)
	for i := 3; i <= 6; i++ {
		assert.Contains(t, calls[i].args, "-c")
	}

	output := out.String()
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
	// Only Phase 2: 4 generation steps (no requirements prompt, no user messages)
	assert.Equal(t, 4, len(calls))

	// First gen step should NOT have -c (fresh conversation start)
	assert.NotContains(t, calls[0].args, "-c")

	// Remaining gen steps should have -c
	for i := 1; i < 4; i++ {
		assert.Contains(t, calls[i].args, "-c")
	}

	// PRD prompt should contain the brief
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
