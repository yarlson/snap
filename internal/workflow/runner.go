package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/prompts"
	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/snapshot"
	"github.com/yarlson/snap/internal/state"
	"github.com/yarlson/snap/internal/ui"
)

// workflowStepCount is the number of steps in the iteration workflow.
// Keep in sync with the steps slice in runIteration.
const workflowStepCount = 10

// StepCount returns the number of steps in the iteration workflow.
func StepCount() int {
	return workflowStepCount
}

// Config holds workflow configuration.
type Config struct {
	TasksDir     string
	PRDPath      string
	FreshStart   bool   // Force fresh start, ignore existing state
	ProviderName string // Provider display name (e.g. "claude", "codex")
	IsTTY        bool   // Whether stdout is a terminal
	DisplayName  string // For startup summary (session name or tasks dir path); falls back to TasksDir if empty
}

// StateManager defines the interface for state management, used in tests for dependency injection.
type StateManager interface {
	Load() (*state.State, error)
	Save(state *state.State) error
	Reset() error
	Exists() bool
}

// Runner orchestrates the task implementation workflow.
type Runner struct {
	stepRunner   *StepRunner
	config       Config
	stateManager StateManager
	snapshotter  *snapshot.Snapshotter
	promptQueue  *queue.Queue
	stepContext  *StepContext
	output       io.Writer
}

// NewRunner creates a new workflow runner. Output defaults to os.Stdout.
// Note: snapshots are disabled by default to avoid side effects in tests.
// Enable snapshots with WithSnapshotter() when snapshots are desired.
func NewRunner(executor Executor, config Config, opts ...RunnerOption) *Runner {
	r := &Runner{
		config:       config,
		stateManager: state.NewManager(),
		snapshotter:  nil,
		promptQueue:  queue.New(),
		stepContext:  NewStepContext(),
		output:       os.Stdout,
	}
	for _, opt := range opts {
		opt(r)
	}
	r.stepRunner = NewStepRunner(executor, r.output)
	return r
}

// RunnerOption configures optional Runner behavior.
type RunnerOption func(*Runner)

// WithOutput sets the writer for all workflow output. When using a SwitchWriter,
// this routes all output through the switchable buffer for modal input support.
func WithRunnerOutput(w io.Writer) RunnerOption {
	return func(r *Runner) {
		r.output = w
	}
}

// WithStateManager overrides the default state manager.
// Useful for testing with a custom state directory.
func WithStateManager(m StateManager) RunnerOption {
	return func(r *Runner) {
		r.stateManager = m
	}
}

// WithSnapshotter overrides the default snapshotter.
// Useful for testing with a custom working directory.
func WithSnapshotter(s *snapshot.Snapshotter) RunnerOption {
	return func(r *Runner) {
		r.snapshotter = s
	}
}

// Queue returns the runner's prompt queue for wiring to an input reader.
func (r *Runner) Queue() *queue.Queue {
	return r.promptQueue
}

// StepContext returns the runner's step context for queue UI display.
func (r *Runner) StepContext() *StepContext {
	return r.stepContext
}

