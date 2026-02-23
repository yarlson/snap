package workflow_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

func TestDrainQueue_ExecutesAllPrompts(t *testing.T) {
	var executed []string
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
			// Last arg is the prompt.
			executed = append(executed, args[len(args)-1])
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("fix the nil pointer")
	q.Enqueue("add a test for empty input")

	runner := workflow.NewStepRunner(mockExec, io.Discard)
	errors := workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	assert.Len(t, errors, 0)
	assert.Equal(t, 2, len(executed))
	// Prompts should contain the user text plus suffixes.
	assert.Contains(t, executed[0], "fix the nil pointer")
	assert.Contains(t, executed[1], "add a test for empty input")
}

func TestDrainQueue_EmptyQueue(t *testing.T) {
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			t.Fatal("executor should not be called for empty queue")
			return nil
		},
	}

	q := queue.New()
	runner := workflow.NewStepRunner(mockExec, io.Discard)
	errors := workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	assert.Nil(t, errors)
}

func TestDrainQueue_ContinuesOnError(t *testing.T) {
	callCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			callCount++
			if callCount == 1 {
				return fmt.Errorf("first prompt failed")
			}
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("failing prompt")
	q.Enqueue("succeeding prompt")

	runner := workflow.NewStepRunner(mockExec, io.Discard)
	errs := workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	// Both prompts should be attempted.
	assert.Equal(t, 2, callCount)
	// One error should be recorded.
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "first prompt failed")
}

func TestDrainQueue_UsesContextFlag(t *testing.T) {
	var capturedArgs []string
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
			capturedArgs = args
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("user directive")

	runner := workflow.NewStepRunner(mockExec, io.Discard)
	workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	// Should include -c flag for session context.
	assert.Contains(t, capturedArgs, "-c")
}

func TestDrainQueue_AddsAutonomousAndNoCommitSuffix(t *testing.T) {
	var capturedPrompt string
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
			capturedPrompt = args[len(args)-1]
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("user directive")

	runner := workflow.NewStepRunner(mockExec, io.Discard)
	workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	assert.True(t, strings.Contains(capturedPrompt, "Work autonomously"))
	assert.True(t, strings.Contains(capturedPrompt, "Do not stage, commit"))
}

func TestDrainQueue_FIFOOrder(t *testing.T) {
	var order []int
	callCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			callCount++
			order = append(order, callCount)
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("first")
	q.Enqueue("second")
	q.Enqueue("third")

	runner := workflow.NewStepRunner(mockExec, io.Discard)
	workflow.DrainQueue(context.Background(), io.Discard, runner, q)

	assert.Equal(t, []int{1, 2, 3}, order)
	assert.Equal(t, 0, q.Len(), "queue should be empty after drain")
}

func TestDrainQueue_StopsOnCancelledContext(t *testing.T) {
	callCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			callCount++
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("first")
	q.Enqueue("second")
	q.Enqueue("third")

	// Cancel context before draining.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf strings.Builder
	runner := workflow.NewStepRunner(mockExec, io.Discard)
	errs := workflow.DrainQueue(ctx, &buf, runner, q)

	// No prompts should have been executed.
	assert.Equal(t, 0, callCount)
	// Should return context error.
	assert.Len(t, errs, 1)
	assert.ErrorIs(t, errs[0], context.Canceled)
	// Should log all skipped prompts.
	output := buf.String()
	assert.Contains(t, output, "Skipped queued prompt: first")
	assert.Contains(t, output, "Skipped queued prompt: second")
	assert.Contains(t, output, "Skipped queued prompt: third")
}

func TestDrainQueue_StopsMidDrainOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			callCount++
			// Cancel context after first prompt executes.
			if callCount == 1 {
				cancel()
			}
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("first")
	q.Enqueue("second")
	q.Enqueue("third")

	var buf strings.Builder
	runner := workflow.NewStepRunner(mockExec, io.Discard)
	errs := workflow.DrainQueue(ctx, &buf, runner, q)

	// Only first prompt should have executed.
	assert.Equal(t, 1, callCount)
	// Should return context error.
	assert.Len(t, errs, 1)
	assert.ErrorIs(t, errs[0], context.Canceled)
	// Should log remaining skipped prompts (not the first, which ran).
	output := buf.String()
	assert.NotContains(t, output, "Skipped queued prompt: first")
	assert.Contains(t, output, "Skipped queued prompt: second")
	assert.Contains(t, output, "Skipped queued prompt: third")
}

func TestDrainQueue_OutputsBoxedRunningHeader(t *testing.T) {
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			return nil
		},
	}

	q := queue.New()
	q.Enqueue("fix the nil pointer")
	q.Enqueue("add test")

	var buf strings.Builder
	runner := workflow.NewStepRunner(mockExec, io.Discard)
	workflow.DrainQueue(context.Background(), &buf, runner, q)

	output := buf.String()
	stripped := ui.StripColors(output)

	// Should contain boxed running headers with numbering.
	assert.Contains(t, stripped, "📌 Running queued prompt (1/2)", "Should show first running header")
	assert.Contains(t, stripped, "📌 Running queued prompt (2/2)", "Should show second running header")
	assert.Contains(t, stripped, "fix the nil pointer", "Should show first prompt text")
	assert.Contains(t, stripped, "add test", "Should show second prompt text")
	// Should contain box drawing characters.
	assert.Contains(t, stripped, "┌", "Should have box top border")
	assert.Contains(t, stripped, "└", "Should have box bottom border")
}
