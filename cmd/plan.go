package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/input"
	"github.com/yarlson/snap/internal/plan"
	"github.com/yarlson/snap/internal/provider"
	"github.com/yarlson/snap/internal/session"
	"github.com/yarlson/snap/internal/ui"
)

var fromFile string

var planCmd = &cobra.Command{
	Use:           "plan [session]",
	Short:         "Plan tasks for a session interactively",
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          planRun,
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVar(&fromFile, "from", "", "Input file to use instead of interactive requirements gathering")
}

func planRun(_ *cobra.Command, args []string) error {
	sessionName, err := resolvePlanSession(args)
	if err != nil {
		return err
	}

	// Conflict guard: check for existing planning artifacts.
	isTTY := input.IsTerminal(os.Stdin)
	sessionName, err = checkPlanConflict(sessionName, isTTY, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	// Pre-flight: validate provider CLI is available in PATH.
	providerName := provider.ResolveProviderName()
	if err := provider.ValidateCLI(providerName); err != nil {
		return err
	}

	// Read --from file if specified.
	var opts []plan.PlannerOption
	var planOutput io.Writer = os.Stdout
	if input.IsTerminal(os.Stdin) {
		planOutput = ui.NewSwitchWriter(os.Stdout, ui.WithLFToCRLF())
	}
	opts = append(opts, plan.WithOutput(planOutput), plan.WithInput(os.Stdin), plan.WithInteractive(input.IsTerminal(os.Stdin)))

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		opts = append(opts, plan.WithBrief(filepath.Base(fromFile), string(content)))
	}

	executor, err := provider.NewExecutorFromEnv()
	if err != nil {
		return err
	}

	td := session.TasksDir(".", sessionName)

	// Check if this is a resume (marker already exists) or fresh start.
	// Create .plan-started marker after first successful message (not before).
	resumePlan := session.HasPlanHistory(".", sessionName)
	opts = append(opts,
		plan.WithResume(resumePlan),
		plan.WithAfterFirstMessage(func() error {
			return session.MarkPlanStarted(".", sessionName)
		}),
	)

	planner := plan.NewPlanner(executor, sessionName, td, opts...)

	// Set up signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := planner.Run(ctx); err != nil {
		if ctx.Err() != nil {
			// Signal-initiated cancellation — planner already printed abort message.
			return ctx.Err()
		}
		return err
	}

	// Print file listing after completion.
	printFileListing(planOutput, td)

	fmt.Print("\n")
	fmt.Print(ui.Info(fmt.Sprintf("Run: snap run %s", sessionName)))

	return nil
}

// resolvePlanSession resolves the session name for the plan command.
func resolvePlanSession(args []string) (string, error) {
	if len(args) > 0 {
		name := args[0]
		if _, err := session.Resolve(".", name); err != nil {
			return "", err
		}
		return name, nil
	}

	// Auto-detect: exactly one session → use it.
	sessions, err := session.List(".")
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	switch len(sessions) {
	case 0:
		if err := session.EnsureDefault("."); err != nil {
			return "", err
		}
		return "default", nil
	case 1:
		return sessions[0].Name, nil
	default:
		return "", formatMultiplePlanSessionsError(sessions)
	}
}

// formatMultiplePlanSessionsError builds an error message listing available sessions for plan.
func formatMultiplePlanSessionsError(sessions []session.Info) error {
	var b strings.Builder
	b.WriteString("Error: multiple sessions found\n\nAvailable sessions:\n")
	for _, s := range sessions {
		fmt.Fprintf(&b, "  %-12s  %s\n", s.Name, formatTaskSummary(s.TaskCount, s.CompletedCount))
	}
	b.WriteString("\nSpecify a session:\n  snap plan <name>")
	return fmt.Errorf("%s", b.String())
}

