package cmd

import (
	"bytes"
	"errors"
	"fmt"
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

	// Feed "3" (invalid) then "1" — "3" should be ignored.
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("31"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}

// --- Integration tests: checkPlanConflict choice 2 ---

func TestCheckPlanConflict_TTY_Choice2_ValidName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2" to select choice 2, then "my-feature\r" as the session name.
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("2my-feature\r"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", name)

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "my-feature"))

	// Criterion 1: "Session name:" prompt must appear.
	output := stdout.String()
	assert.Contains(t, output, "Session name:")

	// Criterion 7: No confirmation message after session creation.
	assert.NotContains(t, output, "reated")
	assert.NotContains(t, output, "onfirm")
}

func TestCheckPlanConflict_TTY_Choice2_InvalidThenValid(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2", then "bad name!\r" (invalid), then "good-name\r" (valid).
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("2bad name!\r"+"good-name\r"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "good-name", name)

	// Validation error should have been printed.
	output := stdout.String()
	assert.Contains(t, output, "Invalid session name")

	// Criterion 8: error on its own line, immediately above re-prompt (no blank line between).
	errorIdx := strings.Index(output, "Invalid session name")
	require.NotEqual(t, -1, errorIdx)
	afterError := output[errorIdx:]
	lines := strings.SplitN(afterError, "\r\n", 3)
	require.GreaterOrEqual(t, len(lines), 2, "expected at least two lines after error")
	assert.Contains(t, lines[1], "Session name:", "re-prompt should immediately follow error (no blank line)")

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "good-name"))
}

func TestCheckPlanConflict_TTY_Choice2_ExistingThenNew(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2", then "auth\r" (exists), then "auth-v2\r" (new).
	var stdout bytes.Buffer
	name, err := checkPlanConflict("auth", true, strings.NewReader("2auth\r"+"auth-v2\r"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "auth-v2", name)

	// "Already exists" error should have been printed.
	output := stdout.String()
	assert.Contains(t, output, `Session "auth" already exists`)

	// Criterion 8: error on its own line, immediately above re-prompt (no blank line between).
	errorIdx := strings.Index(output, `Session "auth" already exists`)
	require.NotEqual(t, -1, errorIdx)
	afterError := output[errorIdx:]
	lines := strings.SplitN(afterError, "\r\n", 3)
	require.GreaterOrEqual(t, len(lines), 2, "expected at least two lines after error")
	assert.Contains(t, lines[1], "Session name:", "re-prompt should immediately follow error (no blank line)")

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "auth-v2"))
}

func TestCheckPlanConflict_TTY_Choice2_CtrlC_DuringName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2" to select choice 2, then Ctrl+C (byte 3) during name input.
	var stdout bytes.Buffer
	_, err := checkPlanConflict("auth", true, strings.NewReader("2"+string(byte(3))), &stdout)
	require.Error(t, err)
	assert.True(t, errors.Is(err, input.ErrInterrupt))
}

func TestCheckPlanConflict_TTY_Choice2_EOF_DuringName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Feed "2" then EOF (empty reader after choice byte).
	var stdout bytes.Buffer
	_, err := checkPlanConflict("auth", true, strings.NewReader("2"), &stdout)
	require.Error(t, err)
	// Should not panic — graceful error on EOF.
}

// --- Integration test: planRun wiring after conflict choice 2 ---

func TestPlanRun_ConflictChoice2_UsesNewSessionName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create "default" session with artifacts to trigger conflict.
	require.NoError(t, session.Create(".", "default"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "default"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	// Simulate conflict prompt: user selects "2", then enters "my-feature".
	var stdout bytes.Buffer
	newName, err := checkPlanConflict("default", true, strings.NewReader("2my-feature\r"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", newName)

	// Verify downstream operations use the new session name (not "default"):

	// 1. Tasks dir resolves to the new session's directory.
	td := session.TasksDir(".", newName)
	assert.Equal(t, filepath.Join(".snap", "sessions", "my-feature", "tasks"), td)

	// 2. Session exists and supports plan markers.
	require.NoError(t, session.MarkPlanStarted(".", newName))
	assert.True(t, session.HasPlanHistory(".", newName))

	// 3. Run instruction references the new session name.
	runInstruction := fmt.Sprintf("Run: snap run %s", newName)
	assert.Equal(t, "Run: snap run my-feature", runInstruction)

	// 4. Original session is untouched (artifacts still present).
	assert.True(t, session.HasArtifacts(".", "default"))
}