// Run executes the task implementation loop until interrupted.
func (r *Runner) Run(ctx context.Context) error {
	// Set up signal handling: cancel context on SIGINT/SIGTERM, letting the
	// main goroutine exit through its normal defer chain. This ensures all
	// deferred cleanup (terminal restore, signal cleanup) runs before exit.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		// Write the interrupted message, bypassing a potentially-paused buffer
		// (e.g., SwitchWriter in paused mode when user is composing input).
		// Use Direct() if available to ensure the message is always visible.
		// After cancel(), the main goroutine will return through its defer chain,
		// ensuring terminal cleanup runs.
		currentState, err := r.stateManager.Load()
		var msg string
		if err == nil && currentState != nil {
			msg = ui.InterruptedWithContext("Stopped by user", currentState.CurrentStep, currentState.TotalSteps)
		} else {
			msg = ui.Interrupted("Stopped by user")
		}

		if sw, ok := r.output.(*ui.SwitchWriter); ok {
			//nolint:errcheck // Best-effort flush of interrupted message; signal handler context.
			_, _ = sw.Direct([]byte(msg))
		} else {
			fmt.Fprint(r.output, msg)
		}
		cancel()
		// After signal.Stop (deferred above), a second SIGINT gets Go's
		// default behavior: immediate process termination.
	}()

	// Handle fresh start flag
	if r.config.FreshStart && r.stateManager.Exists() {
		fmt.Fprint(r.output, ui.Info("Fresh start requested, deleting existing state"))
		if err := r.stateManager.Reset(); err != nil {
			return fmt.Errorf("failed to reset state: %w", err)
		}
	}

	// Load existing state
	workflowState, err := r.stateManager.Load()
	if err != nil {
		// Corrupt state - warn user and start fresh
		fmt.Fprint(r.output, ui.Interrupted(fmt.Sprintf("State file corrupt (%v), starting fresh", err)))
		if err := r.stateManager.Reset(); err != nil {
			return fmt.Errorf("failed to reset corrupt state: %w", err)
		}
		workflowState = nil
	}

	// Initialize state if needed
	if workflowState == nil {
		workflowState = state.NewState(r.config.TasksDir, r.config.PRDPath, workflowStepCount)
	}

	// Resolve startup target: resume active task or select next.
	target, err := resolveStartup(workflowState, r.config.TasksDir, workflowStepCount)
	if err != nil {
		return fmt.Errorf("cannot resume: %w", err)
	}

	isResume := target.action == actionResume

	switch target.action {
	case actionResume:
		if workflowState.LastError != "" {
			fmt.Fprint(r.output, ui.Interrupted(fmt.Sprintf("Last error: %s", workflowState.LastError)))
		}
	case actionSelect:
		done, err := r.selectIdleTask(workflowState)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}

	// Print startup summary.
	// For resume, reuse scanned tasks from resolveStartup to avoid redundant I/O.
	// For select action, scan now to get task count.
	var taskCount int
	if isResume {
		taskCount = len(target.tasks)
	} else {
		tasks, err := ScanTasks(r.config.TasksDir)
		if err != nil {
			return fmt.Errorf("failed to scan tasks for summary: %w", err)
		}
		taskCount = len(tasks)
	}
	doneCount := len(workflowState.CompletedTaskIDs)
	var action string
	if isResume {
		action = fmt.Sprintf("resuming %s from step %d", target.taskID, target.step)
	} else {
		action = fmt.Sprintf("starting %s", workflowState.CurrentTaskID)
	}
	displayName := r.config.TasksDir
	if r.config.DisplayName != "" {
		displayName = r.config.DisplayName
	}
	fmt.Fprintln(r.output, ui.FormatStartupSummary(displayName, r.config.ProviderName, taskCount, doneCount, action))

	// Print prompt hint on fresh start with TTY (suppress on resume).
	if !isResume && r.config.IsTTY {
		fmt.Fprint(r.output, ui.Info("Type a directive and press Enter to queue it between steps"))
	}

	// Run the workflow loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			iterationComplete, err := r.runIteration(ctx, workflowState)
			if err != nil {
				// If context was cancelled (e.g., by signal handler), return
				// the context error so the caller can map it to exit code 130.
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Save error state
				workflowState.MarkStepFailed(err)
				if saveErr := r.stateManager.Save(workflowState); saveErr != nil {
					fmt.Fprint(r.output, ui.Interrupted(fmt.Sprintf("Failed to save error state: %v", saveErr)))
				}
				return fmt.Errorf("iteration failed: %w", err)
			}

			if iterationComplete {
				// Select next task for the next iteration.
				// selectIdleTask handles the "all complete" case.
				done, err := r.selectIdleTask(workflowState)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
			}
		}
	}
}

