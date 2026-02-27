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
