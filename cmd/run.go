package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yarlson/snap/internal/input"
	"github.com/yarlson/snap/internal/pathutil"
	"github.com/yarlson/snap/internal/postrun"
	"github.com/yarlson/snap/internal/provider"
	"github.com/yarlson/snap/internal/session"
	"github.com/yarlson/snap/internal/state"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

var runCmd = &cobra.Command{
	Use:           "run [session]",
	Short:         "Run the task implementation workflow",
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          run,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&prdPath, "prd", "p", "", "Path to PRD file (default: <tasks-dir>/PRD.md)")
	runCmd.Flags().StringVar(&taskFile, "task-file", "", "Path to a single task file to run")
	runCmd.Flags().BoolVar(&freshStart, "fresh", false, "Force fresh start, ignore existing state")
	runCmd.Flags().BoolVar(&showState, "show-state", false, "Show current state and exit")
	runCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON (only with --show-state)")
}

// runConfig holds resolved paths and state manager for a run invocation.
type runConfig struct {
	tasksDir     string
	prdPath      string
	taskFile     string
	displayName  string
	stateManager workflow.StateManager
	userSupplied bool // true when paths come from user-provided flags; false for auto-detected or session-derived paths
}

func run(cmd *cobra.Command, args []string) error {
	var sessionName string
	if len(args) > 0 {
		sessionName = args[0]
	}

	if err := validateRunFlags(cmd, sessionName, taskFile); err != nil {
		return err
	}

	// Handle --show-state flag before provider validation.
	if showState {
		return handleShowState(sessionName, taskFile)
	}

	// Pre-flight: validate provider CLI is available in PATH.
	providerName := provider.ResolveProviderName()
	if err := provider.ValidateCLI(providerName); err != nil {
		return err
	}

	// Pre-flight: detect git remote and validate gh CLI if GitHub.
	remoteURL, err := postrun.DetectRemote()
	if err != nil {
		return fmt.Errorf("failed to detect git remote: %w", err)
	}
	isGitHub := postrun.IsGitHubRemote(remoteURL)
	if isGitHub {
		if err := provider.ValidateGH(); err != nil {
			return err
		}
	}

	// Resolve session or legacy layout.
	rc, err := resolveRunConfig(sessionName, tasksDir, prdPath, taskFile)
	if err != nil {
		return err
	}

	// Validate paths for security (injection, traversal) — only for user-provided flags.
	// Auto-detected and session-derived paths are constructed from validated sources.
	if rc.userSupplied {
		if rc.taskFile != "" {
			if _, err := normalizeTaskFilePath(rc.taskFile); err != nil {
				return fmt.Errorf("invalid task file: %w", err)
			}
		} else {
			if err := pathutil.ValidatePath(rc.tasksDir); err != nil {
				return fmt.Errorf("invalid tasks directory: %w", err)
			}
			if err := pathutil.ValidatePath(rc.prdPath); err != nil {
				return fmt.Errorf("invalid PRD path: %w", err)
			}
		}
	}

	// Check if PRD file exists and warn if not.
	if rc.prdPath != "" {
		if exists, warning := pathutil.CheckPathExists(rc.prdPath); !exists {
			fmt.Fprint(os.Stderr, ui.Info(warning))
		}
	}

	executor, err := provider.NewExecutorFromEnv()
	if err != nil {
		return err
	}
	isTTY := input.IsTerminal(os.Stdin)

	config := workflow.Config{
		TasksDir:     rc.tasksDir,
		PRDPath:      rc.prdPath,
		TaskFilePath: rc.taskFile,
		FreshStart:   freshStart,
		ProviderName: providerName,
		IsTTY:        isTTY,
		DisplayName:  rc.displayName,
		RemoteURL:    remoteURL,
		IsGitHub:     isGitHub,
	}

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

	runnerOpts = append(runnerOpts, workflow.WithStateManager(rc.stateManager))

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
				w, _, err := term.GetSize(int(os.Stdout.Fd()))
				if err == nil {
					im.SetTermWidth(w)
				}
			}
		}()
		defer signal.Stop(winchChan)

		// Set initial terminal width.
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
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

func validateRunFlags(cmd *cobra.Command, sessionName, taskFilePath string) error {
	if taskFilePath == "" {
		return nil
	}
	if sessionName != "" {
		return fmt.Errorf("--task-file cannot be used with a session name")
	}
	if cmd.Flags().Changed("prd") {
		return fmt.Errorf("--task-file cannot be used with --prd")
	}
	if cmd.Flags().Changed("tasks-dir") {
		return fmt.Errorf("--task-file cannot be used with --tasks-dir")
	}
	return nil
}

