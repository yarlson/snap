package postrun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasRelevantWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // path relative to .github/workflows/ → content
		expected bool
	}{
		{
			name: "push trigger",
			files: map[string]string{
				"ci.yml": "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "pull_request trigger",
			files: map[string]string{
				"ci.yaml": "on: pull_request\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "both triggers",
			files: map[string]string{
				"ci.yml": "on: [push, pull_request]\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "schedule only",
			files: map[string]string{
				"cron.yml": "on:\n  schedule:\n    - cron: '0 0 * * *'\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: false,
		},
		{
			name: "workflow_dispatch only",
			files: map[string]string{
				"manual.yml": "on: workflow_dispatch\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: false,
		},
		{
			name:     "no workflows dir",
			files:    nil, // don't create .github/workflows/
			expected: false,
		},
		{
			name:     "empty workflows dir",
			files:    map[string]string{}, // create empty dir
			expected: false,
		},
		{
			name: "malformed file",
			files: map[string]string{
				"bad.yml": "this is not yaml at all just random text\nno triggers here\n",
			},
			expected: false,
		},
		{
			name: "multiple files one relevant",
			files: map[string]string{
				"deploy.yml": "on: workflow_dispatch\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
				"ci.yml":     "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "push in map-style trigger",
			files: map[string]string{
				"ci.yml": "on:\n  push:\n    branches: [main]\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "pull_request in map-style trigger",
			files: map[string]string{
				"ci.yml": "on:\n  pull_request:\n    branches: [main]\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: true,
		},
		{
			name: "non-yml extension ignored",
			files: map[string]string{
				"notes.txt": "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()

			if tt.files != nil {
				wfDir := filepath.Join(repoRoot, ".github", "workflows")
				require.NoError(t, os.MkdirAll(wfDir, 0o755))

				for name, content := range tt.files {
					require.NoError(t, os.WriteFile(filepath.Join(wfDir, name), []byte(content), 0o600))
				}
			}

			result, err := HasRelevantWorkflows(repoRoot)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
