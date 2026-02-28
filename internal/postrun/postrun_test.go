package postrun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/model"
)

// mockExecutor is a test double for the LLM executor.
type mockExecutor struct {
	output string
	err    error
}

func (m *mockExecutor) Run(_ context.Context, w io.Writer, _ model.Type, _ ...string) error {
	if m.err != nil {
		return m.err
	}
	_, err := fmt.Fprint(w, m.output)
	return err
}

// capturingExecutor captures the prompt sent to the executor.
type capturingExecutor struct {
	capturedPrompt string
	output         string
}

func (m *capturingExecutor) Run(_ context.Context, w io.Writer, _ model.Type, args ...string) error {
	if len(args) > 0 {
		m.capturedPrompt = args[0]
	}
	_, err := fmt.Fprint(w, m.output)
	return err
}

// mockGHMulti creates a gh script that handles repo view, pr view, and pr create subcommands.
// defaultBranch: returned by "gh repo view"; prViewJSON: returned by "gh pr view" (empty string = exit 1);
// prCreateURL: returned by "gh pr create" (empty string = exit 1 with error).
//
//nolint:unparam // defaultBranch is parameterized for readability even though tests use "main"
func mockGHMulti(t *testing.T, defaultBranch, prViewJSON, prCreateURL string) {
	t.Helper()
	binDir := t.TempDir()

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("case \"$1 $2\" in\n")

	// gh repo view
	script.WriteString("  \"repo view\")\n")
	script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", defaultBranch))
	script.WriteString("    ;;\n")

	// gh pr view
	script.WriteString("  \"pr view\")\n")
	if prViewJSON == "" {
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prViewJSON))
	}
	script.WriteString("    ;;\n")

	// gh pr create
	script.WriteString("  \"pr create\")\n")
	if prCreateURL == "" {
		script.WriteString("    echo 'creation failed' >&2\n")
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prCreateURL))
	}
	script.WriteString("    ;;\n")

	script.WriteString("  *)\n")
	script.WriteString("    echo \"unexpected gh call: $*\" >&2\n")
	script.WriteString("    exit 99\n")
	script.WriteString("    ;;\n")
	script.WriteString("esac\n")

	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script.String()), 0o755)) //nolint:gosec // test script needs execute permission

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

func TestRun_NoRemote(t *testing.T) {
	dir := initGitRepo(t)
	chdir(t, dir)

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "",
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No remote configured, skipping push")
}

func TestRun_PushToBareSelfRemote(t *testing.T) {
	dir := initGitRepo(t)
	bareDir := initBareRemote(t, dir)
	chdir(t, dir)

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: bareDir,
		IsGitHub:  false,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Pushed to origin/")
	assert.Contains(t, output, "Non-GitHub remote, skipping PR and CI")

	// Verify commit exists in bare repo
	gitOut := gitOutput(t, bareDir, "log", "--oneline")
	assert.Contains(t, gitOut, "initial")
}