// selectIdleTask scans for TASK<n>.md files and selects the next incomplete task.
// It updates the state with the selected task and saves it. Returns (true, nil) when
// all tasks are complete (caller should exit cleanly).
func (r *Runner) selectIdleTask(workflowState *state.State) (bool, error) {
	tasks, err := ScanTasks(r.config.TasksDir)
	if err != nil {
		return false, fmt.Errorf("failed to scan tasks: %w", err)
	}
	if len(tasks) == 0 {
		hints := DiagnoseEmptyTaskDir(r.config.TasksDir)
		return false, fmt.Errorf("%s", FormatTaskDirError(r.config.TasksDir, hints))
	}

	next := SelectNextTask(tasks, workflowState.CompletedTaskIDs)
	if next == nil {
		// All discovered tasks are completed.
		fmt.Fprint(r.output, ui.Complete("All tasks implemented!"))
		if err := r.stateManager.Reset(); err != nil {
			fmt.Fprint(r.output, ui.Interrupted(fmt.Sprintf("Warning: failed to clean up state: %v", err)))
		}
		return true, nil
	}

	workflowState.CurrentTaskID = next.ID
	workflowState.CurrentTaskFile = next.Filename
	if err := r.stateManager.Save(workflowState); err != nil {
		return false, fmt.Errorf("failed to save state after task selection: %w", err)
	}

	return false, nil
}

