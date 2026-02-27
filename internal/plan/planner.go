package plan

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

// planStep defines one step in the Phase 2 generation pipeline.
type planStep struct {
	name       string
	renderFunc func(tasksDir, brief string) (string, error)
}

var planSteps = []planStep{
	{name: "Generate PRD", renderFunc: RenderPRDPrompt},
	{name: "Generate technology plan", renderFunc: func(td, _ string) (string, error) { return RenderTechnologyPrompt(td) }},
	{name: "Generate design spec", renderFunc: func(td, _ string) (string, error) { return RenderDesignPrompt(td) }},
	{name: "Split into tasks", renderFunc: func(td, _ string) (string, error) { return RenderTasksPrompt(td) }},
}

// Planner orchestrates the two-phase planning pipeline.
type Planner struct {
	executor    workflow.Executor
	sessionName string
	tasksDir    string
	output      io.Writer
	input       io.Reader
	briefFile   string // filename for display (e.g., "brief.md")
	briefBody   string // file content
}

// PlannerOption configures a Planner.
type PlannerOption func(*Planner)

// WithOutput sets the output writer.
func WithOutput(w io.Writer) PlannerOption {
	return func(p *Planner) { p.output = w }
}

// WithInput sets the input reader for Phase 1.
func WithInput(r io.Reader) PlannerOption {
	return func(p *Planner) { p.input = r }
}

// WithBrief sets the brief file content, skipping Phase 1.
func WithBrief(filename, content string) PlannerOption {
	return func(p *Planner) {
		p.briefFile = filename
		p.briefBody = content
	}
}

// NewPlanner creates a new Planner with the given options.
func NewPlanner(executor workflow.Executor, sessionName, tasksDir string, opts ...PlannerOption) *Planner {
	p := &Planner{
		executor:    executor,
		sessionName: sessionName,
		tasksDir:    tasksDir,
		output:      os.Stdout,
		input:       os.Stdin,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run orchestrates the full planning pipeline: Phase 1 (requirements gathering)
// followed by Phase 2 (autonomous document generation).
func (p *Planner) Run(ctx context.Context) error {
	if p.briefBody != "" {
		fmt.Fprintf(p.output, "Planning session '%s' — using %s as input\n", p.sessionName, p.briefFile)
	} else {
		fmt.Fprintf(p.output, "Planning session '%s'\n", p.sessionName)
	}

	// Phase 1: requirements gathering (skipped when brief is set).
	if p.briefBody == "" {
		if err := p.gatherRequirements(ctx); err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(p.output, ui.Interrupted("Planning aborted"))
			}
			return err
		}
	}

	// Phase 2: autonomous document generation.
	return p.generateDocuments(ctx)
}

// gatherRequirements runs the interactive Phase 1 chat loop.
func (p *Planner) gatherRequirements(ctx context.Context) error {
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, "Gathering requirements — type /done when ready")
	fmt.Fprintln(p.output)

	// Send the initial requirements-gathering prompt.
	prompt := RenderRequirementsPrompt()
	if err := p.executor.Run(ctx, p.output, model.Thinking, prompt); err != nil {
		return fmt.Errorf("requirements prompt failed: %w", err)
	}

	// Chat loop: read user input, send with -c, repeat until /done or EOF.
	scanner := bufio.NewScanner(p.input)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fmt.Fprint(p.output, "\nsnap plan> ")

		if !scanner.Scan() {
			// EOF — transition to Phase 2.
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(line, "/done") {
			break
		}

		if line == "" {
			continue
		}

		// Send user message with -c for conversation continuity.
		if err := p.executor.Run(ctx, p.output, model.Thinking, "-c", line); err != nil {
			return fmt.Errorf("chat message failed: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("input read error: %w", err)
	}

	return nil
}

// generateDocuments runs the autonomous Phase 2 document generation pipeline.
func (p *Planner) generateDocuments(ctx context.Context) error {
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, "Generating planning documents...")
	fmt.Fprintln(p.output)

	for i, step := range planSteps {
		stepNum := i + 1

		if ctx.Err() != nil {
			fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step %d/%d", stepNum, len(planSteps))))
			fmt.Fprintf(p.output, "  Files written so far are preserved in %s\n", p.tasksDir)
			return ctx.Err()
		}

		prompt, err := step.renderFunc(p.tasksDir, p.briefBody)
		if err != nil {
			return fmt.Errorf("failed to render %s prompt: %w", step.name, err)
		}

		// Build args: first step in --from mode has no -c (fresh conversation).
		// All other steps use -c for conversation continuity.
		var args []string
		needsContinuation := true
		if p.briefBody != "" && i == 0 {
			needsContinuation = false
		}
		if needsContinuation {
			args = append(args, "-c")
		}
		args = append(args, prompt)

		fmt.Fprint(p.output, ui.StepNumbered(stepNum, len(planSteps), step.name))

		start := time.Now()
		if err := p.executor.Run(ctx, p.output, model.Thinking, args...); err != nil {
			elapsed := time.Since(start)
			fmt.Fprintln(p.output, ui.StepFailed("Step failed", elapsed))

			// Check if this is a context cancellation.
			if ctx.Err() != nil {
				fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step %d/%d", stepNum, len(planSteps))))
				fmt.Fprintf(p.output, "  Files written so far are preserved in %s\n", p.tasksDir)
				return ctx.Err()
			}

			return fmt.Errorf("step %d/%d %q failed: %w", stepNum, len(planSteps), step.name, err)
		}

		elapsed := time.Since(start)
		fmt.Fprintln(p.output, ui.StepComplete("Step complete", elapsed))
	}

	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, ui.Complete("Planning complete"))

	return nil
}