func TestRun_PushRejected(t *testing.T) {
	dir := initGitRepo(t)
	bareDir := initBareRemote(t, dir)

	// Push initial commit to bare repo
	gitCmd(t, dir, "push", "origin", "HEAD")

	// Create a divergent commit on bare repo by cloning and pushing from another work tree
	cloneDir := t.TempDir()
	gitCmd(t, cloneDir, "clone", bareDir, ".")
	gitCmd(t, cloneDir, "config", "user.email", "test@test.com")
	gitCmd(t, cloneDir, "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(cloneDir, "diverge.txt"), []byte("diverge"), 0o600))
	gitCmd(t, cloneDir, "add", ".")
	gitCmd(t, cloneDir, "commit", "-m", "divergent")
	gitCmd(t, cloneDir, "push", "origin", "HEAD")

	// Now create a local commit that diverges from the bare repo
	require.NoError(t, os.WriteFile(filepath.Join(dir, "local.txt"), []byte("local"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "local divergent")

	chdir(t, dir)

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: bareDir,
		IsGitHub:  false,
	}

	err := Run(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "push failed")
}

func TestRun_NonGitHubRemote(t *testing.T) {
	dir := initGitRepo(t)
	bareDir := initBareRemote(t, dir)
	chdir(t, dir)

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: bareDir,
		IsGitHub:  false,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Pushed to origin/")
	assert.Contains(t, output, "Non-GitHub remote, skipping PR and CI")
}

func TestRun_PRCreation_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch with a commit
	gitCmd(t, dir, "checkout", "-b", "feature-x")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")
	chdir(t, dir)

	// Mock gh: default branch "main", no existing PR, PR create returns URL
	mockGHMulti(t, "main", "", "https://github.com/user/repo/pull/42")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{output: "Add new feature\n\nImplements the feature for better UX."},
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Creating pull request...")
	assert.Contains(t, output, "PR #42 created: https://github.com/user/repo/pull/42")
}

func TestRun_PRSkip_DefaultBranch(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)
	chdir(t, dir)

	// Current branch is "main", default branch is "main"
	mockGHMulti(t, "main", "", "")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{output: "Title\n\nBody"},
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "On default branch, skipping PR creation")
	assert.NotContains(t, output, "Creating pull request...")
}

func TestRun_PRSkip_Existing(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch
	gitCmd(t, dir, "checkout", "-b", "feature-y")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")
	chdir(t, dir)

	// Mock gh: default branch "main", PR exists
	mockGHMulti(t, "main",
		`{"state":"OPEN","url":"https://github.com/user/repo/pull/10"}`,
		"")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{output: "Title\n\nBody"},
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "PR already exists: https://github.com/user/repo/pull/10")
	assert.NotContains(t, output, "Creating pull request...")
}

func TestRun_PRSkip_DetachedHead(t *testing.T) {
	// Test createPRFlow directly with empty branch (simulates detached HEAD).
	// Detached HEAD causes git push to fail, so we test the PR skip logic directly.
	mockGHMulti(t, "main", "", "")

	var buf bytes.Buffer
	cfg := Config{
		Output:   &buf,
		IsGitHub: true,
		Executor: &mockExecutor{output: "Title\n\nBody"},
	}

	// Empty branch = detached HEAD
	_, err := createPRFlow(context.Background(), cfg, "")
	require.NoError(t, err)

	output := buf.String()
	assert.NotContains(t, output, "Creating pull request...")
	assert.NotContains(t, output, "On default branch")
}

func TestRun_PRCreation_UsesPRDContext(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch with a commit
	gitCmd(t, dir, "checkout", "-b", "feature-prd")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")

	// Write a PRD file with identifiable content
	prdPath := filepath.Join(dir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("## Goal\nImplement offline-first sync engine for mobile clients."), 0o600))

	chdir(t, dir)

	// Mock gh: default branch "main", no existing PR, PR create returns URL
	mockGHMulti(t, "main", "", "https://github.com/user/repo/pull/99")

	executor := &capturingExecutor{output: "Add sync engine\n\nImplements offline-first sync for mobile."}

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  executor,
		PRDPath:   prdPath,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	// Verify PRD content was included in the prompt sent to the executor
	assert.Contains(t, executor.capturedPrompt, "offline-first sync engine")
	assert.Contains(t, executor.capturedPrompt, "mobile clients")
}

func TestRun_PRCreation_Failed(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch
	gitCmd(t, dir, "checkout", "-b", "feature-z")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")
	chdir(t, dir)

	// Mock gh: default branch "main", no existing PR, PR create fails
	mockGHMulti(t, "main", "", "")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{output: "Add feature\n\nBody text."},
	}

	err := Run(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PR creation failed")

	output := buf.String()
	assert.Contains(t, output, "PR creation failed")
}

