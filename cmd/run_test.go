package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Integration tests: resolveRunConfig ---

func TestResolveRunConfig_NamedSession_Exists(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create a session with a task file.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	rc, err := resolveRunConfig("auth", "docs/tasks", "")
	require.NoError(t, err)

	// Paths are relative to cwd (project root is ".").
	assert.Equal(t, filepath.Join(".snap", "sessions", "auth", "tasks"), rc.tasksDir)
	assert.Equal(t, filepath.Join(".snap", "sessions", "auth", "tasks", "PRD.md"), rc.prdPath)
	assert.Equal(t, "auth", rc.displayName)
	assert.NotNil(t, rc.stateManager)
}

func TestResolveRunConfig_NamedSession_NotFound(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	_, err := resolveRunConfig("nonexistent", "docs/tasks", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "snap new nonexistent")
}

func TestResolveRunConfig_NoName_ZeroSessions_NoLegacy(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	rc, err := resolveRunConfig("", "docs/tasks", "")
	require.NoError(t, err)

	// Should auto-create "default" session and return its config.
	assert.Equal(t, filepath.Join(".snap", "sessions", "default", "tasks"), rc.tasksDir)
	assert.Equal(t, filepath.Join(".snap", "sessions", "default", "tasks", "PRD.md"), rc.prdPath)
	assert.Equal(t, "default", rc.displayName)
	assert.NotNil(t, rc.stateManager)
	assert.False(t, rc.userSupplied)
}

func TestResolveRunConfig_NoName_ZeroSessions_LegacyTaskFiles(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Set up legacy layout with task files.
	legacyDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	rc, err := resolveRunConfig("", "docs/tasks", "")
	require.NoError(t, err)

	assert.Equal(t, "docs/tasks", rc.tasksDir)
	assert.Equal(t, "docs/tasks/PRD.md", rc.prdPath)
	assert.Equal(t, "docs/tasks", rc.displayName)

	// No "default" session should have been created.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	_, err = os.Stat(defaultDir)
	assert.True(t, os.IsNotExist(err), "default session should not be created when legacy layout exists")
}

