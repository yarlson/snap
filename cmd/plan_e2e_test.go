package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlanProvider creates a mock claude script that accepts any args and exits 0.
// It outputs a minimal stream-json line so the parser has something to process.
func mockPlanProvider(t *testing.T) string {
	t.Helper()
	mockBinDir := t.TempDir()
	// The mock outputs a minimal assistant message in stream-json format, then exits.
	script := `#!/bin/sh
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"OK"}]}}'
exit 0
`
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte(script), 0o755)) //nolint:gosec // G306: test mock
	return mockBinDir + ":/usr/bin:/bin"
}

// CUJ-1: Fresh-start planning — snap plan on a project with no sessions.
func TestE2E_PlanFreshProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	mockPath := mockPlanProvider(t)

	// Run snap plan on a fresh project with no sessions — should auto-create "default".
	plan := exec.CommandContext(ctx, binPath, "plan")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)
	plan.Stdin = strings.NewReader("/done\n")

	output, planErr := plan.CombinedOutput()
	require.NoError(t, planErr, "snap plan (fresh project) failed: %s", output)

	outputStr := string(output)

	// Auto-creation should be silent — no "created" message in output.
	assert.NotContains(t, outputStr, "created")

	// Planning should proceed with the "default" session.
	assert.Contains(t, outputStr, "Planning session 'default'")
	assert.Contains(t, outputStr, "Planning complete")

	// The "default" session directory should exist on disk.
	defaultSessionDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	info, err := os.Stat(defaultSessionDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// The tasks directory should exist.
	tasksDir := filepath.Join(defaultSessionDir, "tasks")
	info, err = os.Stat(tasksDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// CUJ-2: Plan and Implement — plan portion (interactive).
func TestE2E_CUJ2_PlanInteractive(t *testing.T) {
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

	// Step 2: Run plan with piped input.
	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "auth")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)
	plan.Stdin = strings.NewReader("Add auth feature\n/done\n")

	output, planErr := plan.CombinedOutput()
	require.NoError(t, planErr, "snap plan failed: %s", output)

	outputStr := string(output)

	// Assert step headers present.
	assert.Contains(t, outputStr, "Step 1/4")
	assert.Contains(t, outputStr, "Step 2/4")
	assert.Contains(t, outputStr, "Step 3/4")
	assert.Contains(t, outputStr, "Step 4/4")

	// Assert planning complete message.
	assert.Contains(t, outputStr, "Planning complete")

	// Assert .plan-started marker was written.
	markerPath := filepath.Join(projectDir, ".snap", "sessions", "auth", ".plan-started")
	_, err = os.Stat(markerPath)
	assert.NoError(t, err, ".plan-started marker should exist")
}

// CUJ-2: Plan and Implement — with file (--from).
func TestE2E_CUJ2_PlanWithFile(t *testing.T) {
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

	// Step 2: Create brief.md.
	briefPath := filepath.Join(projectDir, "brief.md")
	require.NoError(t, os.WriteFile(briefPath, []byte("I want OAuth2 authentication"), 0o600))

	// Step 3: Run plan with --from (no stdin needed).
	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "auth", "--from", "brief.md")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)

	output, planErr := plan.CombinedOutput()
	require.NoError(t, planErr, "snap plan --from failed: %s", output)

	outputStr := string(output)

	// Assert --from header.
	assert.Contains(t, outputStr, "using brief.md as input")

	// Assert step headers.
	assert.Contains(t, outputStr, "Step 1/4")
	assert.Contains(t, outputStr, "Step 2/4")
	assert.Contains(t, outputStr, "Step 3/4")
	assert.Contains(t, outputStr, "Step 4/4")

	// Assert planning complete.
	assert.Contains(t, outputStr, "Planning complete")
}

// Test: snap plan with nonexistent session.
func TestE2E_PlanNonexistentSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "nonexistent")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)

	output, planErr := plan.CombinedOutput()
	require.Error(t, planErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "not found")
	assert.Contains(t, outputStr, "snap new nonexistent")
}

// Test: snap plan --from with nonexistent file.
func TestE2E_PlanFromNonexistentFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create session.
	create := exec.CommandContext(ctx, binPath, "new", "auth")
	create.Dir = projectDir
	out, err := create.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", out)

	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "auth", "--from", "nonexistent.md")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)

	output, planErr := plan.CombinedOutput()
	require.Error(t, planErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "failed to read input file")
}

// Test: snap plan auto-detects single session.
func TestE2E_PlanAutoDetectSingleSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Create one session.
	create := exec.CommandContext(ctx, binPath, "new", "auth")
	create.Dir = projectDir
	out, err := create.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", out)

	mockPath := mockPlanProvider(t)

	// Run plan without session name — should auto-detect.
	plan := exec.CommandContext(ctx, binPath, "plan")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)
	plan.Stdin = strings.NewReader("/done\n")

	output, planErr := plan.CombinedOutput()
	require.NoError(t, planErr, "snap plan (auto-detect) failed: %s", output)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Planning session 'auth'")
	assert.Contains(t, outputStr, "Planning complete")
}

// Test: file listing printed after plan completion.
// This test uses an empty session (no artifacts) so the conflict guard is not triggered.
func TestE2E_PlanFileListing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Step 1: Create session (empty — no conflict guard trigger).
	create := exec.CommandContext(ctx, binPath, "new", "auth")
	create.Dir = projectDir
	out, err := create.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", out)

	// Step 2: Run plan (mock provider outputs nothing, so no files generated).
	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "auth")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)
	plan.Stdin = strings.NewReader("/done\n")

	output, planErr := plan.CombinedOutput()
	require.NoError(t, planErr, "snap plan failed: %s", output)

	outputStr := string(output)

	// Planning should proceed without conflict prompt.
	assert.Contains(t, outputStr, "Planning session 'auth'")
	assert.Contains(t, outputStr, "Planning complete")

	// Assert run instruction is printed after plan completion.
	assert.Contains(t, outputStr, "Run: snap run auth")
}

// Test: snap plan on non-empty session with piped input returns conflict error.
func TestE2E_PlanConflictNonTTY(t *testing.T) {
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

	// Step 2: Place a task file to trigger conflict guard.
	tasksDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// Step 3: Run plan with piped input (non-TTY).
	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan", "auth")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)
	plan.Stdin = strings.NewReader("/done\n")

	output, planErr := plan.CombinedOutput()
	require.Error(t, planErr, "snap plan should fail with conflict error")

	outputStr := string(output)
	assert.Contains(t, outputStr, "already has planning artifacts")
	assert.Contains(t, outputStr, "snap delete auth")
	assert.Contains(t, outputStr, "snap new")
	assert.Contains(t, outputStr, "snap plan")
}

// Test: snap plan with multiple sessions and no name shows error.
func TestE2E_PlanMultipleSessionsError(t *testing.T) {
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

	mockPath := mockPlanProvider(t)

	plan := exec.CommandContext(ctx, binPath, "plan")
	plan.Dir = projectDir
	plan.Env = append(os.Environ(), "PATH="+mockPath)

	output, planErr := plan.CombinedOutput()
	require.Error(t, planErr)

	outputStr := string(output)
	assert.Contains(t, outputStr, "multiple sessions found")
	assert.Contains(t, outputStr, "auth")
	assert.Contains(t, outputStr, "api")
	assert.Contains(t, outputStr, "snap plan <name>")
}
