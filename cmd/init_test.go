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

// --- E2E tests (CUJ-1: First Run Success — init portion) ---

func TestInitE2E_CreatesFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Build the snap binary.
	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// Run snap init in a temp directory.
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "docs", "tasks")
	run := exec.CommandContext(ctx, binPath, "init", "-d", tasksDir)
	output, err := run.CombinedOutput()
	require.NoError(t, err, "snap init failed: %s", output)

	outputStr := string(output)

	// Assert files exist.
	_, err = os.Stat(filepath.Join(tasksDir, "PRD.md"))
	assert.NoError(t, err, "PRD.md should exist")
	_, err = os.Stat(filepath.Join(tasksDir, "TASK1.md"))
	assert.NoError(t, err, "TASK1.md should exist")

	// Assert stdout contains "Created" lines.
	assert.Contains(t, outputStr, "Created")
	assert.Contains(t, outputStr, "PRD.md")
	assert.Contains(t, outputStr, "TASK1.md")

	// Assert stdout contains "Next steps".
	assert.Contains(t, outputStr, "Next steps:")
}

func TestInitE2E_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Build the snap binary.
	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// First run.
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "docs", "tasks")
	run1 := exec.CommandContext(ctx, binPath, "init", "-d", tasksDir)
	_, err = run1.CombinedOutput()
	require.NoError(t, err)

	// Read file contents after first run.
	prdContent1, err := os.ReadFile(filepath.Join(tasksDir, "PRD.md"))
	require.NoError(t, err)
	taskContent1, err := os.ReadFile(filepath.Join(tasksDir, "TASK1.md"))
	require.NoError(t, err)

	// Second run.
	run2 := exec.CommandContext(ctx, binPath, "init", "-d", tasksDir)
	output2, err := run2.CombinedOutput()
	require.NoError(t, err)

	// Assert "Already initialized" message.
	assert.Contains(t, string(output2), "Already initialized")

	// Assert files unchanged.
	prdContent2, err := os.ReadFile(filepath.Join(tasksDir, "PRD.md"))
	require.NoError(t, err)
	taskContent2, err := os.ReadFile(filepath.Join(tasksDir, "TASK1.md"))
	require.NoError(t, err)

	assert.Equal(t, prdContent1, prdContent2, "PRD.md should not change on second run")
	assert.Equal(t, taskContent1, taskContent2, "TASK1.md should not change on second run")
}

// --- Integration tests ---
//
// NOTE: These tests mutate the package-level tasksDir variable and initCmd.
// They must not be run in parallel (do not add t.Parallel() to these tests).
// Refactor initRun to accept a config struct if parallelism becomes necessary.

func TestInit_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	var outBuf strings.Builder
	initCmd.SetOut(&outBuf)
	initCmd.SetArgs(nil)
	defer func() {
		initCmd.SetOut(nil)
	}()

	// Override tasksDir for this test.
	origTasksDir := tasksDir
	setTasksDir(origTasksDir)
	defer setTasksDir("docs/tasks")

	err := initCmd.RunE(initCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()

	// Both files created.
	assert.Contains(t, output, "Created "+filepath.Join(origTasksDir, "PRD.md"))
	assert.Contains(t, output, "Created "+filepath.Join(origTasksDir, "TASK1.md"))
	assert.Contains(t, output, "Next steps:")

	// Files actually exist.
	_, err = os.Stat(filepath.Join(origTasksDir, "PRD.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(origTasksDir, "TASK1.md"))
	assert.NoError(t, err)
}

func TestInit_PRDExists_OnlyCreatesTask(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	// Create PRD.md manually.
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "PRD.md"), []byte("existing PRD"), 0o600))

	var outBuf strings.Builder
	initCmd.SetOut(&outBuf)
	defer initCmd.SetOut(nil)

	setTasksDir(tasksDir)
	defer setTasksDir("docs/tasks")

	err := initCmd.RunE(initCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()

	// Only TASK1.md created.
	assert.NotContains(t, output, "Created "+filepath.Join(tasksDir, "PRD.md"))
	assert.Contains(t, output, "Created "+filepath.Join(tasksDir, "TASK1.md"))
	assert.Contains(t, output, "Next steps:")

	// PRD.md unchanged.
	content, err := os.ReadFile(filepath.Join(tasksDir, "PRD.md"))
	require.NoError(t, err)
	assert.Equal(t, "existing PRD", string(content))
}

func TestInit_AllFilesExist(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	// Create both files manually.
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "PRD.md"), []byte("existing PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("existing TASK"), 0o600))

	var outBuf strings.Builder
	initCmd.SetOut(&outBuf)
	defer initCmd.SetOut(nil)

	setTasksDir(tasksDir)
	defer setTasksDir("docs/tasks")

	err := initCmd.RunE(initCmd, nil)
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Already initialized")
	assert.NotContains(t, output, "Created")
}

func TestInit_CreatesParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "deep", "nested", "tasks")

	var outBuf strings.Builder
	initCmd.SetOut(&outBuf)
	defer initCmd.SetOut(nil)

	setTasksDir(tasksDir)
	defer setTasksDir("docs/tasks")

	err := initCmd.RunE(initCmd, nil)
	require.NoError(t, err)

	// Directory and files should exist.
	_, err = os.Stat(filepath.Join(tasksDir, "PRD.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tasksDir, "TASK1.md"))
	assert.NoError(t, err)
}

func TestInit_ReadOnlyDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o755))
	require.NoError(t, os.Chmod(readOnlyDir, 0o444))
	t.Cleanup(func() {
		os.Chmod(readOnlyDir, 0o755) //nolint:errcheck // best-effort cleanup in test
	})

	tasksDir := filepath.Join(readOnlyDir, "tasks")

	var outBuf strings.Builder
	initCmd.SetOut(&outBuf)
	defer initCmd.SetOut(nil)

	setTasksDir(tasksDir)
	defer setTasksDir("docs/tasks")

	err := initCmd.RunE(initCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create")
}

func TestInit_PersistentFlagWorksForBothCommands(t *testing.T) {
	// Verify --tasks-dir is a persistent flag accessible by both root and init.
	pf := rootCmd.PersistentFlags().Lookup("tasks-dir")
	require.NotNil(t, pf, "--tasks-dir should be a persistent flag on rootCmd")
	assert.Equal(t, "d", pf.Shorthand, "--tasks-dir should have -d shorthand")
}

// --- Unit tests ---

func TestInit_TemplateContent(t *testing.T) {
	assert.NotEmpty(t, prdTemplate, "PRD template should not be empty")
	assert.NotEmpty(t, taskTemplate, "TASK template should not be empty")

	assert.Contains(t, prdTemplate, "# Product Requirements")
	assert.Contains(t, taskTemplate, "## Objective")
	assert.Contains(t, taskTemplate, "## Requirements")
	assert.Contains(t, taskTemplate, "## Acceptance Criteria")
}

// setTasksDir is a test helper that sets the tasksDir package variable.
func setTasksDir(dir string) {
	tasksDir = dir
}
