package workflow

import (
	"context"
	"fmt"
	"io"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/ui"
)

// DrainQueue executes all queued prompts in FIFO order via the step runner.
// Each prompt runs as a context-continuing invocation with autonomous and no-commit suffixes.
// Errors are collected but do not stop execution of remaining prompts.
// Returns nil if the queue was empty. Stops early if the context is cancelled.
func DrainQueue(ctx context.Context, w io.Writer, stepRunner *StepRunner, q *queue.Queue) []error {
	prompts := q.DrainAll()
	if len(prompts) == 0 {
		return nil
	}

	var errs []error
	total := len(prompts)

	for i, prompt := range prompts {
		// Check for context cancellation before executing each prompt.
		if err := ctx.Err(); err != nil {
			for _, skipped := range prompts[i:] {
				fmt.Fprint(w, ui.Info(fmt.Sprintf("Skipped queued prompt: %s", skipped)))
			}
			errs = append(errs, err)
			return errs
		}

		fmt.Fprint(w, ui.QueueRunning(prompt, i+1, total))

		// Build prompt with autonomous + no-commit suffixes.
		fullPrompt := BuildPrompt(prompt, WithNoCommit())

		// Execute with -c flag to maintain session context.
		if err := stepRunner.RunStep(ctx, fmt.Sprintf("Queued prompt %d/%d", i+1, total), model.Fast, "-c", fullPrompt); err != nil {
			fmt.Fprint(w, ui.Error(fmt.Sprintf("Queued prompt failed: %v", err)))
			fmt.Fprintln(w)
			errs = append(errs, err)
		}
	}

	return errs
}
