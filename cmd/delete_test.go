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

func TestDelete_WithForceFlag(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	// Reset flags to avoid test pollution.
	require.NoError(t, deleteCmd.Flags().Set("force", "true"))
	defer func() { require.NoError(t, deleteCmd.Flags().Set("force", "false")) }()

	var outBuf strings.Builder
	deleteCmd.SetOut(&outBuf)
	defer deleteCmd.SetOut(nil)

	err = deleteCmd.RunE(deleteCmd, []string{"auth"})
	require.NoError(t, err)

	assert.Contains(t, outBuf.String(), "Deleted session 'auth'")

	// Verify directory is gone.
	_, err = os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_WithConfirmationYes(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	// Simulate stdin with "y\n".
	deleteCmd.SetIn(strings.NewReader("y\n"))
	defer deleteCmd.SetIn(nil)

	var outBuf strings.Builder
	deleteCmd.SetOut(&outBuf)
	defer deleteCmd.SetOut(nil)

	err = deleteCmd.RunE(deleteCmd, []string{"auth"})
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Delete session 'auth' and all its files? (y/N)")
	assert.Contains(t, output, "Deleted session 'auth'")
}

func TestDelete_WithConfirmationNo(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	// Simulate stdin with "n\n".
	deleteCmd.SetIn(strings.NewReader("n\n"))
	defer deleteCmd.SetIn(nil)

	var outBuf strings.Builder
	deleteCmd.SetOut(&outBuf)
	defer deleteCmd.SetOut(nil)

	err = deleteCmd.RunE(deleteCmd, []string{"auth"})
	require.NoError(t, err)

	// Session should still exist.
	_, err = os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	require.NoError(t, err)
}

func TestDelete_NonexistentSession(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	require.NoError(t, deleteCmd.Flags().Set("force", "true"))
	defer func() { require.NoError(t, deleteCmd.Flags().Set("force", "false")) }()

	err = deleteCmd.RunE(deleteCmd, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