func TestRun_PRCreation_LLMFails(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch
	gitCmd(t, dir, "checkout", "-b", "feature-llm-fail")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")
	chdir(t, dir)

	// Mock gh: default branch "main", no existing PR, PR create succeeds
	mockGHMulti(t, "main", "", "https://github.com/user/repo/pull/77")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{err: fmt.Errorf("LLM timeout")},
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	// PR should still be created with fallback title "Update" when LLM fails
	assert.Contains(t, output, "Creating pull request...")
	assert.Contains(t, output, "PR #77 created: https://github.com/user/repo/pull/77")
}

// gitOutput runs a git command in a directory and returns combined output.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
	return string(out)
}

func TestFormatCheckStatus_FewChecks(t *testing.T) {
	checks := []CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "running"},
		{Name: "build", Status: "pending"},
	}
	result := formatCheckStatus(checks)
	assert.Equal(t, "lint: passed, test: running, build: pending", result)
}

func TestFormatCheckStatus_ManyChecks(t *testing.T) {
	checks := []CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "passed"},
		{Name: "build", Status: "passed"},
		{Name: "e2e", Status: "running"},
		{Name: "deploy", Status: "pending"},
		{Name: "security", Status: "pending"},
	}
	result := formatCheckStatus(checks)
	assert.Equal(t, "3 passed, 1 running, 2 pending", result)
}

func TestFormatCheckStatus_ManyChecks_AllPassed(t *testing.T) {
	checks := make([]CheckResult, 6)
	for i := range checks {
		checks[i] = CheckResult{Name: fmt.Sprintf("check%d", i), Status: "passed"}
	}
	result := formatCheckStatus(checks)
	assert.Equal(t, "6 passed", result)
}

func TestChecksChanged(t *testing.T) {
	prev := []CheckResult{
		{Name: "lint", Status: "running"},
		{Name: "test", Status: "pending"},
	}
	curr := []CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "running"},
	}
	assert.True(t, checksChanged(prev, curr))
}

func TestChecksChanged_Identical(t *testing.T) {
	checks := []CheckResult{
		{Name: "lint", Status: "running"},
		{Name: "test", Status: "pending"},
	}
	assert.False(t, checksChanged(checks, checks))
}

func TestChecksChanged_DifferentLength(t *testing.T) {
	prev := []CheckResult{{Name: "lint", Status: "running"}}
	curr := []CheckResult{
		{Name: "lint", Status: "running"},
		{Name: "test", Status: "pending"},
	}
	assert.True(t, checksChanged(prev, curr))
}

func TestChecksChanged_NilPrev(t *testing.T) {
	curr := []CheckResult{{Name: "lint", Status: "running"}}
	assert.True(t, checksChanged(nil, curr))
}

func TestAllCompleted(t *testing.T) {
	assert.True(t, allCompleted([]CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "failed"},
	}))
	assert.False(t, allCompleted([]CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "running"},
	}))
	assert.False(t, allCompleted([]CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "pending"},
	}))
}

func TestAnyFailed(t *testing.T) {
	assert.True(t, anyFailed([]CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "failed"},
	}))
	assert.False(t, anyFailed([]CheckResult{
		{Name: "lint", Status: "passed"},
		{Name: "test", Status: "passed"},
	}))
}

// addWorkflowFile creates a .github/workflows/ci.yml with a push trigger in the given repo root.
func addWorkflowFile(t *testing.T, repoRoot string) {
	t.Helper()
	wfDir := filepath.Join(repoRoot, ".github", "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))
	content := "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n"
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte(content), 0o600))
}

