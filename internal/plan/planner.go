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
	executor          workflow.Executor
	sessionName       string
	tasksDir          string
	output            io.Writer
	input             io.Reader
	briefFile         string       // filename for display (e.g., "brief.md")
	briefBody         string       // file content
	resume            bool         // when true, first executor call uses -c to continue previous conversation
	afterFirstMessage func() error // called once after the first successful executor call
	firstMessageDone  bool
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

// WithResume sets whether to resume a previous planning conversation.
// When true, the first executor call uses -c for conversation continuity.
func WithResume(resume bool) PlannerOption {
	return func(p *Planner) { p.resume = resume }
}

// WithAfterFirstMessage sets a callback that fires once after the first successful executor call.
func WithAfterFirstMessage(fn func() error) PlannerOption {
	return func(p *Planner) { p.afterFirstMessage = fn }
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

// onFirstMessage fires the afterFirstMessage callback once after the first successful executor call.
func (p *Planner) onFirstMessage() error {
	if p.firstMessageDone || p.afterFirstMessage == nil {
		return nil
	}
	p.firstMessageDone = true
	return p.afterFirstMessage()
}

// Run orchestrates the full planning pipeline: Phase 1 (requirements gathering)
// followed by Phase 2 (autonomous document generation).
func (p *Planner) Run(ctx context.Context) error {
	switch {
	case p.briefBody != "":
		fmt.Fprintf(p.output, "Planning session '%s' — using %s as input\n", p.sessionName, p.briefFile)
	case p.resume:
		fmt.Fprintf(p.output, "Resuming planning for session '%s'\n", p.sessionName)
	default:
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
	// When resuming, add -c flag to continue previous conversation.
	prompt := RenderRequirementsPrompt()
	var initArgs []string
	if p.resume {
		initArgs = append(initArgs, "-c")
	}
	initArgs = append(initArgs, prompt)
	if err := p.executor.Run(ctx, p.output, model.Thinking, initArgs...); err != nil {
		return fmt.Errorf("requirements prompt failed: %w", err)
	}

	if err := p.onFirstMessage(); err != nil {
		return err
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

		if err := p.onFirstMessage(); err != nil {
			return err
		}

		elapsed := time.Since(start)
		fmt.Fprintln(p.output, ui.StepComplete("Step complete", elapsed))
	}

	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, ui.Complete("Planning complete"))

	return nil
}
