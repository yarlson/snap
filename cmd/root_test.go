package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/yarlson/snap/internal/pathutil"
)

func TestFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedPRD string
	}{
		{
			name:        "default paths",
			args:        []string{},
			expectedPRD: "docs/tasks/PRD.md",
		},
		{
			name:        "custom tasks dir",
			args:        []string{"--tasks-dir", "features"},
			expectedPRD: "features/PRD.md",
		},
		{
			name:        "custom tasks dir short flag",
			args:        []string{"-d", "docs"},
			expectedPRD: "docs/PRD.md",
		},
		{
			name:        "custom prd path overrides tasks dir",
			args:        []string{"--tasks-dir", "features", "--prd", "custom/requirements.md"},
			expectedPRD: "custom/requirements.md",
		},
		{
			name:        "short flags",
			args:        []string{"-d", "features", "-p", "my-prd.md"},
			expectedPRD: "my-prd.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to defaults
			tasksDir = "docs/tasks"
			prdPath = ""

			// Create a fresh command for each test
			cmd := rootCmd
			cmd.SetArgs(tt.args)

			// Parse flags
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Apply defaults using pathutil (same as run() does)
			prdPath = pathutil.ResolvePRDPath(tasksDir, prdPath)

			// Verify resolved paths
			assert.Equal(t, tt.expectedPRD, prdPath)
		})
	}
}

func TestRootCommand_InvalidFlagDoesNotPrintUsage(t *testing.T) {
	// Use the shared root command but restore test-facing settings afterward.
	origArgs := rootCmd.Flags().Args()
	_ = origArgs // Cobra does not expose current raw args; keep local pattern explicit.

	var outBuf strings.Builder
	var errBuf strings.Builder
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--definitely-not-a-real-flag"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	output := errBuf.String()
	assert.NotContains(t, output, "Usage:", "usage/help should not be printed on errors")
	assert.Empty(t, output, "cobra should not print the error when Execute() handles it")

	// Reset to avoid leaking test state into other tests.
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
}

func TestRootCommand_SilencesUsageAndErrors(t *testing.T) {
	assert.True(t, rootCmd.SilenceUsage, "usage/help should be suppressed on errors")
	assert.True(t, rootCmd.SilenceErrors, "cobra should not print errors when Execute() handles them")
}

func TestVersion_DefaultValue(t *testing.T) {
	assert.Equal(t, "dev", Version, "Version should default to dev")
}

func TestVersion_FlagRecognized(t *testing.T) {
	var outBuf strings.Builder
	rootCmd.SetOut(&outBuf)
	rootCmd.SetArgs([]string{"--version"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "snap dev\n", outBuf.String())

	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
}

func TestVersion_LdflagsInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Build binary with custom version via ldflags.
	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build",
		"-ldflags", "-X github.com/yarlson/snap/cmd.Version=v1.2.3",
		"-o", binPath, ".",
	)
	// Build from the module root (one directory up from cmd/).
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// Execute binary with --version.
	run := exec.CommandContext(ctx, binPath, "--version")
	output, err := run.Output()
	require.NoError(t, err)

	assert.Equal(t, "snap v1.2.3\n", string(output))
}

// mustModuleRoot returns the module root by walking up from the current file.
func mustModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find go.mod")
		dir = parent
	}
}

// ciWorkflow is a minimal representation of GitHub Actions workflow YAML.
type ciWorkflow struct {
	On   ciOn             `yaml:"on"`
	Jobs map[string]ciJob `yaml:"jobs"`
}

type ciOn struct {
	Push        ciBranches `yaml:"push"`
	PullRequest ciBranches `yaml:"pull_request"`
}

type ciBranches struct {
	Branches []string `yaml:"branches"`
}

type ciJob struct {
	Steps []ciStep `yaml:"steps"`
}

type ciStep struct {
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
}

func loadCIWorkflow(t *testing.T) ciWorkflow {
	t.Helper()
	root := mustModuleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err, ".github/workflows/ci.yml must exist")

	var wf ciWorkflow
	require.NoError(t, yaml.Unmarshal(data, &wf), "ci.yml must be valid YAML")
	return wf
}

func TestPreflightProviderCLI_MissingBinary(t *testing.T) {
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

	// Create a tasks directory with a valid task file.
	tasksDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "TASK1.md"), []byte("# Task 1\n"), 0o600))

	// Run snap with an empty PATH so claude is not found.
	run := exec.CommandContext(ctx, binPath, "-d", tasksDir)
	run.Env = append(os.Environ(), "PATH="+t.TempDir())
	output, runErr := run.CombinedOutput()

	require.Error(t, runErr)
	outputStr := string(output)
	assert.Contains(t, outputStr, "not found in PATH")
	assert.Contains(t, outputStr, "https://")

	// Verify exit code is 1.
	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestCI_WorkflowExistsAndValid(t *testing.T) {
	loadCIWorkflow(t) // fails if file missing or invalid YAML
}

