package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/session"
)

var listCmd = &cobra.Command{
	Use:           "list",
	Short:         "List all sessions",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          listRun,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func listRun(cmd *cobra.Command, _ []string) error {
	sessions, err := session.List(".")
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if len(sessions) == 0 {
		fmt.Fprintln(out, "No sessions found")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "To create a session:")
		fmt.Fprintln(out, "  snap new <name>")
		return nil
	}

	// Calculate column widths for alignment.
	maxName := 0
	maxTasks := 0
	taskSummaries := make([]string, len(sessions))

	for i, s := range sessions {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
		ts := formatTaskSummary(s.TaskCount, s.CompletedCount)
		taskSummaries[i] = ts
		if len(ts) > maxTasks {
			maxTasks = len(ts)
		}
	}

	for i, s := range sessions {
		fmt.Fprintf(out, "  %-*s  %-*s  %s\n",
			maxName, s.Name,
			maxTasks, taskSummaries[i],
			s.Status)
	}

	return nil
}

func formatTaskSummary(taskCount, completedCount int) string {
	if taskCount == 0 {
		return "0 tasks"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d task", taskCount)
	if taskCount != 1 {
		b.WriteString("s")
	}
	if completedCount > 0 {
		fmt.Fprintf(&b, " (%d done)", completedCount)
	}
	return b.String()
}
