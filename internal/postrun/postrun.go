package postrun

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/yarlson/snap/internal/ui"
)

// Config holds configuration for the post-run step.
type Config struct {
	Output    io.Writer
	RemoteURL string // Pre-detected remote URL (empty = no remote)
	IsGitHub  bool   // Pre-detected GitHub flag
	PRDPath   string // For future PR body context (TASK2)
	TasksDir  string // For future PR body context (TASK2)
}

// Run executes the post-run step: push to remote, and (in future tasks)
// create PR and monitor CI.
func Run(ctx context.Context, cfg Config) error {
	if cfg.RemoteURL == "" {
		fmt.Fprint(cfg.Output, ui.Info("No remote configured, skipping push"))
		return nil
	}

	// Push to origin
	fmt.Fprint(cfg.Output, ui.Step("Pushing to origin..."))
	pushStart := time.Now()

	if err := Push(ctx); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	branch, err := CurrentBranch(ctx)
	if err != nil {
		branch = "unknown"
	}
	fmt.Fprint(cfg.Output, ui.StepComplete(fmt.Sprintf("Pushed to origin/%s", branch), time.Since(pushStart)))

	if !cfg.IsGitHub {
		fmt.Fprint(cfg.Output, ui.Info("Non-GitHub remote, skipping PR and CI"))
		return nil
	}

	// TASK2 extends from here: PR creation, CI monitoring
	return nil
}
