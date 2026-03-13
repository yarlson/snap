package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags:
//
//	go build -ldflags "-X github.com/yarlson/snap/cmd.Version=v0.1.0"
var Version = "dev"

var (
	tasksDir   string
	prdPath    string
	taskFile   string
	freshStart bool
	showState  bool
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:           "snap [session]",
	Short:         "Autonomous task-by-task implementation tool",
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `snap automates the task-by-task implementation workflow:
- Reads the next task from session/task files, or a single task file
- Implements the task
- Validates with linters and tests
- Reviews code changes
- Commits changes
- Updates project context

Runs continuously until interrupted with Ctrl+C.`,
	RunE: run,
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("snap {{.Version}}\n")

	rootCmd.PersistentFlags().StringVarP(&tasksDir, "tasks-dir", "d", "docs/tasks", "Directory containing PRD and task files")
	rootCmd.Flags().StringVar(&taskFile, "task-file", "", "Path to a single task file to run")
	rootCmd.Flags().StringVarP(&prdPath, "prd", "p", "", "Path to PRD file (default: <tasks-dir>/PRD.md)")
	rootCmd.Flags().BoolVar(&freshStart, "fresh", false, "Force fresh start, ignore existing state")
	rootCmd.Flags().BoolVar(&showState, "show-state", false, "Show current state and exit")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON (only with --show-state)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Map context.Canceled (from SIGINT/SIGTERM) to exit code 130
		// (standard SIGINT convention: 128 + 2). The signal handler in
		// Runner.Run() already printed the interruption message.
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
