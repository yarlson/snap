package plan

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/yarlson/snap/internal/input"
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
	terminal          *os.File     // when set and is a TTY, enables raw-mode input via ReadLine
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

// WithTerminal sets the terminal file for raw-mode input during Phase 1.
// When set and stdin is a TTY, the planner uses ReadLine for interactive input
// with proper Ctrl+C handling and escape sequence consumption.
func WithTerminal(f *os.File) PlannerOption {
	return func(p *Planner) { p.terminal = f }
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
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
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
	if p.terminal != nil && input.IsTerminal(p.terminal) {
		return p.gatherRequirementsRaw(ctx)
	}
	return p.gatherRequirementsScanner(ctx)
}

// gatherRequirementsRaw uses raw-mode ReadLine for interactive TTY input.
// Ctrl+C returns context.Canceled to abort the plan command.
func (p *Planner) gatherRequirementsRaw(ctx context.Context) error {
	fd := int(p.terminal.Fd()) //nolint:gosec // G115: fd fits int
	return input.WithRawMode(fd, func() error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			fmt.Fprint(p.output, "\n")
			line, err := input.ReadLine(p.terminal, p.output, "snap plan> ")
			if errors.Is(err, input.ErrInterrupt) {
				return context.Canceled
			}
			if err != nil {
				// EOF or read error — transition to Phase 2.
				return nil //nolint:nilerr // EOF means end of input, not failure.
			}

			line = strings.TrimSpace(line)
			if strings.EqualFold(line, "/done") {
				return nil
			}
			if line == "" {
				continue
			}

			if err := p.executor.Run(ctx, p.output, model.Thinking, "-c", line); err != nil {
				return fmt.Errorf("chat message failed: %w", err)
			}
		}
	})
}

// gatherRequirementsScanner uses bufio.Scanner for non-TTY (piped) input.
func (p *Planner) gatherRequirementsScanner(ctx context.Context) error {
	scanner := bufio.NewScanner(p.input)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fmt.Fprint(p.output, "\nsnap plan> ")

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(line, "/done") {
			break
		}
		if line == "" {
			continue
		}

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
