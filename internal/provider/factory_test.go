package provider

import (
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
