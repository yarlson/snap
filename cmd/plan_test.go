package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/session"
)

// --- Integration tests: resolvePlanSession ---

func TestResolvePlanSession_ZeroSessions_CreatesDefault(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	name, err := resolvePlanSession(nil)
	require.NoError(t, err)
	assert.Equal(t, "default", name)

	// "default" session directory should exist on disk.
	assert.True(t, session.Exists(".", "default"))
}

func TestResolvePlanSession_OneSession_AutoDetects(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create one session.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessDir, 0o755))

	name, err := resolvePlanSession(nil)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}

func TestResolvePlanSession_MultipleSessions_Errors(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create two sessions.
	for _, n := range []string{"auth", "api"} {
		sessDir := filepath.Join(projectDir, ".snap", "sessions", n, "tasks")
		require.NoError(t, os.MkdirAll(sessDir, 0o755))
	}

	_, err := resolvePlanSession(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple sessions found")
}