func (r *Runner) runIteration(ctx context.Context, workflowState *state.State) (bool, error) {
	taskStart := time.Now()
	taskLabel := workflowState.CurrentTaskID
	if taskLabel == "" {
		taskLabel = "next task"
	}
	// Generate a one-line task description via fast model (best-effort).
	var description string
	if workflowState.CurrentTaskFile != "" {
		taskFilePath := filepath.Join(r.config.TasksDir, workflowState.CurrentTaskFile)
		if content, err := os.ReadFile(taskFilePath); err == nil {
			// Truncate to first 2000 bytes to avoid sending large files to LLM.
			taskContent := string(content)
			if len(taskContent) > 2000 {
				taskContent = taskContent[:2000]
			}
			if prompt, err := prompts.TaskSummary(prompts.TaskSummaryData{TaskContent: taskContent}); err == nil {
				var buf strings.Builder
				if err := r.stepRunner.executor.Run(ctx, &buf, model.Fast, prompt); err == nil {
					description = ui.StripColors(strings.TrimSpace(buf.String()))
				}
			}
		}
	}

	fmt.Fprint(r.output, ui.Header(fmt.Sprintf("Implementing %s", taskLabel), description))

	// Build the Step 1 prompt based on whether a specific task is targeted.
	implementData := prompts.ImplementData{
		PRDPath: r.config.PRDPath,
	}
	if workflowState.CurrentTaskFile != "" {
		implementData.TaskPath = filepath.Join(r.config.TasksDir, workflowState.CurrentTaskFile)
		implementData.TaskID = workflowState.CurrentTaskID
	}
	implementPrompt, err := prompts.Implement(implementData)
	if err != nil {
		return false, fmt.Errorf("failed to render implement prompt: %w", err)
	}

	ensureCompletenessPrompt, err := prompts.EnsureCompleteness(prompts.EnsureCompletenessData{
		TaskPath: implementData.TaskPath,
		TaskID:   implementData.TaskID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to render ensure-completeness prompt: %w", err)
	}

	steps := []struct {
		name   string
		prompt string
		args   []string
		model  model.Type
	}{
		{
			name:   fmt.Sprintf("Implement %s", taskLabel),
			prompt: implementPrompt,
			model:  model.Thinking,
		},
		{
			name:   "Ensure completeness",
			prompt: ensureCompletenessPrompt,
			model:  model.Thinking,
		},
		{
			name:   "Lint & test",
			prompt: prompts.LintAndTest(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
		{
			name:   "Code review",
			prompt: prompts.CodeReview(),
			model:  model.Thinking,
		},
		{
			name:   "Apply fixes",
			prompt: prompts.ApplyFixes(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
		{
			name:   "Verify fixes",
			prompt: prompts.LintAndTest(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
		{
			name:   "Update docs",
			prompt: prompts.UpdateDocs(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
		{
			name:   "Commit code",
			prompt: prompts.Commit(),
			model:  model.Fast,
		},
		{
			name:   "Update memory",
			prompt: prompts.MemoryUpdate(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
		{
			name:   "Commit memory",
			prompt: prompts.Commit(),
			args:   []string{"-c"},
			model:  model.Fast,
		},
	}

	// Resume from current step
	startStep := workflowState.CurrentStep
	if startStep > 1 {
		fmt.Fprint(r.output, ui.Info(fmt.Sprintf("Resuming from step %d: %s", startStep, steps[startStep-1].name)))
	}

	totalSteps := len(steps)
	if totalSteps != workflowStepCount {
		return false, fmt.Errorf("internal error: workflowStepCount(%d) != len(steps)(%d); update the constant", workflowStepCount, totalSteps)
	}

	// Ensure state has correct total steps (handles state from older versions or fresh start)
	if workflowState.TotalSteps != totalSteps {
		workflowState.TotalSteps = totalSteps
		if err := r.stateManager.Save(workflowState); err != nil {
			return false, fmt.Errorf("failed to update total steps in state: %w", err)
		}
	}

	for stepNum := startStep; stepNum <= totalSteps; stepNum++ {
		// Check for context cancellation before starting each step.
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		step := steps[stepNum-1]

		// Update step context for queue UI display.
		r.stepContext.Set(stepNum, totalSteps, step.name)

		// Determine if this step should have no-commit suffix
		var prompt string
		if strings.Contains(step.name, "Commit") {
			prompt = BuildPrompt(step.prompt)
		} else {
			prompt = BuildPrompt(step.prompt, WithNoCommit())
		}

		// Build full args with prompt
		fullArgs := make([]string, 0, len(step.args)+1)
		fullArgs = append(fullArgs, step.args...)
		fullArgs = append(fullArgs, prompt)

		// Execute step with numbering
		if err := r.stepRunner.RunStepNumbered(ctx, stepNum, totalSteps, step.name, step.model, fullArgs...); err != nil {
			return false, err
		}

		// Capture a snapshot of the working tree after this step (if snapshotter is enabled).
		// Skip snapshots for commit steps (tree is clean after commit, no-op operation).
		if r.snapshotter != nil && !strings.Contains(step.name, "Commit") {
			snapMsg := fmt.Sprintf("snap: %s step %d/%d — %s", taskLabel, stepNum, totalSteps, step.name)
			if created, snapErr := r.snapshotter.Capture(ctx, snapMsg); snapErr != nil {
				fmt.Fprintf(r.output, "  snapshot skipped: %v\n", snapErr)
			} else if created {
				fmt.Fprintln(r.output, "  snapshot saved")
			}
		}

		// Drain queued user prompts between steps.
		if errs := DrainQueue(ctx, r.output, r.stepRunner, r.promptQueue); len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "%d queued prompt(s) failed\n", len(errs))
		}

		// Mark step complete and save state
		workflowState.MarkStepComplete()
		if err := r.stateManager.Save(workflowState); err != nil {
			return false, fmt.Errorf("failed to save state after step %d: %w", stepNum, err)
		}
	}

	// Task complete - mark as completed and reset to idle.
	fmt.Fprint(r.output, ui.CompleteWithDuration("Iteration complete", time.Since(taskStart)))

	if id := workflowState.CurrentTaskID; id != "" {
		alreadyCompleted := false
		for _, cid := range workflowState.CompletedTaskIDs {
			if cid == id {
				alreadyCompleted = true
				break
			}
		}
		if !alreadyCompleted {
			workflowState.CompletedTaskIDs = append(workflowState.CompletedTaskIDs, id)
		}
	}
	workflowState.CurrentTaskID = ""
	workflowState.CurrentTaskFile = ""
	workflowState.CurrentStep = 1
	workflowState.LastError = ""
	workflowState.SessionID = ""
	workflowState.LastUpdated = time.Now()

	if err := r.stateManager.Save(workflowState); err != nil {
		return false, fmt.Errorf("failed to save state after completion: %w", err)
	}

	return true, nil
}
