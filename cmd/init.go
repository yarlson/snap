package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var prdTemplate = `# Product Requirements

## Summary

Describe the product or feature at a high level. What problem does it solve?

## Goals

1. Goal one
2. Goal two

## Non-goals

- What this project is NOT trying to do

## Requirements

### Must have

- Requirement one
- Requirement two

### Should have

- Nice-to-have requirement
`

var taskTemplate = `# Task 1: <title>

## Objective

Describe what this task accomplishes in one sentence.

## Requirements

- Requirement one
- Requirement two

## Acceptance Criteria

- [ ] Criterion one
- [ ] Criterion two
`

var initCmd = &cobra.Command{
	Use:           "init",
	Short:         "Scaffold a new snap project with template files",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          initRun,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initRun(cmd *cobra.Command, _ []string) error {
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", tasksDir, err)
	}

	templates := []struct {
		name    string
		content string
	}{
		{"PRD.md", prdTemplate},
		{"TASK1.md", taskTemplate},
	}

	var created int
	for _, tmpl := range templates {
		path := filepath.Join(tasksDir, tmpl.name)
		if _, err := os.Stat(path); err == nil {
			continue // file exists, skip
		}
		if err := os.WriteFile(path, []byte(tmpl.content), 0o600); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Created "+path)
		created++
	}

	if created == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Already initialized \u2014 nothing to create")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Edit "+filepath.Join(tasksDir, "PRD.md")+" with your product context")
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Edit "+filepath.Join(tasksDir, "TASK1.md")+" with your first task")
	fmt.Fprintln(cmd.OutOrStdout(), "  3. Run: snap")

	return nil
}
