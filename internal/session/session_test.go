package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests: ValidateName ---

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{name: "simple lowercase", input: "auth", wantErr: false},
		{name: "with hyphens", input: "api-refactor", wantErr: false},
		{name: "with underscores", input: "api_refactor", wantErr: false},
		{name: "mixed case", input: "MyFeature", wantErr: false},
		{name: "alphanumeric", input: "a-valid-name_123", wantErr: false},
		{name: "single char", input: "a", wantErr: false},
		{name: "max length 64", input: strings.Repeat("a", 64), wantErr: false},

		{name: "empty string", input: "", wantErr: true, errMsg: "session name required"},
		{name: "65 chars", input: strings.Repeat("a", 65), wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "spaces", input: "bad name", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "special chars", input: "bad name!", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "dots", input: "bad.name", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "slashes", input: "bad/name", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "path traversal", input: "..", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
		{name: "hidden dir", input: ".hidden", wantErr: true, errMsg: "use alphanumeric, hyphens, underscores"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- Integration tests: Create, Exists, path helpers ---

func TestCreate_Success(t *testing.T) {
	root := t.TempDir()

	err := Create(root, "auth")
	require.NoError(t, err)

	// tasks directory should exist
	tasksDir := filepath.Join(root, ".snap", "sessions", "auth", "tasks")
	info, err := os.Stat(tasksDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCreate_DuplicateReturnsError(t *testing.T) {
	root := t.TempDir()

	err := Create(root, "auth")
	require.NoError(t, err)

	err = Create(root, "auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreate_EmptyName(t *testing.T) {
	root := t.TempDir()

	err := Create(root, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session name required")
}

func TestCreate_InvalidName(t *testing.T) {
	root := t.TempDir()

	err := Create(root, "bad name!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use alphanumeric, hyphens, underscores")
}

func TestCreate_LongName(t *testing.T) {
	root := t.TempDir()

	err := Create(root, strings.Repeat("a", 65))
	require.Error(t, err)
}

func TestExists_AfterCreate(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Create(root, "auth"))
	assert.True(t, Exists(root, "auth"))
}

func TestExists_Nonexistent(t *testing.T) {
	root := t.TempDir()

	assert.False(t, Exists(root, "nonexistent"))
}

func TestDir(t *testing.T) {
	root := t.TempDir()
	got := Dir(root, "auth")
	assert.Equal(t, filepath.Join(root, ".snap", "sessions", "auth"), got)
}

func TestTasksDir(t *testing.T) {
	root := t.TempDir()
	got := TasksDir(root, "auth")
	assert.Equal(t, filepath.Join(root, ".snap", "sessions", "auth", "tasks"), got)
}

// --- Integration tests: List ---

func TestList_ZeroSessions(t *testing.T) {
	root := t.TempDir()

	sessions, err := List(root)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestList_ThreeSessions_SortedByName(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Create(root, "cleanup"))
	require.NoError(t, Create(root, "auth"))
	require.NoError(t, Create(root, "api"))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	assert.Equal(t, "api", sessions[0].Name)
	assert.Equal(t, "auth", sessions[1].Name)
	assert.Equal(t, "cleanup", sessions[2].Name)
}

func TestList_TaskCountsAndCompleted(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	// Add 2 task files.
	tasksDir := TasksDir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK2.md"), []byte("# Task 2\n"), 0o600))

	// Write state.json with 1 completed task.
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
	sessionDir := Dir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte(stateJSON), 0o600))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, "auth", sessions[0].Name)
	assert.Equal(t, 2, sessions[0].TaskCount)
	assert.Equal(t, 1, sessions[0].CompletedCount)
	assert.Equal(t, "paused at step 5", sessions[0].Status)
}

func TestList_CorruptStateJSON(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "broken"))

	// Write corrupt state.json.
	sessionDir := Dir(root, "broken")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte("not json"), 0o600))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, "broken", sessions[0].Name)
	assert.Equal(t, "unknown", sessions[0].Status)
}

func TestList_PlanningMarker(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "api"))

	// Write .plan-started marker.
	sessionDir := Dir(root, "api")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, ".plan-started"), []byte(""), 0o600))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, "api", sessions[0].Name)
	assert.Equal(t, 0, sessions[0].TaskCount)
	assert.Equal(t, "planning", sessions[0].Status)
}

func TestList_CompleteStatus(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "done"))

	tasksDir := TasksDir(root, "done")
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK2.md"), []byte("# Task 2\n"), 0o600))

	stateJSON := `{
		"tasks_dir": "tasks",
		"current_task_id": "",
		"current_step": 1,
		"total_steps": 10,
		"completed_task_ids": ["TASK1", "TASK2"],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "tasks/PRD.md"
	}`
	sessionDir := Dir(root, "done")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte(stateJSON), 0o600))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, 2, sessions[0].CompletedCount)
	assert.Equal(t, "complete", sessions[0].Status)
}

func TestList_NoTasksStatus(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "empty"))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, 0, sessions[0].TaskCount)
	assert.Equal(t, "no tasks", sessions[0].Status)
}

func TestList_IdleStatus(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "idle"))

	tasksDir := TasksDir(root, "idle")
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// State exists but no active task and no completed tasks.
	stateJSON := `{
		"tasks_dir": "tasks",
		"current_task_id": "",
		"current_step": 1,
		"total_steps": 10,
		"completed_task_ids": [],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "tasks/PRD.md"
	}`
	sessionDir := Dir(root, "idle")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte(stateJSON), 0o600))

	sessions, err := List(root)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, "idle", sessions[0].Status)
}

