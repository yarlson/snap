package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/session"
)

var forceDelete bool

var deleteCmd = &cobra.Command{
	Use:           "delete <name>",
	Short:         "Delete a session and all its files",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          deleteRun,
}

func init() {
	deleteCmd.Flags().BoolVar(&forceDelete, "force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func deleteRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := cmd.OutOrStdout()

	if !forceDelete {
		fmt.Fprintf(out, "Delete session '%s' and all its files? (y/N) ", name)

		scanner := bufio.NewScanner(cmd.InOrStdin())
		if !scanner.Scan() {
			return nil
		}
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "y") {
			return nil
		}
	}

	if err := session.Delete(".", name); err != nil {
		return err
	}

	fmt.Fprintf(out, "Deleted session '%s'\n", name)
	return nil
}
