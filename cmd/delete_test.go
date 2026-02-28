package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/tap"
)

// --- Integration tests ---

func TestDelete_WithForceFlag(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	// Reset flags to avoid test pollution.
	require.NoError(t, deleteCmd.Flags().Set("force", "true"))
	defer func() { require.NoError(t, deleteCmd.Flags().Set("force", "false")) }()

	var outBuf strings.Builder
	deleteCmd.SetOut(&outBuf)
	defer deleteCmd.SetOut(nil)

	err := deleteCmd.RunE(deleteCmd, []string{"auth"})
	require.NoError(t, err)

	assert.Contains(t, outBuf.String(), "Deleted session 'auth'")

	// Verify directory is gone.
	_, err = os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_WithConfirmationYes(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	var outBuf strings.Builder
	deleteCmd.SetOut(&outBuf)
	defer deleteCmd.SetOut(nil)

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- deleteCmd.RunE(deleteCmd, []string{"auth"})
	}()

	// Confirm: press "y" to accept.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("y", tap.Key{Name: "y"})

	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	assert.Contains(t, outBuf.String(), "Deleted session 'auth'")

	// Verify directory is gone.
	_, err := os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_WithConfirmationNo(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- deleteCmd.RunE(deleteCmd, []string{"auth"})
	}()

	// Decline: press "n" to reject.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("n", tap.Key{Name: "n"})

	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// Session should still exist.
	_, err := os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	require.NoError(t, err)
}

func TestDelete_WithConfirmationCtrlC(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create session.
	sessionDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- deleteCmd.RunE(deleteCmd, []string{"auth"})
	}()

	// Cancel: press Ctrl+C.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("\x03", tap.Key{Name: "c", Ctrl: true})

	select {
	case err := <-resultCh:
		require.NoError(t, err) // Cancellation returns nil (same as declining).
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// Session should still exist.
	_, err := os.Stat(filepath.Join(projectDir, ".snap", "sessions", "auth"))
	require.NoError(t, err)
}

func TestDelete_NonexistentSession(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	require.NoError(t, deleteCmd.Flags().Set("force", "true"))
	defer func() { require.NoError(t, deleteCmd.Flags().Set("force", "false")) }()

	err := deleteCmd.RunE(deleteCmd, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
