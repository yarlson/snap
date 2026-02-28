package postrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGH creates a fake gh script in a temp directory and prepends it to PATH.
// The script writes the given stdout to stdout.
func mockGH(t *testing.T, stdout string) {
	t.Helper()
	binDir := t.TempDir()

	script := "#!/bin/sh\n"
	if stdout != "" {
		script += "printf '%s' '" + stdout + "'\n"
	}

	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script), 0o755)) //nolint:gosec // test script needs execute permission

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

// mockGHScript creates a fake gh script with custom shell content.
func mockGHScript(t *testing.T, script string) {
	t.Helper()
	binDir := t.TempDir()

	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte("#!/bin/sh\n"+script), 0o755)) //nolint:gosec // test script needs execute permission

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

func TestDefaultBranch(t *testing.T) {
	mockGH(t, "main")

	branch, err := DefaultBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestDefaultBranch_DevelopBranch(t *testing.T) {
	mockGH(t, "develop")

	branch, err := DefaultBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "develop", branch)
}

func TestPRExists_NoPR(t *testing.T) {
	mockGHScript(t, "exit 1\n")

	exists, url, err := PRExists(context.Background())
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, url)
}

func TestPRExists_HasPR(t *testing.T) {
	mockGH(t, `{"state":"OPEN","url":"https://github.com/user/repo/pull/42"}`)

	exists, url, err := PRExists(context.Background())
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "https://github.com/user/repo/pull/42", url)
}

func TestCreatePR(t *testing.T) {
	mockGH(t, "https://github.com/user/repo/pull/42")

	url, err := CreatePR(context.Background(), "Add feature", "Body text")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/user/repo/pull/42", url)
}

func TestCreatePR_Failure(t *testing.T) {
	mockGHScript(t, "echo 'permission denied' >&2\nexit 1\n")

	_, err := CreatePR(context.Background(), "Title", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}
