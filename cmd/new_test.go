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

// --- E2E tests ---

func TestNewE2E_CreatesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	projectDir := t.TempDir()
	run := exec.CommandContext(ctx, binPath, "new", "test-session")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err, "snap new failed: %s", output)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Created session 'test-session'")
	assert.Contains(t, outputStr, "Next steps:")
	assert.Contains(t, outputStr, "snap plan test-session")
	assert.Contains(t, outputStr, "snap run test-session")

	// Verify directory was created.
	tasksDir := filepath.Join(projectDir, ".snap", "sessions", "test-session", "tasks")
	info, err := os.Stat(tasksDir)
	require.NoError(t, err, "tasks directory should exist")
	assert.True(t, info.IsDir())
}

func TestNewE2E_DuplicateSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	projectDir := t.TempDir()

	// First create succeeds.
	run1 := exec.CommandContext(ctx, binPath, "new", "test-session")
	run1.Dir = projectDir
	_, err = run1.CombinedOutput()
	require.NoError(t, err)

	// Second create fails.
	run2 := exec.CommandContext(ctx, binPath, "new", "test-session")
	run2.Dir = projectDir
	output, err := run2.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(output), "already exists")

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestNewE2E_InvalidName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	projectDir := t.TempDir()
	run := exec.CommandContext(ctx, binPath, "new", "bad name!")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.Error(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "use alphanumeric, hyphens, underscores")

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestNewE2E_NoNameArg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	projectDir := t.TempDir()
	run := exec.CommandContext(ctx, binPath, "new")
	run.Dir = projectDir
	_, err = run.CombinedOutput()
	require.Error(t, err, "snap new with no args should fail")
}

func TestRunE2E_SubcommandWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// `snap run` with empty PATH should hit the provider validation error,
	// which proves the run subcommand routes to the run logic.
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	run := exec.CommandContext(ctx, binPath, "run")
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+t.TempDir())
	output, runErr := run.CombinedOutput()

	require.Error(t, runErr)
	assert.Contains(t, string(output), "not found in PATH",
		"snap run should invoke run logic")
}

func TestStubE2E_PlanNotImplemented(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	run := exec.CommandContext(ctx, binPath, "plan", "test-session")
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)
	assert.Contains(t, string(output), "not implemented")
}

func TestStubE2E_ListNotImplemented(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	run := exec.CommandContext(ctx, binPath, "list")
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)
	assert.Contains(t, string(output), "not implemented")
}

func TestStubE2E_DeleteNotImplemented(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	run := exec.CommandContext(ctx, binPath, "delete", "test-session")
	output, runErr := run.CombinedOutput()
	require.Error(t, runErr)
	assert.Contains(t, string(output), "not implemented")
}

func TestGitignore_CoversSessionsDir(t *testing.T) {
	root := mustModuleRoot(t)
	gitignorePath := filepath.Join(root, ".snap", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err, ".snap/.gitignore must exist")

	content := string(data)
	// The gitignore must contain a pattern that covers sessions/.
	// Current pattern is `*` which ignores everything (including sessions/).
	assert.True(t,
		strings.Contains(content, "sessions") || strings.Contains(content, "*"),
		".snap/.gitignore must cover sessions/ (via explicit rule or wildcard)")
}

func TestNewE2E_BareSnapStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// Bare snap with empty PATH should hit the provider validation error,
	// which proves the run logic is being invoked (backward compat).
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	run := exec.CommandContext(ctx, binPath)
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+t.TempDir())
	output, runErr := run.CombinedOutput()

	require.Error(t, runErr)
	assert.Contains(t, string(output), "not found in PATH",
		"bare snap should still invoke run logic")
}

// --- Integration tests ---

func TestNew_CreatesSessionDirectory(t *testing.T) {
	projectDir := t.TempDir()

	// Change to project dir so session.Create(".", name) works relative to it.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	var outBuf strings.Builder
	newCmd.SetOut(&outBuf)
	defer newCmd.SetOut(nil)

	err = newCmd.RunE(newCmd, []string{"my-session"})
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Created session 'my-session'")
	assert.Contains(t, output, "Next steps:")

	// Verify directory exists.
	info, err := os.Stat(filepath.Join(projectDir, ".snap", "sessions", "my-session", "tasks"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNew_DuplicateSessionErrors(t *testing.T) {
	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	var outBuf strings.Builder
	newCmd.SetOut(&outBuf)
	defer newCmd.SetOut(nil)

	err = newCmd.RunE(newCmd, []string{"dup"})
	require.NoError(t, err)

	err = newCmd.RunE(newCmd, []string{"dup"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
