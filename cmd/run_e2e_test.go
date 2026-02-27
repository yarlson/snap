package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSnap builds the snap binary and returns its path.
func buildSnap(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)
	return binPath
}

// createMockProvider creates a mock claude script in a temp dir and returns the PATH.
func createMockProvider(t *testing.T, script string) string {
	t.Helper()
	mockBinDir := t.TempDir()
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte(script), 0o755)) //nolint:gosec // G306: test mock
	return mockBinDir + ":/usr/bin:/bin"
}

// CUJ-1: Create and Run Session — verify session-scoped startup summary and state.
func TestE2E_CUJ1_CreateAndRunSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Step 1: Create session.
	create := exec.CommandContext(ctx, binPath, "new", "auth")
	create.Dir = projectDir
	out, err := create.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", out)

	// Step 2: Copy task file into session tasks dir.
	sessTasksDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.WriteFile(
		filepath.Join(sessTasksDir, "TASK1.md"),
		[]byte("# Task 1\nImplement something small"), 0o600))

	// Step 3: Run with mock provider that blocks.
	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	run := exec.CommandContext(ctx, binPath, "run", "auth")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)

	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut

	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))

	//nolint:errcheck // expect non-zero exit from SIGINT
	_ = run.Wait()

	output := combinedOut.String()

	// Verify startup summary shows session name.
	assert.Contains(t, output, "snap: auth |",
		"startup summary should show session name 'auth'")

	// Verify state.json created in session dir (not legacy location).
	sessionStateFile := filepath.Join(projectDir, ".snap", "sessions", "auth", "state.json")
	_, err = os.Stat(sessionStateFile)
	assert.NoError(t, err, "state.json should exist in session dir")

	// Verify legacy state.json does NOT exist.
	legacyStateFile := filepath.Join(projectDir, ".snap", "state.json")
	_, err = os.Stat(legacyStateFile)
	assert.True(t, os.IsNotExist(err), "legacy state.json should not exist")
}

// CUJ-3: Switch Between Sessions — verify independent state.
func TestE2E_CUJ3_SwitchBetweenSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create two sessions with tasks.
	for _, name := range []string{"auth", "api"} {
		create := exec.CommandContext(ctx, binPath, "new", name)
		create.Dir = projectDir
		out, err := create.CombinedOutput()
		require.NoError(t, err, "snap new %s failed: %s", name, out)

		sessTasksDir := filepath.Join(projectDir, ".snap", "sessions", name, "tasks")
		require.NoError(t, os.WriteFile(
			filepath.Join(sessTasksDir, "TASK1.md"),
			[]byte("# Task 1\nDo something"), 0o600))
	}

	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	// Run first session, let it start, then SIGINT.
	run1 := exec.CommandContext(ctx, binPath, "run", "auth")
	run1.Dir = projectDir
	run1.Env = append(os.Environ(), "PATH="+mockPath)
	require.NoError(t, run1.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run1.Process.Signal(syscall.SIGINT))
	_ = run1.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	// Run second session — should start fresh (independent state).
	run2 := exec.CommandContext(ctx, binPath, "run", "api")
	run2.Dir = projectDir
	run2.Env = append(os.Environ(), "PATH="+mockPath)
	var out2 strings.Builder
	run2.Stdout = &out2
	run2.Stderr = &out2
	require.NoError(t, run2.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run2.Process.Signal(syscall.SIGINT))
	_ = run2.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	output2 := out2.String()
	assert.Contains(t, output2, "snap: api |",
		"second session should show 'api' in startup summary")

	// Both sessions should have independent state.json files.
	authState := filepath.Join(projectDir, ".snap", "sessions", "auth", "state.json")
	apiState := filepath.Join(projectDir, ".snap", "sessions", "api", "state.json")
	_, err := os.Stat(authState)
	assert.NoError(t, err, "auth state should exist")
	_, err = os.Stat(apiState)
	assert.NoError(t, err, "api state should exist")
}

// CUJ-4: Auto-detect Single Session.
func TestE2E_CUJ4_AutoDetectSingleSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create one session with tasks.
	create := exec.CommandContext(ctx, binPath, "new", "auth")
	create.Dir = projectDir
	out, err := create.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", out)

	sessTasksDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.WriteFile(
		filepath.Join(sessTasksDir, "TASK1.md"),
		[]byte("# Task 1\nDo something"), 0o600))

	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	// Run bare snap run (no session name) — should auto-detect.
	run := exec.CommandContext(ctx, binPath, "run")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut
	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))
	_ = run.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	output := combinedOut.String()
	assert.Contains(t, output, "snap: auth |",
		"bare snap run should auto-detect single session 'auth'")
}

