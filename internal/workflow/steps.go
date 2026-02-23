package workflow

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/ui"
)

const (
	autonomousSuffix = "Work autonomously end-to-end. Do not ask the user any questions. Do not request approval. Do not pause for confirmation."
	noCommitSuffix   = "Do not stage, commit, amend, rebase, or push any changes in this step."
)

// Executor runs an external coding agent command (e.g., claude or codex).
type Executor interface {
	Run(ctx context.Context, w io.Writer, mt model.Type, args ...string) error
}

// StepRunner executes workflow steps using the configured agent CLI.
type StepRunner struct {
	executor Executor
	output   io.Writer
}

// NewStepRunner creates a new step runner that writes output to w.
func NewStepRunner(executor Executor, w io.Writer) *StepRunner {
	return &StepRunner{
		executor: executor,
		output:   w,
	}
}

// RunStep executes a single workflow step with the given name and arguments.
func (r *StepRunner) RunStep(ctx context.Context, stepName string, mt model.Type, args ...string) error {
	fmt.Fprint(r.output, ui.Step(stepName))

	if err := r.executor.Run(ctx, r.output, mt, args...); err != nil {
		return fmt.Errorf("step %q failed: %w", stepName, err)
	}

	return nil
}

// RunStepNumbered executes a single workflow step with step numbering.
func (r *StepRunner) RunStepNumbered(ctx context.Context, current, total int, stepName string, mt model.Type, args ...string) error {
	fmt.Fprint(r.output, ui.StepNumbered(current, total, stepName))

	if err := r.executor.Run(ctx, r.output, mt, args...); err != nil {
		return fmt.Errorf("step %d/%d %q failed: %w", current, total, stepName, err)
	}

	return nil
}

// PromptOption is a function that modifies prompt building behavior.
type PromptOption func(*promptConfig)

type promptConfig struct {
	noCommit bool
}

// WithNoCommit adds the no-commit suffix to the prompt.
func WithNoCommit() PromptOption {
	return func(c *promptConfig) {
		c.noCommit = true
	}
}

// BuildPrompt constructs a prompt with the autonomous suffix and optional no-commit suffix.
func BuildPrompt(base string, options ...PromptOption) string {
	cfg := &promptConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	var parts []string
	if base != "" {
		parts = append(parts, base)
	}
	if cfg.noCommit {
		parts = append(parts, noCommitSuffix)
	}
	parts = append(parts, autonomousSuffix)

	return strings.Join(parts, " ")
}
