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

	"github.com/yarlson/tap"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

// Planner orchestrates the two-phase planning pipeline.
type Planner struct {
	executor          workflow.Executor
	sessionName       string
	tasksDir          string
	output            io.Writer
	input             io.Reader
	interactive       bool         // when true, uses tap for interactive TTY input
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

// WithInteractive enables interactive input via tap during Phase 1.
// When true, the planner uses tap.Textarea for styled multiline TTY input
// with Ctrl+C/Escape abort support.
func WithInteractive(interactive bool) PlannerOption {
	return func(p *Planner) { p.interactive = interactive }
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
		fmt.Fprint(p.output, ui.Step(fmt.Sprintf("Planning session '%s' — using %s as input", p.sessionName, p.briefFile)))
	case p.resume:
		fmt.Fprint(p.output, ui.Step(fmt.Sprintf("Resuming planning for session '%s'", p.sessionName)))
	default:
		fmt.Fprint(p.output, ui.Step(fmt.Sprintf("Planning session '%s'", p.sessionName)))
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
	fmt.Fprint(p.output, ui.Step("Gathering requirements — type /done when ready"))

	// Send the initial requirements-gathering prompt.
	// When resuming, add -c flag to continue previous conversation.
	prompt, err := RenderRequirementsPrompt()
	if err != nil {
		return fmt.Errorf("requirements prompt failed: %w", err)
	}
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
	if p.interactive {
		return p.gatherRequirementsInteractive(ctx)
	}
	return p.gatherRequirementsScanner(ctx)
}

// gatherRequirementsInteractive uses tap.Textarea for interactive TTY input.
// Ctrl+C or Escape returns context.Canceled to abort the plan command.
func (p *Planner) gatherRequirementsInteractive(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fmt.Fprint(p.output, "\n")

		result := tap.Textarea(ctx, tap.TextareaOptions{
			Message:     "Your response",
			Placeholder: "Describe your requirements, or /done to finish",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("enter a message, or /done to finish")
				}
				return nil
			},
		})

		if ctx.Err() != nil {
			return ctx.Err()
		}
		// tap.Text returns empty string when user aborts (Ctrl+C or Escape).
		// The Validate func ensures non-empty input on normal submission.
		if result == "" {
			return context.Canceled
		}

		result = strings.TrimSpace(result)
		if strings.EqualFold(result, "/done") {
			return nil
		}

		if err := p.executor.Run(ctx, p.output, model.Thinking, "-c", result); err != nil {
			return fmt.Errorf("chat message failed: %w", err)
		}
	}
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
// Pipeline: PRD (sequential) → TECHNOLOGY + DESIGN (parallel) → 4-step task splitting (sequential)
// → parallel batched TASK<N>.md generation.
func (p *Planner) generateDocuments(ctx context.Context) error {
	fmt.Fprint(p.output, ui.Step("Generating planning documents..."))

	const totalSteps = 7
	const taskFileBatchSize = 5

	// --- Step 1/7: Generate PRD (sequential) ---
	if ctx.Err() != nil {
		fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 1/%d", totalSteps)))
		fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
		return ctx.Err()
	}

	prdPrompt, err := RenderPRDPrompt(p.tasksDir, p.briefBody)
	if err != nil {
		return fmt.Errorf("failed to render Generate PRD prompt: %w", err)
	}

	var prdArgs []string
	// In --from mode, first call starts a fresh conversation (no -c).
	// Otherwise, -c continues from Phase 1 conversation.
	if p.briefBody == "" {
		prdArgs = append(prdArgs, "-c")
	}
	prdArgs = append(prdArgs, prdPrompt)

	fmt.Fprint(p.output, ui.StepNumbered(1, totalSteps, "Generate PRD"))

	start := time.Now()
	if err := p.executor.Run(ctx, p.output, model.Thinking, prdArgs...); err != nil {
		elapsed := time.Since(start)
		fmt.Fprintln(p.output, ui.StepFailed("Step failed", elapsed))

		if ctx.Err() != nil {
			fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 1/%d", totalSteps)))
			fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
			return ctx.Err()
		}
		return fmt.Errorf("step 1/%d %q failed: %w", totalSteps, "Generate PRD", err)
	}

	if err := p.onFirstMessage(); err != nil {
		return err
	}

	fmt.Fprintln(p.output, ui.StepComplete("Step complete", time.Since(start)))

	// --- Step 2/7: Generate technology plan + design spec (parallel) ---
	if ctx.Err() != nil {
		fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 2/%d", totalSteps)))
		fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
		return ctx.Err()
	}

	techPrompt, err := RenderTechnologyPrompt(p.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to render technology prompt: %w", err)
	}

	designPrompt, err := RenderDesignPrompt(p.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to render design prompt: %w", err)
	}

	tasks := []parallelTask{
		{name: "Technology plan", modelType: model.Thinking, args: []string{techPrompt}},
		{name: "Design spec", modelType: model.Thinking, args: []string{designPrompt}},
	}

	fmt.Fprint(p.output, ui.StepNumbered(2, totalSteps, "Generate technology plan + design spec"))

	results := runParallel(ctx, p.executor, tasks, 0)

	// Print sub-step results and check for failures.
	var parallelFailed bool
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintln(p.output, ui.StepFailed(r.name, r.elapsed))
			parallelFailed = true
		} else {
			fmt.Fprintln(p.output, ui.StepComplete(r.name, r.elapsed))
		}
	}

	if parallelFailed {
		if ctx.Err() != nil {
			fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 2/%d", totalSteps)))
			fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
			return ctx.Err()
		}
		var errs []string
		for _, r := range results {
			if r.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", r.name, r.err))
			}
		}
		return fmt.Errorf("step 2/%d failed: %s", totalSteps, strings.Join(errs, "; "))
	}

	// --- Steps 3–6: Multi-pass task splitting (sequential, conversation chain) ---
	type taskSplitStep struct {
		num                  int
		name                 string
		continueConversation bool   // true → pass -c to continue conversation chain
		prompt               string // rendered prompt content
	}

	// Step 3: Create task list — fresh conversation (no -c), preamble included via render function.
	createPrompt, err := RenderCreateTasksPrompt(p.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to render Create task list prompt: %w", err)
	}

	// Step 4: Assess tasks — continues conversation (-c), no preamble needed (already in context from step 3).
	assessPrompt, err := RenderAssessTasksPrompt()
	if err != nil {
		return fmt.Errorf("failed to render Assess tasks prompt: %w", err)
	}

	// Step 5: Refine tasks — continues conversation (-c), no preamble needed.
	mergePrompt, err := RenderMergeTasksPrompt()
	if err != nil {
		return fmt.Errorf("failed to render Refine tasks prompt: %w", err)
	}

	// Step 6: Generate task summary — continues conversation (-c), preamble included via render function.
	summaryPrompt, err := RenderGenerateTaskSummaryPrompt(p.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to render Generate task summary prompt: %w", err)
	}

	splitSteps := []taskSplitStep{
		{num: 3, name: "Create task list", continueConversation: false, prompt: createPrompt},
		{num: 4, name: "Assess tasks", continueConversation: true, prompt: assessPrompt},
		{num: 5, name: "Refine tasks", continueConversation: true, prompt: mergePrompt},
		{num: 6, name: "Generate task summary", continueConversation: true, prompt: summaryPrompt},
	}

	for _, step := range splitSteps {
		if ctx.Err() != nil {
			fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step %d/%d", step.num, totalSteps)))
			fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
			return ctx.Err()
		}

		fmt.Fprint(p.output, ui.StepNumbered(step.num, totalSteps, step.name))

		var args []string
		if step.continueConversation {
			args = append(args, "-c")
		}
		args = append(args, step.prompt)

		start = time.Now()
		if err := p.executor.Run(ctx, p.output, model.Thinking, args...); err != nil {
			elapsed := time.Since(start)
			fmt.Fprintln(p.output, ui.StepFailed("Step failed", elapsed))

			if ctx.Err() != nil {
				fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step %d/%d", step.num, totalSteps)))
				fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
				return ctx.Err()
			}
			return fmt.Errorf("step %d/%d %q failed: %w", step.num, totalSteps, step.name, err)
		}

		fmt.Fprintln(p.output, ui.StepComplete("Step complete", time.Since(start)))
	}

	// --- Step 7/7: Generate TASK<N>.md files (parallel, batched) ---
	if ctx.Err() != nil {
		fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 7/%d", totalSteps)))
		fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
		return ctx.Err()
	}

	taskSpecs, err := ExtractTaskSpecs(p.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to parse TASKS.md for task count: %w", err)
	}

	fmt.Fprint(p.output, ui.StepNumbered(7, totalSteps, "Generate task files"))

	if len(taskSpecs) == 0 {
		fmt.Fprintln(p.output, ui.StepComplete("0 task files generated", time.Duration(0)))
	} else if err := p.generateTaskFiles(ctx, taskSpecs, totalSteps, taskFileBatchSize); err != nil {
		return err
	}

	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, ui.Complete("Planning complete"))

	return nil
}

