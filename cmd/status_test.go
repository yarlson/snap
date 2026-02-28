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

func TestStatus_WithTasksAndActiveStep(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create session with 3 tasks and state.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	tasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK2.md"), []byte("# Task 2\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK3.md"), []byte("# Task 3\n"), 0o600))

	stateJSON := `{
		"tasks_dir": "tasks",
		"current_task_id": "TASK2",
		"current_task_file": "TASK2.md",
		"current_step": 5,
		"total_steps": 10,
		"completed_task_ids": ["TASK1"],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "state.json"), []byte(stateJSON), 0o600))

	var outBuf strings.Builder
	statusCmd.SetOut(&outBuf)
	defer statusCmd.SetOut(nil)

	err = statusCmd.RunE(statusCmd, []string{"auth"})
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Session: auth")
	assert.Contains(t, output, "[x] TASK1")
	assert.Contains(t, output, "[~] TASK2")
	assert.Contains(t, output, "step 5/10")
	assert.Contains(t, output, "[ ] TASK3")
	assert.Contains(t, output, "2 tasks remaining")
	assert.Contains(t, output, "1 complete")
}

func TestStatus_NoTasks(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create session with no tasks.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	tasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	var outBuf strings.Builder
	statusCmd.SetOut(&outBuf)
	defer statusCmd.SetOut(nil)

	err = statusCmd.RunE(statusCmd, []string{"auth"})
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "No task files found")
	assert.Contains(t, output, "snap plan auth")
}

func TestStatus_NonexistentSession(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	err = statusCmd.RunE(statusCmd, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatus_AutoDetectSingleSession(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Create one session.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	tasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	var outBuf strings.Builder
	statusCmd.SetOut(&outBuf)
	defer statusCmd.SetOut(nil)

	// No args — should auto-detect.
	err = statusCmd.RunE(statusCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Session: auth")
	assert.Contains(t, output, "TASK1")
}

func TestResolveStatusSession_ZeroSessions_CreatesDefault(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// No sessions exist — should auto-create "default".
	name, err := resolveStatusSession(nil)
	require.NoError(t, err)
	assert.Equal(t, "default", name)

	// The "default" session directory should exist.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	info, err := os.Stat(defaultDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestResolveStatusSession_ZeroSessions_LegacyTaskFiles(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	// Set up legacy layout with task files but no sessions.
	legacyDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// Should return error, not auto-create "default".
	_, err = resolveStatusSession(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sessions found")

	// No "default" session should have been created.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	_, err = os.Stat(defaultDir)
	assert.True(t, os.IsNotExist(err), "default session should not be created when legacy task directory exists")
}
