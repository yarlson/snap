package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/session"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

var statusCmd = &cobra.Command{
	Use:           "status [session]",
	Short:         "Show detailed status for a session",
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          statusRun,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func statusRun(cmd *cobra.Command, args []string) error {
	sessionName, err := resolveStatusSession(args)
	if err != nil {
		return err
	}

	st, err := session.Status(".", sessionName)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	fmt.Fprint(out, ui.KeyValue("Session", st.Name))
	fmt.Fprint(out, ui.KeyValue("Path   ", st.TasksDir))

	if len(st.Tasks) == 0 {
		fmt.Fprintln(out)
		fmt.Fprint(out, ui.Info(fmt.Sprintf("No task files found. Run: snap plan %s", st.Name)))
		return nil
	}

	fmt.Fprintln(out)
	fmt.Fprint(out, ui.Info("Tasks:"))

	completedCount := 0
	for _, task := range st.Tasks {
		switch {
		case task.Completed:
			completedCount++
			fmt.Fprint(out, ui.TaskDone(task.ID))
		case task.ID == st.ActiveTask && st.ActiveStep > 0:
			suffix := fmt.Sprintf("step %d/%d: %s", st.ActiveStep, st.TotalSteps, workflow.StepName(st.ActiveStep))
			fmt.Fprint(out, ui.TaskActive(task.ID, suffix))
		default:
			fmt.Fprint(out, ui.TaskPending(task.ID))
		}
	}

	remaining := len(st.Tasks) - completedCount
	fmt.Fprintln(out)
	fmt.Fprint(out, ui.Info(fmt.Sprintf("%d tasks remaining, %d complete", remaining, completedCount)))

	return nil
}

// resolveStatusSession resolves the session name for the status command.
func resolveStatusSession(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	// Auto-detect: exactly one session → use it.
	sessions, err := session.List(".")
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	switch len(sessions) {
	case 0:
		// Check for legacy layout before creating default session.
		if dirExists("docs/tasks") {
			return "", fmt.Errorf("no sessions found\n\nTo create a session:\n  snap new <name>")
		}
		if err := session.EnsureDefault("."); err != nil {
			return "", err
		}
		return "default", nil
	case 1:
		return sessions[0].Name, nil
	default:
		return "", formatMultipleStatusSessionsError(sessions)
	}
}

// formatMultipleStatusSessionsError builds an error listing available sessions.
func formatMultipleStatusSessionsError(sessions []session.Info) error {
	var b strings.Builder
	b.WriteString("Error: multiple sessions found\n\nAvailable sessions:\n")
	for _, s := range sessions {
		fmt.Fprintf(&b, "  %-12s  %s\n", s.Name, formatTaskSummary(s.TaskCount, s.CompletedCount))
	}
	b.WriteString("\nSpecify a session:\n  snap status <name>")
	return fmt.Errorf("%s", b.String())
}
