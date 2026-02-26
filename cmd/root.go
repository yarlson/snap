package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yarlson/snap/internal/input"
	"github.com/yarlson/snap/internal/pathutil"
	"github.com/yarlson/snap/internal/provider"
	"github.com/yarlson/snap/internal/state"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

var (
	tasksDir   string
	prdPath    string
	freshStart bool
	showState  bool
)

var rootCmd = &cobra.Command{
	Use:           "snap",
	Short:         "Autonomous task-by-task implementation tool",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `snap automates the task-by-task implementation workflow:
- Reads next unimplemented task from PRD
- Implements the task
- Validates with linters and tests
- Reviews code changes
- Commits changes
- Updates memory vault

Runs continuously until interrupted with Ctrl+C.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVarP(&tasksDir, "tasks-dir", "d", "docs/tasks", "Directory containing PRD and task files")
	rootCmd.Flags().StringVarP(&prdPath, "prd", "p", "", "Path to PRD file (default: <tasks-dir>/PRD.md)")
	rootCmd.Flags().BoolVar(&freshStart, "fresh", false, "Force fresh start, ignore existing state")
	rootCmd.Flags().BoolVar(&showState, "show-state", false, "Show current state and exit")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	// Handle --show-state flag
	if showState {
		return handleShowState()
	}

	// Resolve paths with defaults from tasks directory
	prdPath = pathutil.ResolvePRDPath(tasksDir, prdPath)

	// Validate paths for security (injection, traversal)
	if err := pathutil.ValidatePath(tasksDir); err != nil {
		return fmt.Errorf("invalid tasks directory: %w", err)
	}
	if err := pathutil.ValidatePath(prdPath); err != nil {
		return fmt.Errorf("invalid PRD path: %w", err)
	}

	// Check if files exist and warn if not
	if exists, warning := pathutil.CheckPathExists(prdPath); !exists {
		fmt.Fprintln(os.Stderr, warning)
	}

	executor, err := provider.NewExecutorFromEnv()
	if err != nil {
		return err
	}
	config := workflow.Config{
		TasksDir:   tasksDir,
		PRDPath:    prdPath,
		FreshStart: freshStart,
	}

	isTTY := input.IsTerminal(os.Stdin)

	// When running in a TTY, create a SwitchWriter for modal input support.
	// All workflow output routes through the SwitchWriter so it can be paused
	// during user input composing and flushed on submit/cancel.
	var runnerOpts []workflow.RunnerOption
	var sw *ui.SwitchWriter
	if isTTY {
		swOpts := []ui.SwitchWriterOption{}
		if input.IsTerminal(os.Stdout) {
			swOpts = append(swOpts, ui.WithLFToCRLF())
		}
		sw = ui.NewSwitchWriter(os.Stdout, swOpts...)
		runnerOpts = append(runnerOpts, workflow.WithRunnerOutput(sw))
	}

	runner := workflow.NewRunner(executor, config, runnerOpts...)

	// Start reading user prompts from stdin in background (TTY only).
	// Raw terminal mode suppresses echo to prevent garbled output during streaming.
	// Modal input: first keystroke pauses output and shows input prompt;
	// Enter submits, Escape cancels, both flush buffered output and resume.
	if isTTY {
		im := input.NewMode(sw)

		// Handle terminal resize (SIGWINCH) to update input mode width.
		winchChan := make(chan os.Signal, 1)
		signal.Notify(winchChan, syscall.SIGWINCH)
		go func() {
			for range winchChan {
				w, _, err := term.GetSize(int(os.Stdout.Fd())) //nolint:gosec // G115: fd fits int
				if err == nil {
					im.SetTermWidth(w)
				}
			}
		}()
		defer signal.Stop(winchChan)

		// Set initial terminal width.
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil { //nolint:gosec // G115: fd fits int
			im.SetTermWidth(w)
		}

		stdinReader := input.NewReader(os.Stdin, runner.Queue(),
			input.WithTerminal(os.Stdin),
			input.WithOutput(sw),
			input.WithStepInfo(runner.StepContext()),
			input.WithMode(im),
		)
		stdinReader.Start()
		defer stdinReader.Stop()
	}

	return runner.Run(context.Background())
}

func handleShowState() error {
	stateManager := state.NewManager()

	if !stateManager.Exists() {
		fmt.Println("No state file exists")
		return nil
	}

	workflowState, err := stateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Pretty-print the state as JSON
	data, err := json.MarshalIndent(workflowState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
