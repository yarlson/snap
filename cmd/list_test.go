package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Integration tests ---

func TestList_EmptyOutput(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	var outBuf strings.Builder
	listCmd.SetOut(&outBuf)
	defer listCmd.SetOut(nil)

	err = listCmd.RunE(listCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "No sessions found")
	assert.Contains(t, output, "snap new <name>")
}

func TestList_FormattedOutput(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create sessions with tasks.
	sessionsDir := filepath.Join(projectDir, ".snap", "sessions")
	for _, name := range []string{"auth", "api"} {
		tasksDir := filepath.Join(sessionsDir, name, "tasks")
		require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	}
	// Add a task to auth.
	require.NoError(t, os.WriteFile(
		filepath.Join(sessionsDir, "auth", "tasks", "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	var outBuf strings.Builder
	listCmd.SetOut(&outBuf)
	defer listCmd.SetOut(nil)

	err = listCmd.RunE(listCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()
	// Should contain both sessions.
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "auth")
	// auth should show 1 task.
	assert.Contains(t, output, "1 task")
	// api should show 0 tasks.
	assert.Contains(t, output, "0 tasks")
}
