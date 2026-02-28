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
	opts = append(opts, plan.WithOutput(planOutput), plan.WithInput(os.Stdin), plan.WithTerminal(os.Stdin))

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
