package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test: snap status on fresh project auto-creates "default" session and shows empty status.
func TestE2E_StatusFreshProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	binPath := buildSnap(t)
	projectDir := t.TempDir()
	ctx := context.Background()

	// Run snap status on a fresh project with no sessions.
	status := exec.CommandContext(ctx, binPath, "status")
	status.Dir = projectDir
	output, err := status.CombinedOutput()
	require.NoError(t, err, "snap status (fresh project) failed: %s", output)

	outputStr := string(output)

	// Should show "default" session with no tasks.
	assert.Contains(t, outputStr, "Session: default")
	assert.Contains(t, outputStr, "No task files found")

	// Auto-creation should be silent — no "created" message in output.
	assert.NotContains(t, outputStr, "created")

	// The "default" session directory should exist on disk.
	defaultSessionDir := filepath.Join(projectDir, ".snap", "sessions", "default")
	info, err := os.Stat(defaultSessionDir)
	require.NoError(t, err, "default session directory should exist")
	assert.True(t, info.IsDir())
}