// CUJ-5: Legacy Fallback — bare snap run with docs/tasks/ layout.
func TestE2E_CUJ5_LegacyFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Set up legacy layout (no sessions, just docs/tasks/).
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(tasksSubDir, "TASK1.md"),
		[]byte("# Task 1\nDo something"), 0o600))

	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	// Run bare snap run — should use legacy layout.
	run := exec.CommandContext(ctx, binPath, "run")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut
	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))
	_ = run.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	output := combinedOut.String()
	assert.Contains(t, output, "snap: docs/tasks",
		"legacy fallback should show 'docs/tasks' in startup summary")
}

// Test: snap run with multiple sessions and no name shows error.
func TestE2E_RunMultipleSessionsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create two sessions.
	for _, name := range []string{"auth", "api"} {
		create := exec.CommandContext(ctx, binPath, "new", name)
		create.Dir = projectDir
		out, err := create.CombinedOutput()
		require.NoError(t, err, "snap new %s failed: %s", name, out)
	}

	mockPath := createMockProvider(t, "#!/bin/sh\nexit 0\n")

	// Run bare snap run — should error with list.
	run := exec.CommandContext(ctx, binPath, "run")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "multiple sessions found")
	assert.Contains(t, outputStr, "auth")
	assert.Contains(t, outputStr, "api")
	assert.Contains(t, outputStr, "snap run <name>")
}

// Test: snap run with no sessions and no legacy shows error.
func TestE2E_RunNoSessionsNoLegacyError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	mockPath := createMockProvider(t, "#!/bin/sh\nexit 0\n")

	// Run bare snap run with no sessions and no legacy layout.
	run := exec.CommandContext(ctx, binPath, "run")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "no sessions found")
	assert.Contains(t, outputStr, "snap new <name>")
}

// Test: snap run <nonexistent> shows error with hint.
func TestE2E_RunNonexistentSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	mockPath := createMockProvider(t, "#!/bin/sh\nexit 0\n")

	run := exec.CommandContext(ctx, binPath, "run", "nonexistent")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "not found")
	assert.Contains(t, outputStr, "snap new nonexistent")
}

// Test: --show-state with session name reads session state.
func TestE2E_ShowStateWithSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create session and write state.json in it.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	tasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

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

	run := exec.CommandContext(ctx, binPath, "run", "auth", "--show-state")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err, "snap run auth --show-state failed: %s", output)

	outputStr := string(output)
	assert.Contains(t, outputStr, "TASK2 in progress")
	assert.Contains(t, outputStr, "step 5/10")
}

// Test: --show-state with session but no state file outputs "No state file exists".
func TestE2E_ShowStateNoStateFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create session without state.json.
	tasksDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	run := exec.CommandContext(ctx, binPath, "run", "auth", "--show-state")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err, "snap run auth --show-state failed: %s", output)

	outputStr := string(output)
	assert.Contains(t, outputStr, "No state file exists")
}

// Test: --fresh with session-scoped state resets the session state.
func TestE2E_FreshWithSessionState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create session with task file.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	sessTasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(sessTasksDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(sessTasksDir, "TASK1.md"),
		[]byte("# Task 1\nDo something"), 0o600))

	// Write pre-existing state at step 5.
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

	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	// Run with --fresh — should reset state, not resume from step 5.
	run := exec.CommandContext(ctx, binPath, "run", "auth", "--fresh")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut
	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))
	_ = run.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	output := combinedOut.String()

	// Should show fresh start info, not resume.
	assert.Contains(t, output, "Fresh start requested",
		"output should indicate fresh start")
	assert.NotContains(t, output, "resuming TASK1 from step 5",
		"output should not show resume from old step")
	assert.Contains(t, output, "starting TASK1",
		"output should show starting task fresh")
}

// Test: resume works across session runs (state persisted correctly).
func TestE2E_ResumeAcrossSessionRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create session with task file.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth")
	sessTasksDir := filepath.Join(sessDir, "tasks")
	require.NoError(t, os.MkdirAll(sessTasksDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(sessTasksDir, "TASK1.md"),
		[]byte("# Task 1\nDo something"), 0o600))

	// Write pre-existing state at step 3 (simulating previous interrupted run).
	stateJSON := `{
		"tasks_dir": ".snap/sessions/auth/tasks",
		"current_task_id": "TASK1",
		"current_task_file": "TASK1.md",
		"current_step": 3,
		"total_steps": 10,
		"completed_task_ids": [],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": ".snap/sessions/auth/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "state.json"), []byte(stateJSON), 0o600))

	mockPath := createMockProvider(t, "#!/bin/sh\nexec /bin/sleep 3600\n")

	// Run session again — should resume from step 3.
	run := exec.CommandContext(ctx, binPath, "run", "auth")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockPath)
	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut
	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))
	_ = run.Wait() //nolint:errcheck // expect non-zero exit from SIGINT

	output := combinedOut.String()

	// Verify startup summary shows resume action.
	assert.Contains(t, output, "resuming TASK1 from step 3",
		"output should show resuming from saved step")
	assert.Contains(t, output, "snap: auth |",
		"startup summary should show session name")
}
