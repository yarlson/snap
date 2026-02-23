package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/pathutil"
)

func TestFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedPRD string
	}{
		{
			name:        "default paths",
			args:        []string{},
			expectedPRD: "docs/tasks/PRD.md",
		},
		{
			name:        "custom tasks dir",
			args:        []string{"--tasks-dir", "features"},
			expectedPRD: "features/PRD.md",
		},
		{
			name:        "custom tasks dir short flag",
			args:        []string{"-d", "docs"},
			expectedPRD: "docs/PRD.md",
		},
		{
			name:        "custom prd path overrides tasks dir",
			args:        []string{"--tasks-dir", "features", "--prd", "custom/requirements.md"},
			expectedPRD: "custom/requirements.md",
		},
		{
			name:        "short flags",
			args:        []string{"-d", "features", "-p", "my-prd.md"},
			expectedPRD: "my-prd.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to defaults
			tasksDir = "docs/tasks"
			prdPath = ""

			// Create a fresh command for each test
			cmd := rootCmd
			cmd.SetArgs(tt.args)

			// Parse flags
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Apply defaults using pathutil (same as run() does)
			prdPath = pathutil.ResolvePRDPath(tasksDir, prdPath)

			// Verify resolved paths
			assert.Equal(t, tt.expectedPRD, prdPath)
		})
	}
}

func TestRootCommand_InvalidFlagDoesNotPrintUsage(t *testing.T) {
	// Use the shared root command but restore test-facing settings afterward.
	origArgs := rootCmd.Flags().Args()
	_ = origArgs // Cobra does not expose current raw args; keep local pattern explicit.

	var outBuf strings.Builder
	var errBuf strings.Builder
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--definitely-not-a-real-flag"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	output := errBuf.String()
	assert.NotContains(t, output, "Usage:", "usage/help should not be printed on errors")
	assert.Empty(t, output, "cobra should not print the error when Execute() handles it")

	// Reset to avoid leaking test state into other tests.
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
}

func TestRootCommand_SilencesUsageAndErrors(t *testing.T) {
	assert.True(t, rootCmd.SilenceUsage, "usage/help should be suppressed on errors")
	assert.True(t, rootCmd.SilenceErrors, "cobra should not print errors when Execute() handles them")
}