func TestResolveRunConfig_NoName_ZeroSessions_LegacyStateFile(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Set up legacy state file without task directory.
	snapDir := filepath.Join(projectDir, ".snap")
	require.NoError(t, os.MkdirAll(snapDir, 0o755))
	stateJSON := `{
		"tasks_dir": "docs/tasks",
		"current_task_id": "TASK1",
		"current_step": 3,
		"total_steps": 10,
		"completed_task_ids": [],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "docs/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(snapDir, "state.json"), []byte(stateJSON), 0o600))

	rc, err := resolveRunConfig("", "docs/tasks", "")
	require.NoError(t, err)

	assert.Equal(t, "docs/tasks", rc.tasksDir)
	assert.Equal(t, "docs/tasks", rc.displayName)

	// No "default" session should have been created.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	_, err = os.Stat(defaultDir)
	assert.True(t, os.IsNotExist(err), "default session should not be created when legacy state exists")
}

func TestResolveRunConfig_NoName_OneSession(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create exactly one session.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessDir, 0o755))

	rc, err := resolveRunConfig("", "docs/tasks", "")
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(".snap", "sessions", "auth", "tasks"), rc.tasksDir)
	assert.Equal(t, "auth", rc.displayName)
}

func TestResolveRunConfig_NoName_MultipleSessions(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create two sessions.
	for _, name := range []string{"auth", "api"} {
		sessDir := filepath.Join(projectDir, ".snap", "sessions", name, "tasks")
		require.NoError(t, os.MkdirAll(sessDir, 0o755))
	}

	_, err := resolveRunConfig("", "docs/tasks", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple sessions found")
	assert.Contains(t, err.Error(), "auth")
	assert.Contains(t, err.Error(), "api")
	assert.Contains(t, err.Error(), "snap run <name>")
}

func TestResolveRunConfig_SessionStateManager_IndependentFromLegacy(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create a session.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessDir, 0o755))

	rc, err := resolveRunConfig("auth", "docs/tasks", "")
	require.NoError(t, err)

	// Session state manager should NOT have state (fresh session).
	assert.False(t, rc.stateManager.Exists())

	// Write state to legacy location — should NOT be visible to session manager.
	snapDir := filepath.Join(projectDir, ".snap")
	stateJSON := `{
		"tasks_dir": "docs/tasks",
		"current_task_id": "TASK1",
		"current_step": 3,
		"total_steps": 10,
		"completed_task_ids": [],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "docs/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(snapDir, "state.json"), []byte(stateJSON), 0o600))

	// Session state manager should still have no state.
	assert.False(t, rc.stateManager.Exists())
}

func TestResolveRunConfig_SessionWhilePlanning(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create a session with planning marker and partial task files.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	td := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(td, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, ".plan-started"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	rc, err := resolveRunConfig("auth", "docs/tasks", "")
	require.NoError(t, err)

	// Should resolve successfully — reads whatever task files exist.
	assert.Equal(t, filepath.Join(".snap", "sessions", "auth", "tasks"), rc.tasksDir)
	assert.Equal(t, "auth", rc.displayName)
}

func TestResolveRunConfig_FreshWithSession(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create a session with pre-existing state.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	td := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(td, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// Write state.json to session dir (simulating prior run).
	stateJSON := `{
		"tasks_dir": ".snap/sessions/auth/tasks",
		"current_task_id": "TASK1",
		"current_task_file": "TASK1.md",
		"current_step": 5,
		"total_steps": 10,
		"completed_task_ids": [],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": ".snap/sessions/auth/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "state.json"), []byte(stateJSON), 0o600))

	rc, err := resolveRunConfig("auth", "docs/tasks", "")
	require.NoError(t, err)

	// Session state manager should be scoped to session dir and see the state.
	assert.Equal(t, "auth", rc.displayName)
	assert.True(t, rc.stateManager.Exists(), "session state manager should see existing state")

	// Verify --fresh resets state through the session-scoped manager.
	require.NoError(t, rc.stateManager.Reset())
	assert.False(t, rc.stateManager.Exists(), "state should be gone after reset")

	// Legacy state should remain unaffected.
	legacyStatePath := filepath.Join(projectDir, ".snap", "state.json")
	_, err = os.Stat(legacyStatePath)
	assert.True(t, os.IsNotExist(err), "legacy state should not exist")
}

func TestResolveRunConfig_ShowStateWithSession(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create a session with state.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	td := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(td, 0o755))

	stateJSON := `{
		"tasks_dir": ".snap/sessions/auth/tasks",
		"current_task_id": "TASK2",
		"current_task_file": "TASK2.md",
		"current_step": 5,
		"total_steps": 10,
		"completed_task_ids": ["TASK1"],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": ".snap/sessions/auth/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "state.json"), []byte(stateJSON), 0o600))

	// resolveStateManager should return session-scoped manager.
	sm, err := resolveStateManager("auth")
	require.NoError(t, err)
	assert.True(t, sm.Exists(), "session state manager should find state")

	// Load and verify it reads session state.
	s, err := sm.Load()
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "TASK2", s.CurrentTaskID)
	assert.Equal(t, 5, s.CurrentStep)
}

func TestResolveStateManager_ZeroSessions_CreatesDefault(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// No sessions exist — resolveStateManager with empty name should auto-create "default".
	sm, err := resolveStateManager("")
	require.NoError(t, err)
	assert.NotNil(t, sm)

	// The "default" session directory should exist.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	info, err := os.Stat(defaultDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestResolveStateManager_ZeroSessions_LegacyTaskFiles(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Set up legacy layout with task files but no state file.
	legacyDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// Should return legacy manager, not create default session.
	sm, err := resolveStateManager("")
	require.NoError(t, err)
	assert.NotNil(t, sm)

	// No "default" session should have been created.
	defaultDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	_, err = os.Stat(defaultDir)
	assert.True(t, os.IsNotExist(err), "default session should not be created when legacy task directory exists")
}

// chdir changes to the given directory and restores on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(origDir)) })
}
