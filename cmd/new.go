package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarlson/snap/internal/session"
	"github.com/yarlson/snap/internal/ui"
)

var newCmd = &cobra.Command{
	Use:           "new <name>",
	Short:         "Create a new named session",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          newRun,
}

func init() {
	rootCmd.AddCommand(newCmd)
}

func newRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := session.Create(".", name); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Created session '"+name+"'")
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), ui.Info("Next steps:"))
	fmt.Fprint(cmd.OutOrStdout(), ui.Info("  1. Plan your tasks: snap plan "+name))
	fmt.Fprint(cmd.OutOrStdout(), ui.Info("  2. Or add task files manually to .snap/sessions/"+name+"/tasks/"))
	fmt.Fprint(cmd.OutOrStdout(), ui.Info("  3. Run tasks: snap run "+name))

	return nil
}