// mockGHWithCI creates a gh mock script that handles repo view, pr view, pr create, and pr checks.
// checksJSON is returned for "gh pr checks" calls.
func mockGHWithCI(t *testing.T, defaultBranch, prViewJSON, prCreateURL, checksJSON string) {
	t.Helper()
	binDir := t.TempDir()

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("case \"$1 $2\" in\n")

	// gh repo view
	script.WriteString("  \"repo view\")\n")
	script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", defaultBranch))
	script.WriteString("    ;;\n")

	// gh pr view
	script.WriteString("  \"pr view\")\n")
	if prViewJSON == "" {
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prViewJSON))
	}
	script.WriteString("    ;;\n")

	// gh pr create
	script.WriteString("  \"pr create\")\n")
	if prCreateURL == "" {
		script.WriteString("    echo 'creation failed' >&2\n")
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prCreateURL))
	}
	script.WriteString("    ;;\n")

	// gh pr checks
	script.WriteString("  \"pr checks\")\n")
	script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", checksJSON))
	script.WriteString("    ;;\n")

	script.WriteString("  *)\n")
	script.WriteString("    echo \"unexpected gh call: $*\" >&2\n")
	script.WriteString("    exit 99\n")
	script.WriteString("    ;;\n")
	script.WriteString("esac\n")

	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script.String()), 0o755)) //nolint:gosec // test script needs execute permission

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

// mockGHWithCIStateful creates a gh mock that returns different pr checks results on successive calls.
// Uses a counter file to track call number.
func mockGHWithCIStateful(t *testing.T, defaultBranch, prViewJSON, prCreateURL string, checksResponses []string) {
	t.Helper()
	binDir := t.TempDir()
	counterFile := filepath.Join(binDir, "counter")
	require.NoError(t, os.WriteFile(counterFile, []byte("0"), 0o600))

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("case \"$1 $2\" in\n")

	// gh repo view
	script.WriteString("  \"repo view\")\n")
	script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", defaultBranch))
	script.WriteString("    ;;\n")

	// gh pr view
	script.WriteString("  \"pr view\")\n")
	if prViewJSON == "" {
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prViewJSON))
	}
	script.WriteString("    ;;\n")

	// gh pr create
	script.WriteString("  \"pr create\")\n")
	if prCreateURL == "" {
		script.WriteString("    echo 'creation failed' >&2\n")
		script.WriteString("    exit 1\n")
	} else {
		script.WriteString(fmt.Sprintf("    printf '%%s' '%s'\n", prCreateURL))
	}
	script.WriteString("    ;;\n")

	// gh pr checks — stateful
	script.WriteString("  \"pr checks\")\n")
	script.WriteString(fmt.Sprintf("    COUNT=$(cat %s)\n", counterFile))
	script.WriteString("    COUNT=$((COUNT + 1))\n")
	script.WriteString(fmt.Sprintf("    printf '%%s' \"$COUNT\" > %s\n", counterFile))
	script.WriteString("    case $COUNT in\n")
	for i, resp := range checksResponses {
		script.WriteString(fmt.Sprintf("      %d)\n", i+1))
		script.WriteString(fmt.Sprintf("        printf '%%s' '%s'\n", resp))
		script.WriteString("        ;;\n")
	}
	// After all responses exhausted, return the last one
	script.WriteString("      *)\n")
	if len(checksResponses) > 0 {
		script.WriteString(fmt.Sprintf("        printf '%%s' '%s'\n", checksResponses[len(checksResponses)-1]))
	}
	script.WriteString("        ;;\n")
	script.WriteString("    esac\n")
	script.WriteString("    ;;\n")

	script.WriteString("  *)\n")
	script.WriteString("    echo \"unexpected gh call: $*\" >&2\n")
	script.WriteString("    exit 99\n")
	script.WriteString("    ;;\n")
	script.WriteString("esac\n")

	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script.String()), 0o755)) //nolint:gosec // test script needs execute permission

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

