package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/session"
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

	fmt.Fprintf(out, "Session: %s\n", st.Name)
	fmt.Fprintf(out, "Path:    %s\n", st.TasksDir)

	if len(st.Tasks) == 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "No task files found. Run: snap plan %s\n", st.Name)
		return nil
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Tasks:")

	completedCount := 0
	for _, task := range st.Tasks {
		marker := "[ ]"
		suffix := ""
		if task.Completed {
			marker = "[x]"
			completedCount++
		} else if task.ID == st.ActiveTask && st.ActiveStep > 0 {
			marker = "[~]"
			suffix = fmt.Sprintf(" (step %d/%d: %s)", st.ActiveStep, st.TotalSteps, workflow.StepName(st.ActiveStep))
		}
		fmt.Fprintf(out, "  %s %s%s\n", marker, task.ID, suffix)
	}

	remaining := len(st.Tasks) - completedCount
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%d tasks remaining, %d complete\n", remaining, completedCount)

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
		return "", fmt.Errorf("no sessions found\n\nTo create a session:\n  snap new <name>")
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