func TestCI_TriggersOnMainPushAndPR(t *testing.T) {
	wf := loadCIWorkflow(t)

	assert.Contains(t, wf.On.Push.Branches, "main", "CI should trigger on push to main")
	assert.Contains(t, wf.On.PullRequest.Branches, "main", "CI should trigger on PR to main")
}

func TestGracefulSignalHandling_ExitCode130(t *testing.T) {
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

	// Create a project directory with a tasks subdirectory.
	// pathutil.ValidatePath requires tasks dir within cwd.
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "TASK1.md"), []byte("# Task 1\nDo something long"), 0o600))

	// Create a mock provider script that blocks until killed.
	// Use exec so sleep replaces the shell — no orphaned child holding the pipe open.
	mockBinDir := t.TempDir()
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte("#!/bin/sh\nexec /bin/sleep 3600\n"), 0o755)) //nolint:gosec // G306: test mock script needs exec permission

	run := exec.CommandContext(ctx, binPath)
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockBinDir+":/usr/bin:/bin")

	// Start the process.
	require.NoError(t, run.Start())

	// Give it time to start up and reach the workflow.
	// Note: This is a fixed sleep which can be flaky on very slow systems,
	// but is acceptable for E2E tests. A more robust approach would wait for
	// a sentinel output, but that adds complexity for marginal benefit.
	time.Sleep(2 * time.Second)

	// Send SIGINT.
	require.NoError(t, run.Process.Signal(syscall.SIGINT))

	// Wait for exit.
	runErr := run.Wait()
	require.Error(t, runErr)

	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, 130, exitErr.ExitCode(), "SIGINT should produce exit code 130")
}

func TestGracefulSignalHandling_InterruptedMessage(t *testing.T) {
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

	// Create a project directory with a tasks subdirectory.
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "TASK1.md"), []byte("# Task 1\nDo something"), 0o600))

	// Create a mock provider that blocks until killed.
	// Use exec so sleep replaces the shell — no orphaned child holding the pipe open.
	mockBinDir := t.TempDir()
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte("#!/bin/sh\nexec /bin/sleep 3600\n"), 0o755)) //nolint:gosec // G306: test mock script needs exec permission

	run := exec.CommandContext(ctx, binPath)
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockBinDir+":/usr/bin:/bin")

	// Capture combined output.
	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut

	require.NoError(t, run.Start())
	// Give it time to start up and reach the workflow.
	// Note: This is a fixed sleep which can be flaky on very slow systems,
	// but is acceptable for E2E tests.
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))

	//nolint:errcheck // we expect non-zero exit
	_ = run.Wait()

	output := combinedOut.String()
	assert.Contains(t, output, "Stopped by user", "output should contain interruption message")
}

func TestShowState_HumanReadable_ActiveTask(t *testing.T) {
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

	// Create a project directory with a state file containing an active task.
	projectDir := t.TempDir()
	stateDir := filepath.Join(projectDir, ".snap")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	stateJSON := `{
		"tasks_dir": "docs/tasks",
		"current_task_id": "TASK2",
		"current_task_file": "TASK2.md",
		"current_step": 5,
		"total_steps": 10,
		"completed_task_ids": ["TASK1"],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "docs/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600))

	run := exec.CommandContext(ctx, binPath, "--show-state")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "TASK2 in progress")
	assert.Contains(t, outputStr, "step 5/10")
	assert.Contains(t, outputStr, "Apply fixes")
	assert.NotContains(t, outputStr, "{", "human-readable output should not contain JSON")
}

func TestShowState_JSON_ActiveTask(t *testing.T) {
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
	stateDir := filepath.Join(projectDir, ".snap")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	stateJSON := `{
		"tasks_dir": "docs/tasks",
		"current_task_id": "TASK2",
		"current_task_file": "TASK2.md",
		"current_step": 5,
		"total_steps": 10,
		"completed_task_ids": ["TASK1"],
		"session_id": "",
		"last_updated": "2025-01-01T00:00:00Z",
		"prd_path": "docs/tasks/PRD.md"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600))

	run := exec.CommandContext(ctx, binPath, "--show-state", "--json")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err)

	// Verify output is valid JSON.
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(output, &parsed), "output should be valid JSON")
	assert.Equal(t, "TASK2", parsed["current_task_id"])
}

func TestShowState_NoStateFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	binPath := filepath.Join(t.TempDir(), "snap")
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Dir = mustModuleRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// Run from a temp dir with no state file.
	projectDir := t.TempDir()
	run := exec.CommandContext(ctx, binPath, "--show-state")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	require.NoError(t, err)

	assert.Contains(t, string(output), "No state file exists")
}

