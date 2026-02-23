package pathutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/pathutil"
)

func TestValidatePath(t *testing.T) {
	// Get current directory for tests
	cwd, err := os.Getwd()
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "docs/tasks/PRD.md",
			wantErr: false,
		},
		{
			name:    "valid absolute path in cwd",
			path:    filepath.Join(cwd, "test.md"),
			wantErr: false,
		},
		{
			name:    "path with newline",
			path:    "test\ninjection.md",
			wantErr: true,
			errMsg:  "newline",
		},
		{
			name:    "path with carriage return",
			path:    "test\rinjection.md",
			wantErr: true,
			errMsg:  "newline",
		},
		{
			name:    "path traversal outside project",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "within project directory",
		},
		{
			name:    "path traversal with dots",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "within project directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pathutil.ValidatePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolvePRDPath(t *testing.T) {
	tests := []struct {
		name        string
		tasksDir    string
		prdPath     string
		expectedPRD string
	}{
		{
			name:        "empty - use default",
			tasksDir:    "docs/tasks",
			prdPath:     "",
			expectedPRD: "docs/tasks/PRD.md",
		},
		{
			name:        "custom tasks dir",
			tasksDir:    "features",
			prdPath:     "",
			expectedPRD: filepath.Join("features", "PRD.md"),
		},
		{
			name:        "prd override",
			tasksDir:    "docs/tasks",
			prdPath:     "custom/prd.md",
			expectedPRD: "custom/prd.md",
		},
		{
			name:        "both override",
			tasksDir:    "ignored",
			prdPath:     "my-prd.md",
			expectedPRD: "my-prd.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathutil.ResolvePRDPath(tt.tasksDir, tt.prdPath)
			assert.Equal(t, tt.expectedPRD, got)
		})
	}
}

func TestCheckPathExists(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-*.md")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name        string
		path        string
		wantExists  bool
		wantWarning bool
	}{
		{
			name:        "existing file",
			path:        tmpFile.Name(),
			wantExists:  true,
			wantWarning: false,
		},
		{
			name:        "non-existent file",
			path:        "/tmp/definitely-does-not-exist-12345.md",
			wantExists:  false,
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, warning := pathutil.CheckPathExists(tt.path)
			assert.Equal(t, tt.wantExists, exists)
			if tt.wantWarning {
				assert.NotEmpty(t, warning)
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}
