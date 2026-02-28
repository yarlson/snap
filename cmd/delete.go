package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yarlson/tap"

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

	if !forceDelete {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		confirmed := tap.Confirm(ctx, tap.ConfirmOptions{
			Message:  fmt.Sprintf("Delete session '%s' and all its files?", name),
			Active:   "Yes",
			Inactive: "No",
		})
		if !confirmed {
			return nil
		}
	}

	if err := session.Delete(".", name); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted session '%s'\n", name)
	return nil
}
