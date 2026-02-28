package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/input"
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

// --- Integration tests: checkPlanConflict ---

func TestCheckPlanConflict_EmptySession_NoPrompt(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))

	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
	assert.Empty(t, stdout.String())
}

func TestCheckPlanConflict_NonTTY_ReturnsError(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	var stdout bytes.Buffer
	_, err := checkPlanConflict("auth", false, strings.NewReader(""), &stdout)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already has planning artifacts")
	assert.Contains(t, err.Error(), "snap delete auth")
	assert.Contains(t, err.Error(), "snap new")
}

func TestCheckPlanConflict_TTY_Choice1_CleansUp(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	td := session.TasksDir(".", "auth")
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "PRD.md"), []byte("# PRD\n"), 0o600))

	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("1"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)

	// Session should be cleaned.
	assert.False(t, session.HasArtifacts(".", "auth"))
}

func TestCheckPlanConflict_TTY_CtrlC_ReturnsInterrupt(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Ctrl+C is byte 3.
	var stdout bytes.Buffer
	_, err := checkPlanConflict("auth", true, strings.NewReader(string(byte(3))), &stdout)
	require.Error(t, err)
	assert.True(t, errors.Is(err, input.ErrInterrupt))
}

func TestCheckPlanConflict_TTY_IgnoresInvalidInput(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "x" then "1" — "x" should be ignored.
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("x1"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}

func TestCheckPlanConflict_TTY_InvalidChoice_IsIgnored(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2" (no longer valid) then "1" — "2" should be ignored.
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("21"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}