func TestRun_CI_AllGreen(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch with a commit
	gitCmd(t, dir, "checkout", "-b", "feature-ci")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")

	// Add workflow file with push trigger
	addWorkflowFile(t, dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add workflows")

	chdir(t, dir)

	mockGHWithCI(t, "main", "", "https://github.com/user/repo/pull/50",
		`[{"name":"lint","state":"SUCCESS","conclusion":"success"},{"name":"test","state":"SUCCESS","conclusion":"success"}]`)

	var buf bytes.Buffer
	cfg := Config{
		Output:       &buf,
		RemoteURL:    "https://github.com/user/repo.git",
		IsGitHub:     true,
		Executor:     &mockExecutor{output: "Add feature\n\nImplements the feature."},
		RepoRoot:     dir,
		PollInterval: time.Millisecond,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Waiting for CI checks...")
	assert.Contains(t, output, "CI passed — PR ready for review")
}

func TestRun_CI_PendingThenGreen(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch with a commit
	gitCmd(t, dir, "checkout", "-b", "feature-ci-pending")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")

	// Add workflow file with push trigger
	addWorkflowFile(t, dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add workflows")

	chdir(t, dir)

	mockGHWithCIStateful(t, "main", "", "https://github.com/user/repo/pull/51",
		[]string{
			// First poll: pending
			`[{"name":"lint","state":"SUCCESS","conclusion":"success"},{"name":"test","state":"PENDING","conclusion":""}]`,
			// Second poll: all green
			`[{"name":"lint","state":"SUCCESS","conclusion":"success"},{"name":"test","state":"SUCCESS","conclusion":"success"}]`,
		})

	var buf bytes.Buffer
	cfg := Config{
		Output:       &buf,
		RemoteURL:    "https://github.com/user/repo.git",
		IsGitHub:     true,
		Executor:     &mockExecutor{output: "Add feature\n\nImplements the feature."},
		RepoRoot:     dir,
		PollInterval: time.Millisecond,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Waiting for CI checks...")
	// First poll prints status with pending check
	assert.Contains(t, output, "lint: passed, test: pending")
	// Second poll shows changed status and final success
	assert.Contains(t, output, "lint: passed, test: passed")
	assert.Contains(t, output, "CI passed — PR ready for review")
}

func TestRun_NoWorkflows(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch — no .github/workflows/
	gitCmd(t, dir, "checkout", "-b", "feature-no-ci")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")

	chdir(t, dir)

	mockGHMulti(t, "main", "", "https://github.com/user/repo/pull/52")

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
		Executor:  &mockExecutor{output: "Add feature\n\nBody."},
		RepoRoot:  dir,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No CI workflows found, done")
	assert.NotContains(t, output, "Waiting for CI checks...")
}

func TestRun_CI_Failed(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)

	// Create feature branch
	gitCmd(t, dir, "checkout", "-b", "feature-ci-fail")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("new feature"), 0o600))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add feature")

	// Add workflow file
	addWorkflowFile(t, dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add workflows")

	chdir(t, dir)

	mockGHWithCI(t, "main", "", "https://github.com/user/repo/pull/53",
		`[{"name":"lint","state":"SUCCESS","conclusion":"success"},{"name":"test","state":"FAILURE","conclusion":"failure"}]`)

	var buf bytes.Buffer
	cfg := Config{
		Output:       &buf,
		RemoteURL:    "https://github.com/user/repo.git",
		IsGitHub:     true,
		Executor:     &mockExecutor{output: "Add feature\n\nBody."},
		RepoRoot:     dir,
		PollInterval: time.Millisecond,
	}

	err := Run(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CI failed")
}

func TestRun_CI_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	addWorkflowFile(t, dir)

	// Mock gh that always returns pending
	mockGHScript(t, `printf '%s' '[{"name":"lint","state":"PENDING","conclusion":""},{"name":"test","state":"PENDING","conclusion":""}]'`)

	ctx, cancel := context.WithCancel(context.Background())

	var buf bytes.Buffer
	cfg := Config{
		Output:       &buf,
		RepoRoot:     dir,
		PollInterval: time.Millisecond,
	}

	// Cancel context shortly after starting
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Test monitorCI directly to avoid push timing issues
	err := monitorCI(ctx, cfg, true, "main")
	// Context cancellation should not return an error
	assert.NoError(t, err)
}
