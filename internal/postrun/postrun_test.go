package postrun

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestRun_GitHubRemote_StopsAfterPush(t *testing.T) {
	dir := initGitRepo(t)
	initBareRemote(t, dir)
	chdir(t, dir)

	var buf bytes.Buffer
	cfg := Config{
		Output:    &buf,
		RemoteURL: "https://github.com/user/repo.git",
		IsGitHub:  true,
	}

	err := Run(context.Background(), cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Pushed to origin/")
	// TASK2 extends this path — for now, push + stop
	assert.NotContains(t, output, "Non-GitHub remote")
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
