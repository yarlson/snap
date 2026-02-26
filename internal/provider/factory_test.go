package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/claude"
	"github.com/yarlson/snap/internal/codex"
)

func TestNewExecutorFromEnv(t *testing.T) {
	t.Setenv(envVar, "")
	executor, err := NewExecutorFromEnv()
	require.NoError(t, err)
	assert.IsType(t, &claude.Executor{}, executor)
}

func TestNewExecutorFromEnv_Codex(t *testing.T) {
	t.Setenv(envVar, "codex")
	executor, err := NewExecutorFromEnv()
	require.NoError(t, err)
	assert.IsType(t, &codex.Executor{}, executor)
}

func TestNewExecutorFromEnv_Invalid(t *testing.T) {
	t.Setenv(envVar, "unknown")
	executor, err := NewExecutorFromEnv()
	require.Error(t, err)
	assert.Nil(t, executor)
	assert.Contains(t, err.Error(), envVar)
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: defaultProvider},
		{name: "claude alias", input: "claude-code", expected: "claude"},
		{name: "codex mixed case", input: "CoDeX", expected: "codex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalize(tt.input))
		})
	}
}

func TestValidateCLI_ClaudeMissing(t *testing.T) {
	// Set PATH to a temp directory without claude.
	t.Setenv("PATH", t.TempDir())

	err := ValidateCLI("claude")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude")
	assert.Contains(t, err.Error(), "not found in PATH")
	assert.Contains(t, err.Error(), "https://docs.anthropic.com")
}

func TestValidateCLI_CodexMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := ValidateCLI("codex")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "codex")
	assert.Contains(t, err.Error(), "not found in PATH")
	assert.Contains(t, err.Error(), "https://")
	assert.Contains(t, err.Error(), "SNAP_PROVIDER=claude")
}

func TestValidateCLI_BinaryExists(t *testing.T) {
	// Create a mock binary in a temp directory.
	dir := t.TempDir()
	binaryName := "claude"
	if runtime.GOOS == "windows" {
		binaryName = "claude.exe"
	}
	mockBin := filepath.Join(dir, binaryName)
	require.NoError(t, os.WriteFile(mockBin, []byte("#!/bin/sh\n"), 0o755)) //nolint:gosec // G306: executable permission required for LookPath

	t.Setenv("PATH", dir)

	err := ValidateCLI("claude")
	assert.NoError(t, err)
}

func TestValidateCLI_UnknownProvider(t *testing.T) {
	err := ValidateCLI("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidateCLI_ProviderBinaryMapping(t *testing.T) {
	// Verify all supported providers have a binary mapping.
	for _, p := range []string{"claude", "codex"} {
		t.Run(p, func(t *testing.T) {
			// With empty PATH, error should contain the provider-specific binary name.
			t.Setenv("PATH", t.TempDir())
			err := ValidateCLI(p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not found in PATH")
		})
	}
}

func TestValidateCLI_ErrorFormat(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := ValidateCLI("claude")
	require.Error(t, err)

	msg := err.Error()
	// DESIGN.md pattern: Error: <what> + context + fix.
	assert.Contains(t, msg, "claude not found in PATH")
	assert.Contains(t, msg, "Install it:")
	assert.Contains(t, msg, "Or use a different provider:")
}

func TestProviderMapMatchesExecutorFactory(t *testing.T) {
	// Guard against drift: every provider in the ValidateCLI map must also
	// be supported by NewExecutorFromEnv. If this test fails, a provider was added
	// to one but not the other.
	for name := range providers {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envVar, name)
			_, err := NewExecutorFromEnv()
			assert.NoError(t, err, "provider %q is in ValidateCLI map but not supported by NewExecutorFromEnv", name)
		})
	}
}