func TestJSONFlag_IgnoredWithoutShowState(t *testing.T) {
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

	// Run snap with --json but without --show-state, using empty PATH so
	// provider validation fails. If --json were not ignored, we'd get JSON
	// output or a different error. Instead we expect the normal workflow
	// flow which hits the provider pre-flight check.
	run := exec.CommandContext(ctx, binPath, "--json")
	run.Env = append(os.Environ(), "PATH="+t.TempDir())
	output, runErr := run.CombinedOutput()

	require.Error(t, runErr)
	outputStr := string(output)
	assert.Contains(t, outputStr, "not found in PATH", "--json without --show-state should run normal workflow")
	assert.NotContains(t, outputStr, "{", "should not produce JSON output")
}

func TestE2E_TaskFileErrorRecovery_CaseMismatch(t *testing.T) {
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

	// Create a project directory with a tasks subdirectory containing a lowercase task file.
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "task1.md"), []byte("# Task 1\n"), 0o600))

	// Create a mock provider so pre-flight passes.
	mockBinDir := t.TempDir()
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte("#!/bin/sh\nexit 0\n"), 0o755)) //nolint:gosec // G306: test mock script needs exec permission

	run := exec.CommandContext(ctx, binPath)
	run.Dir = projectDir
	run.Env = append(os.Environ(), "PATH="+mockBinDir+":/usr/bin:/bin")
	output, runErr := run.CombinedOutput()

	require.Error(t, runErr)
	outputStr := string(output)
	assert.Contains(t, outputStr, "Found: task1.md")
	assert.Contains(t, outputStr, "rename to TASK1.md")
	assert.Contains(t, outputStr, "snap init")
}

func TestCI_RunsLintAndRaceTests(t *testing.T) {
	wf := loadCIWorkflow(t)

	// Check lint job uses golangci-lint.
	lintJob, ok := wf.Jobs["lint"]
	require.True(t, ok, "CI must have a lint job")
	var hasLint bool
	for _, step := range lintJob.Steps {
		if strings.HasPrefix(step.Uses, "golangci/golangci-lint-action@") {
			hasLint = true
			break
		}
	}
	assert.True(t, hasLint, "lint job must use golangci-lint-action")

	// Check test job runs go test with -race.
	testJob, ok := wf.Jobs["test"]
	require.True(t, ok, "CI must have a test job")
	var hasRaceTest bool
	for _, step := range testJob.Steps {
		if strings.Contains(step.Run, "go test") && strings.Contains(step.Run, "-race") {
			hasRaceTest = true
			break
		}
	}
	assert.True(t, hasRaceTest, "test job must run go test with -race flag")
}

func TestE2E_NoColor_VersionOutputClean(t *testing.T) {
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

	// Run with NO_COLOR=1.
	run := exec.CommandContext(ctx, binPath, "--version")
	run.Env = append(os.Environ(), "NO_COLOR=1")
	output, err := run.Output()
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "snap", "version output should contain 'snap'")
	assert.NotContains(t, outputStr, "\033", "NO_COLOR=1 output must contain no escape sequences")
}

func TestE2E_NonTTY_DisablesColorsAutomatically(t *testing.T) {
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

	// Create a project directory with a task file.
	projectDir := t.TempDir()
	tasksSubDir := filepath.Join(projectDir, "docs", "tasks")
	require.NoError(t, os.MkdirAll(tasksSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksSubDir, "TASK1.md"), []byte("# Task 1\nDo something"), 0o600))

	// Create a mock provider that blocks until killed.
	mockBinDir := t.TempDir()
	mockClaude := filepath.Join(mockBinDir, "claude")
	require.NoError(t, os.WriteFile(mockClaude, []byte("#!/bin/sh\nexec /bin/sleep 3600\n"), 0o755)) //nolint:gosec // G306: test mock script needs exec permission

	run := exec.CommandContext(ctx, binPath)
	run.Dir = projectDir
	// Explicitly filter NO_COLOR from environment — only non-TTY detection
	// (pipe stdout) should disable colors.
	env := filterEnv(os.Environ(), "NO_COLOR")
	env = append(env, "PATH="+mockBinDir+":/usr/bin:/bin")
	run.Env = env

	var combinedOut strings.Builder
	run.Stdout = &combinedOut
	run.Stderr = &combinedOut

	require.NoError(t, run.Start())
	time.Sleep(2 * time.Second)
	require.NoError(t, run.Process.Signal(syscall.SIGINT))

	//nolint:errcheck // we expect non-zero exit
	_ = run.Wait()

	output := combinedOut.String()
	// Non-TTY stdout (pipe) should auto-disable colors — no ANSI escape sequences.
	assert.NotContains(t, output, "\033",
		"non-TTY stdout should automatically disable colors (no escape sequences)")
	// Verify the interrupt message is still readable.
	assert.Contains(t, output, "Stopped by user",
		"output should contain readable interrupt message")
}

// filterEnv returns a copy of env with all entries matching the given key removed.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