func TestList_NoSessionsDir(t *testing.T) {
	root := t.TempDir()

	// No .snap/sessions/ directory at all.
	sessions, err := List(root)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// --- Integration tests: Delete ---

func TestDelete_Success(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	err := Delete(root, "auth")
	require.NoError(t, err)
	assert.False(t, Exists(root, "auth"))
}

func TestDelete_Nonexistent(t *testing.T) {
	root := t.TempDir()

	err := Delete(root, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDelete_WithFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	// Add files to the session.
	tasksDir := TasksDir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	sessionDir := Dir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte("{}"), 0o600))

	err := Delete(root, "auth")
	require.NoError(t, err)
	assert.False(t, Exists(root, "auth"))

	// Entire directory tree should be gone.
	_, err = os.Stat(sessionDir)
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_PathTraversal(t *testing.T) {
	root := t.TempDir()

	// Attempt to delete with path traversal should be rejected.
	err := Delete(root, "../../..")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session name")
}

func TestDelete_PathTraversalDot(t *testing.T) {
	root := t.TempDir()

	// Attempt to delete with dot path should be rejected.
	err := Delete(root, "..")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session name")
}

// --- Unit tests: Resolve ---

func TestResolve_ExistingSession(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	dir, err := Resolve(root, "auth")
	require.NoError(t, err)
	assert.Equal(t, Dir(root, "auth"), dir)
}

func TestResolve_NonexistentSession(t *testing.T) {
	root := t.TempDir()

	_, err := Resolve(root, "auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "snap new auth")
}

func TestResolve_InvalidName(t *testing.T) {
	root := t.TempDir()

	_, err := Resolve(root, "bad name!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session name")
}

// --- Integration tests: HasPlanHistory / MarkPlanStarted ---

func TestHasPlanHistory_WithoutMarker(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	assert.False(t, HasPlanHistory(root, "auth"))
}

func TestHasPlanHistory_AfterMarkPlanStarted(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	err := MarkPlanStarted(root, "auth")
	require.NoError(t, err)

	assert.True(t, HasPlanHistory(root, "auth"))
}

func TestMarkPlanStarted_Idempotent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	require.NoError(t, MarkPlanStarted(root, "auth"))
	require.NoError(t, MarkPlanStarted(root, "auth"))

	assert.True(t, HasPlanHistory(root, "auth"))
}

// --- Unit tests: EnsureDefault ---

func TestEnsureDefault_CreatesWhenAbsent(t *testing.T) {
	root := t.TempDir()

	err := EnsureDefault(root)
	require.NoError(t, err)

	// "default" session should now exist.
	assert.True(t, Exists(root, "default"))

	// Tasks directory should exist.
	info, err := os.Stat(TasksDir(root, "default"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureDefault_SucceedsWhenAlreadyExists(t *testing.T) {
	root := t.TempDir()

	// Create "default" session first.
	require.NoError(t, Create(root, "default"))

	// EnsureDefault should succeed (idempotent).
	err := EnsureDefault(root)
	assert.NoError(t, err)
}

func TestEnsureDefault_PropagatesOtherErrors(t *testing.T) {
	// Use a read-only directory to trigger a filesystem error.
	root := t.TempDir()
	snapDir := filepath.Join(root, ".snap")
	require.NoError(t, os.MkdirAll(snapDir, 0o755))
	require.NoError(t, os.Chmod(snapDir, 0o444))
	t.Cleanup(func() {
		//nolint:errcheck // cleanup: restore permissions so t.TempDir can remove it
		os.Chmod(snapDir, 0o755)
	})

	err := EnsureDefault(root)
	require.Error(t, err)
	// Should NOT contain "already exists" — it's a real error.
	assert.NotContains(t, err.Error(), "already exists")
}

// --- Integration tests: Status ---

func TestStatus_WithTasksAndActiveStep(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	// Add 3 task files.
	td := TasksDir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK2.md"), []byte("# Task 2\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK3.md"), []byte("# Task 3\n"), 0o600))

	// Write state: TASK1 completed, TASK2 active at step 5.
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
	sessionDir := Dir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte(stateJSON), 0o600))

	st, err := Status(root, "auth")
	require.NoError(t, err)

	assert.Equal(t, "auth", st.Name)
	assert.Equal(t, td, st.TasksDir)
	require.Len(t, st.Tasks, 3)
	assert.Equal(t, "TASK1", st.Tasks[0].ID)
	assert.True(t, st.Tasks[0].Completed)
	assert.Equal(t, "TASK2", st.Tasks[1].ID)
	assert.False(t, st.Tasks[1].Completed)
	assert.Equal(t, "TASK3", st.Tasks[2].ID)
	assert.False(t, st.Tasks[2].Completed)
	assert.Equal(t, "TASK2", st.ActiveTask)
	assert.Equal(t, 5, st.ActiveStep)
	assert.Equal(t, 10, st.TotalSteps)
}

func TestStatus_NoTasks(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	st, err := Status(root, "auth")
	require.NoError(t, err)

	assert.Equal(t, "auth", st.Name)
	assert.Empty(t, st.Tasks)
	assert.Empty(t, st.ActiveTask)
	assert.Equal(t, 0, st.ActiveStep)
}

func TestStatus_NonexistentSession(t *testing.T) {
	root := t.TempDir()

	_, err := Status(root, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatus_MissingStateJSON(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Create(root, "auth"))

	// Add tasks but no state.json.
	td := TasksDir(root, "auth")
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK2.md"), []byte("# Task 2\n"), 0o600))

	st, err := Status(root, "auth")
	require.NoError(t, err)

	require.Len(t, st.Tasks, 2)
	assert.False(t, st.Tasks[0].Completed)
	assert.False(t, st.Tasks[1].Completed)
	assert.Empty(t, st.ActiveTask)
	assert.Equal(t, 0, st.ActiveStep)
}
