package plan

import (
	"strings"
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

func TestRenderAnalyzeTasksPrompt(t *testing.T) {
	result, err := RenderAnalyzeTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	// Contains tasksDir references.
	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TECHNOLOGY.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/DESIGN.md")

	// Context loading.
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")

	// Definitions from create-tasks.
	assert.Contains(t, result, "Walking Skeleton")
	assert.Contains(t, result, "source files, tests, and build tooling already exist")
	assert.Contains(t, result, "Scope (In) bullets")
	assert.Contains(t, result, "Acceptance criteria")
	assert.Contains(t, result, "vertical slice")

	// Anti-pattern criteria from assess-tasks.
	assert.Contains(t, result, "Horizontal Slice")
	assert.Contains(t, result, "Infrastructure/Docs-Only")
	assert.Contains(t, result, "Too Broad")
	assert.Contains(t, result, "Too Narrow")
	assert.Contains(t, result, "Non-Demoable")

	// Verdict labels.
	assert.Contains(t, result, "PASS")
	assert.Contains(t, result, "MERGE")
	assert.Contains(t, result, "ABSORB")
	assert.Contains(t, result, "SPLIT")
	assert.Contains(t, result, "REWORK")

	// Refinement from merge-tasks.
	assert.Contains(t, result, "Re-verify")

	// Guardrails.
	assert.Contains(t, result, "Guardrails")

	// Preamble (via prependPreamble).
	assert.Contains(t, result, "simplest solution")
}

func TestRenderGenerateTasksPrompt(t *testing.T) {
	result, err := RenderGenerateTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	// Contains tasksDir reference for TASKS.md.
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TASKS.md")

	// Contains TASKS.md section format (A–J).
	for _, section := range []string{"A.", "B.", "C.", "D.", "E.", "F.", "G.", "H.", "I.", "J."} {
		assert.Contains(t, result, section, "should contain section %s", section)
	}

	// Contains TASK<N>.md 15-section format.
	assert.Contains(t, result, "0. Task Type and Placement")
	assert.Contains(t, result, "14. Follow-ups Unlocked")

	// Contains subagent instructions.
	assert.Contains(t, result, "Agent tool")
	assert.Contains(t, result, "subagent")

	// Context loading for subagents.
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")

	// Guardrails.
	assert.Contains(t, result, "Guardrails")

	// Preamble (via prependPreamble).
	assert.Contains(t, result, "simplest solution")
}

func TestRenderPrinciplesPreamble(t *testing.T) {
	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)
	assert.NotEmpty(t, preamble)
	assert.Contains(t, preamble, "KISS")
	assert.Contains(t, preamble, "DRY")
	assert.Contains(t, preamble, "SOLID")
	assert.Contains(t, preamble, "YAGNI")
}

func TestPrependPreamble(t *testing.T) {
	result, err := prependPreamble("hello world")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "result should start with preamble")
	assert.Contains(t, result, "\n\n")
	assert.True(t, strings.HasSuffix(result, "hello world"), "result should end with original prompt")
}

func TestPreamblePrepended_PRD(t *testing.T) {
	result, err := RenderPRDPrompt(".snap/sessions/auth/tasks", "")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "PRD prompt should start with preamble")
}

func TestPreamblePrepended_Technology(t *testing.T) {
	result, err := RenderTechnologyPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "Technology prompt should start with preamble")
}

func TestPreamblePrepended_Design(t *testing.T) {
	result, err := RenderDesignPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "Design prompt should start with preamble")
}

func TestPreamblePrepended_AnalyzeTasks(t *testing.T) {
	result, err := RenderAnalyzeTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "AnalyzeTasks prompt should start with preamble")
}

func TestPreamblePrepended_GenerateTasks(t *testing.T) {
	result, err := RenderGenerateTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "GenerateTasks prompt should start with preamble")
}

func TestAllPrompts_ContainCodebaseExploration(t *testing.T) {
	tests := []struct {
		name   string
		render func() (string, error)
	}{
		{"PRD", func() (string, error) { return RenderPRDPrompt("tasks", "") }},
		{"Technology", func() (string, error) { return RenderTechnologyPrompt("tasks") }},
		{"Design", func() (string, error) { return RenderDesignPrompt("tasks") }},
		{"AnalyzeTasks", func() (string, error) { return RenderAnalyzeTasksPrompt("tasks") }},
		{"GenerateTasks", func() (string, error) { return RenderGenerateTasksPrompt("tasks") }},
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
