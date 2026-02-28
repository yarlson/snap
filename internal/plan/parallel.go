package plan

import (
	"bytes"
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/workflow"
)

// parallelTask defines one task to run concurrently in the parallel step.
type parallelTask struct {
	name      string
	modelType model.Type
	args      []string
}

// parallelResult holds the outcome of a single parallel task.
type parallelResult struct {
	name    string
	output  *bytes.Buffer
	elapsed time.Duration
	err     error
}

// runParallel spawns goroutines via errgroup for each task, collects results with timing,
// and waits for all to complete or context cancel. Each goroutine writes to its own buffer.
// When limit > 0, errgroup.SetLimit restricts maximum concurrent goroutines.
func runParallel(ctx context.Context, executor workflow.Executor, tasks []parallelTask, limit int) []parallelResult {
	results := make([]parallelResult, len(tasks))
	g, gctx := errgroup.WithContext(ctx)

	if limit > 0 {
		g.SetLimit(limit)
	}

	for i, task := range tasks {
		results[i] = parallelResult{name: task.name, output: &bytes.Buffer{}}

		g.Go(func() error {
			start := time.Now()
			err := executor.Run(gctx, results[i].output, task.modelType, task.args...)
			elapsed := time.Since(start)

			results[i].elapsed = elapsed
			results[i].err = err

			// Return nil so errgroup doesn't cancel sibling goroutines on failure.
			// We want all tasks to complete (or be cancelled by parent context) so
			// we can report individual results.
			return nil
		})
	}

	//nolint:errcheck // goroutines always return nil; errors stored in results
	g.Wait()
	return results
}