// checkPlanConflict detects existing planning artifacts and either prompts
// the user (TTY) or returns an error (non-TTY). Returns the session name to
// proceed with, or an error to abort.
func checkPlanConflict(sessionName string, isTTY bool, stdin io.Reader, stdout io.Writer) (string, error) {
	if !session.HasArtifacts(".", sessionName) {
		return sessionName, nil
	}

	if !isTTY {
		return "", fmt.Errorf(
			"session %q already has planning artifacts\n\n"+
				"To re-plan, clean up first:\n"+
				"  snap delete %s && snap new %s\n\n"+
				"Or plan in a new session:\n"+
				"  snap new <name> && snap plan <name>",
			sessionName, sessionName, sessionName)
	}

	// TTY: display prompt and read single-character choice.
	prompt := fmt.Sprintf("Session %q already has planning artifacts.\n\n"+
		"  [1] Clean up and re-plan this session\n"+
		"  [2] Create a new session\n\n",
		sessionName)
	fmt.Fprint(stdout, prompt)

	label := ui.ResolveStyle(ui.WeightBold) +
		ui.ResolveColor(ui.ColorSecondary) +
		"Choice (1/2): " +
		ui.ResolveStyle(ui.WeightNormal)
	fmt.Fprint(stdout, label)

	buf := make([]byte, 1)
	for {
		n, err := stdin.Read(buf)
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		if n == 0 {
			continue
		}

		switch buf[0] {
		case '1':
			fmt.Fprint(stdout, "1\r\n")
			if err := session.CleanSession(".", sessionName); err != nil {
				return "", fmt.Errorf("clean session: %w", err)
			}
			return sessionName, nil
		case '2':
			fmt.Fprint(stdout, "2\r\n")
			// Drain the trailing newline left by cooked-mode line buffering
			// when the user presses Enter after '2'. Only drain on real terminals (with Fd).
			type fder interface{ Fd() uintptr }
			if _, ok := stdin.(fder); ok {
				drain := make([]byte, 1)
				stdin.Read(drain) //nolint:errcheck // consume stale newline from cooked-mode input
			}
			return promptNewSession(stdin, stdout)
		case 3: // Ctrl+C
			fmt.Fprint(stdout, "\r\n")
			return "", input.ErrInterrupt
		}
		// All other bytes: ignore.
	}
}

// promptNewSession displays a "Session name:" prompt, validates the input,
// creates the session, and returns the new session name. Invalid names and
// existing session names trigger a re-prompt.
func promptNewSession(stdin io.Reader, stdout io.Writer) (string, error) {
	nameLoop := func() (string, error) {
		for {
			name, err := input.ReadLine(stdin, stdout, "Session name: ")
			if err != nil {
				return "", err
			}
			name = strings.TrimSpace(name)

			if err := session.ValidateName(name); err != nil {
				msg := err.Error()
				if msg != "" {
					msg = strings.ToUpper(msg[:1]) + msg[1:]
				}
				fmt.Fprintf(stdout, "%s\r\n", msg)
				continue
			}

			if session.Exists(".", name) {
				fmt.Fprintf(stdout, "Session %q already exists\r\n", name)
				continue
			}

			if err := session.Create(".", name); err != nil {
				return "", err
			}
			return name, nil
		}
	}

	// Enter raw mode if stdin supports it (real terminal).
	// In tests, stdin is a bytes.Reader and raw mode is not needed.
	type fder interface{ Fd() uintptr }
	if f, ok := stdin.(fder); ok {
		var result string
		err := input.WithRawMode(int(f.Fd()), func() error {
			var innerErr error
			result, innerErr = nameLoop()
			return innerErr
		})
		return result, err
	}
	return nameLoop()
}

// printFileListing prints all files found in the tasks directory.
func printFileListing(w io.Writer, tasksDir string) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	if len(files) == 0 {
		return
	}

	sort.Strings(files)
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, ui.Info(fmt.Sprintf("Files in %s:", tasksDir)))
	for _, f := range files {
		fmt.Fprint(w, ui.Info("  "+f))
	}
}