func normalizeTaskFilePath(path string) (string, error) {
	if strings.Contains(path, "\n") || strings.Contains(path, "\r") {
		return "", fmt.Errorf("path contains invalid characters (newline)")
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	return absPath, nil
}

func resolveExistingTaskFilePath(path string) (string, error) {
	absPath, err := normalizeTaskFilePath(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("task file not found: %s", absPath)
		}
		return "", fmt.Errorf("task file not accessible: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("task file must be a file: %s", absPath)
	}
	return absPath, nil
}

func resolveTaskFileRun(path string) (*runConfig, error) {
	absPath, err := resolveExistingTaskFilePath(path)
	if err != nil {
		return nil, err
	}

	return &runConfig{
		tasksDir:     filepath.Dir(absPath),
		prdPath:      "",
		taskFile:     absPath,
		displayName:  absPath,
		stateManager: newAdhocStateManager(absPath),
		userSupplied: true,
	}, nil
}

func newAdhocStateManager(taskFilePath string) workflow.StateManager {
	sum := sha256.Sum256([]byte(taskFilePath))
	stateDir := filepath.Join(".snap", "adhoc", hex.EncodeToString(sum[:]))
	return state.NewManagerInDir(stateDir)
}

// resolveRunConfig determines the tasks directory, PRD path, display name, and
// state manager based on the session name (or auto-detection/legacy fallback).
func resolveRunConfig(sessionName, flagTasksDir, flagPRDPath, flagTaskFile string) (*runConfig, error) {
	if flagTaskFile != "" {
		return resolveTaskFileRun(flagTaskFile)
	}
	if sessionName != "" {
		return resolveNamedSession(sessionName)
	}

	// Auto-detect: check for sessions, then fall back to legacy layout.
	sessions, err := session.List(".")
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	switch len(sessions) {
	case 0:
		return resolveLegacyFallback(flagTasksDir, flagPRDPath)
	case 1:
		return resolveNamedSession(sessions[0].Name)
	default:
		return nil, formatMultipleSessionsError(sessions)
	}
}

// resolveNamedSession resolves paths for a named session.
func resolveNamedSession(name string) (*runConfig, error) {
	_, err := session.Resolve(".", name)
	if err != nil {
		return nil, err
	}
	td := session.TasksDir(".", name)
	return &runConfig{
		tasksDir:     td,
		prdPath:      filepath.Join(td, "PRD.md"),
		displayName:  name,
		stateManager: state.NewManagerInDir(session.Dir(".", name)),
		userSupplied: false,
	}, nil
}

// resolveLegacyFallback checks for a legacy layout (tasks directory exists
// or existing .snap/state.json) and returns a legacy run config.
func resolveLegacyFallback(flagTasksDir, flagPRDPath string) (*runConfig, error) {
	legacyManager := state.NewManager()
	if legacyManager.Exists() || dirExists(flagTasksDir) {
		return &runConfig{
			tasksDir:     flagTasksDir,
			prdPath:      pathutil.ResolvePRDPath(flagTasksDir, flagPRDPath),
			displayName:  flagTasksDir,
			stateManager: legacyManager,
			userSupplied: true,
		}, nil
	}
	if err := session.EnsureDefault("."); err != nil {
		return nil, err
	}
	return resolveNamedSession("default")
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// formatMultipleSessionsError builds an error message listing available sessions.
func formatMultipleSessionsError(sessions []session.Info) error {
	var b strings.Builder
	b.WriteString("Error: multiple sessions found\n\nAvailable sessions:\n")
	for _, s := range sessions {
		fmt.Fprintf(&b, "  %-12s  %s\n", s.Name, formatTaskSummary(s.TaskCount, s.CompletedCount))
	}
	b.WriteString("\nSpecify a session:\n  snap run <name>")
	return fmt.Errorf("%s", b.String())
}

// handleShowState displays workflow state for the resolved session or legacy layout.
func handleShowState(sessionName, taskFilePath string) error {
	sm, err := resolveStateManager(sessionName, taskFilePath)
	if err != nil {
		return err
	}

	if !sm.Exists() {
		fmt.Print(ui.Info("No state file exists"))
		return nil
	}

	workflowState, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(workflowState, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal state: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(workflowState.Summary(workflow.StepName))
	return nil
}

// resolveStateManager returns a state manager for the given session or legacy layout.
// Returns an error if a session name is explicitly provided but the session does not exist.
func resolveStateManager(sessionName, taskFilePath string) (workflow.StateManager, error) {
	if taskFilePath != "" {
		absPath, err := resolveExistingTaskFilePath(taskFilePath)
		if err != nil {
			return nil, err
		}
		return newAdhocStateManager(absPath), nil
	}
	if sessionName != "" {
		dir, err := session.Resolve(".", sessionName)
		if err != nil {
			return nil, err
		}
		return state.NewManagerInDir(dir), nil
	}

	// Auto-detect for show-state: if exactly one session exists, use it.
	sessions, err := session.List(".")
	if err == nil {
		switch len(sessions) {
		case 0:
			// Legacy fallback: if a legacy state file or task directory exists, use the legacy manager.
			legacyManager := state.NewManager()
			if legacyManager.Exists() || dirExists("docs/tasks") {
				return legacyManager, nil
			}
			if err := session.EnsureDefault("."); err != nil {
				return nil, err
			}
			return state.NewManagerInDir(session.Dir(".", "default")), nil
		case 1:
			return state.NewManagerInDir(session.Dir(".", sessions[0].Name)), nil
		}
	}

	return state.NewManager(), nil
}
