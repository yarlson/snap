package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderRequirementsPrompt(t *testing.T) {
	prompt, err := RenderRequirementsPrompt()
	require.NoError(t, err)
	assert.Contains(t, prompt, "## Context")
	assert.Contains(t, prompt, "CLAUDE.md")
	assert.Contains(t, prompt, "docs/context/")
	assert.Contains(t, prompt, "## Process")
	assert.Contains(t, prompt, "/done")
	assert.Contains(t, prompt, "## Completion")
}

func TestRenderPRDPrompt_WithoutBrief(t *testing.T) {
	result, err := RenderPRDPrompt(".snap/sessions/auth/tasks", "")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.NotContains(t, result, "Requirements Brief")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
	assert.Contains(t, result, "Guardrails")
	assert.Contains(t, result, "Completion")
}

func TestRenderPRDPrompt_WithBrief(t *testing.T) {
	result, err := RenderPRDPrompt(".snap/sessions/auth/tasks", "I want OAuth2 auth with Google")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, "Requirements Brief")
	assert.Contains(t, result, "I want OAuth2 auth with Google")
	assert.Contains(t, result, "Guardrails")
}

func TestRenderTechnologyPrompt(t *testing.T) {
	result, err := RenderTechnologyPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TECHNOLOGY.md")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
	assert.Contains(t, result, "Guardrails")
	assert.Contains(t, result, "Completion")
}

func TestRenderDesignPrompt(t *testing.T) {
	result, err := RenderDesignPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TECHNOLOGY.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/DESIGN.md")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
	assert.Contains(t, result, "Guardrails")
	assert.Contains(t, result, "Completion")
}

func TestRenderTasksPrompt(t *testing.T) {
	result, err := RenderTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TECHNOLOGY.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/DESIGN.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TASKS.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TASK")
	assert.Contains(t, result, "vertical slice")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
	assert.Contains(t, result, "Guardrails")
	assert.Contains(t, result, "Completion")
}

func TestAllPrompts_ContainCodebaseExploration(t *testing.T) {
	tests := []struct {
		name   string
		render func() (string, error)
	}{
		{"PRD", func() (string, error) { return RenderPRDPrompt("tasks", "") }},
		{"Technology", func() (string, error) { return RenderTechnologyPrompt("tasks") }},
		{"Design", func() (string, error) { return RenderDesignPrompt("tasks") }},
		{"Tasks", func() (string, error) { return RenderTasksPrompt("tasks") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.render()
			require.NoError(t, err)
			assert.Contains(t, result, "CLAUDE.md")
			assert.Contains(t, result, "docs/context/")
		})
	}
}