// generateTaskFiles generates TASK<N>.md files in parallel batches.
func (p *Planner) generateTaskFiles(ctx context.Context, specs []TaskSpec, totalSteps, batchSize int) error {
	// Build parallel tasks for each spec.
	var taskFileTasks []parallelTask
	for _, spec := range specs {
		prompt, err := RenderGenerateTaskFilePrompt(p.tasksDir, spec.Number, spec.Spec)
		if err != nil {
			return fmt.Errorf("failed to render task file prompt for TASK%d: %w", spec.Number, err)
		}
		taskFileTasks = append(taskFileTasks, parallelTask{
			name:      fmt.Sprintf("TASK%d.md", spec.Number),
			modelType: model.Thinking,
			args:      []string{prompt},
		})
	}

	if len(taskFileTasks) <= batchSize {
		// Small batch: run all at once, print simple completion line.
		start := time.Now()
		results := runParallel(ctx, p.executor, taskFileTasks, batchSize)

		var failures []string
		for _, r := range results {
			if r.err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r.name, r.err))
			}
		}

		if len(failures) > 0 {
			fmt.Fprintln(p.output, ui.StepFailed(
				fmt.Sprintf("%d task files generated", len(taskFileTasks)-len(failures)),
				time.Since(start),
			))
			for _, f := range failures {
				fmt.Fprintln(p.output, ui.ErrorWithDetails("Task file generation failed", []string{f}))
			}
			if ctx.Err() != nil {
				fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 7/%d", totalSteps)))
				fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
				return ctx.Err()
			}
			return fmt.Errorf("step 7/%d: %d task file(s) failed: %s", totalSteps, len(failures), strings.Join(failures, "; "))
		}

		fmt.Fprintln(p.output, ui.StepComplete(
			fmt.Sprintf("%d task files generated", len(taskFileTasks)),
			time.Since(start),
		))
		return nil
	}

	// Large batch: split into batches, run sequentially, print batch progress.
	batches := splitBatches(taskFileTasks, batchSize)
	var allFailures []string

	for batchIdx, batch := range batches {
		if ctx.Err() != nil {
			fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 7/%d", totalSteps)))
			fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
			return ctx.Err()
		}

		firstTask := batchIdx * batchSize
		lastTask := firstTask + len(batch) - 1
		batchLabel := fmt.Sprintf("Batch %d/%d (tasks %d\u2013%d)", batchIdx+1, len(batches), firstTask, lastTask)

		start := time.Now()
		results := runParallel(ctx, p.executor, batch, batchSize)

		var failures []string
		for _, r := range results {
			if r.err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r.name, r.err))
			}
		}

		if len(failures) > 0 {
			fmt.Fprintln(p.output, ui.StepFailed(batchLabel, time.Since(start)))
			for _, f := range failures {
				fmt.Fprintln(p.output, ui.ErrorWithDetails(batchLabel, []string{f}))
			}
			allFailures = append(allFailures, failures...)
			if ctx.Err() != nil {
				fmt.Fprintln(p.output, ui.Interrupted(fmt.Sprintf("Planning aborted at step 7/%d", totalSteps)))
				fmt.Fprint(p.output, ui.Info(fmt.Sprintf("  Files written so far are preserved in %s", p.tasksDir)))
				return ctx.Err()
			}
			continue
		}

		fmt.Fprintln(p.output, ui.StepComplete(batchLabel, time.Since(start)))
	}

	if len(allFailures) > 0 {
		return fmt.Errorf("step 7/%d: %d task file(s) failed: %s", totalSteps, len(allFailures), strings.Join(allFailures, "; "))
	}

	return nil
}
