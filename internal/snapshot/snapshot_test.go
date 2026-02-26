package snapshot_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/snapshot"
)

// initGitRepo creates a git repo in dir with one initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# init"), 0o600))
	run("add", ".")
	run("commit", "-m", "initial commit")
}

// stashList returns the stash reflog entries for the repo at dir.
func stashList(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", "stash", "list")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func TestCapture_WithChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Modify tracked file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# modified"), 0o600))

	s := snapshot.New(dir)
	created, err := s.Capture(context.Background(), "snap: TASK1 step 1/9 — Implement")
	require.NoError(t, err)
	assert.True(t, created)

	entries := stashList(t, dir)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], "snap: TASK1 step 1/9")
}

func TestCapture_CleanTree(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	s := snapshot.New(dir)
	created, err := s.Capture(context.Background(), "snap: TASK1 step 1/9 — Implement")
	require.NoError(t, err)
	assert.False(t, created)

	entries := stashList(t, dir)
	assert.Empty(t, entries)
}

func TestCapture_UntrackedFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Add a new file without git-adding it
	require.NoError(t, os.WriteFile(filepath.Join(dir, "newfile.go"), []byte("package main"), 0o600))

	s := snapshot.New(dir)
	created, err := s.Capture(context.Background(), "snap: TASK1 step 2/9 — Check")
	require.NoError(t, err)
	assert.True(t, created)

	entries := stashList(t, dir)
	require.Len(t, entries, 1)

	// Verify the stash contains the untracked file by showing the stash diff
	cmd := exec.CommandContext(context.Background(), "git", "stash", "show", "--name-only", "stash@{0}")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "newfile.go")

	// Verify the untracked file is still untracked (not staged)
	cmd = exec.CommandContext(context.Background(), "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err = cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "?? newfile.go")
}

func TestCapture_MessageFormat(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed"), 0o600))

	msg := "snap: TASK3 step 5/9 — Apply fixes"
	s := snapshot.New(dir)
	created, err := s.Capture(context.Background(), msg)
	require.NoError(t, err)
	assert.True(t, created)

	entries := stashList(t, dir)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], msg)
}
