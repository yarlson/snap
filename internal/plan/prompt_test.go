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

func TestRenderCreateTasksPrompt(t *testing.T) {
	result, err := RenderCreateTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/PRD.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/TECHNOLOGY.md")
	assert.Contains(t, result, ".snap/sessions/auth/tasks/DESIGN.md")
	assert.Contains(t, result, "Walking Skeleton")
	assert.Contains(t, result, "source files, tests, and build tooling already exist")
	assert.Contains(t, result, "Scope (In) bullets")
	assert.Contains(t, result, "Acceptance criteria")
	assert.Contains(t, result, "vertical slice")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
	assert.Contains(t, result, "Guardrails")
}

func TestRenderAssessTasksPrompt(t *testing.T) {
	result, err := RenderAssessTasksPrompt()
	require.NoError(t, err)

	// All 5 anti-pattern names.
	assert.Contains(t, result, "Horizontal Slice")
	assert.Contains(t, result, "Infrastructure/Docs-Only")
	assert.Contains(t, result, "Too Broad")
	assert.Contains(t, result, "Too Narrow")
	assert.Contains(t, result, "Non-Demoable")

	// All verdict labels.
	assert.Contains(t, result, "PASS")
	assert.Contains(t, result, "MERGE")
	assert.Contains(t, result, "ABSORB")
	assert.Contains(t, result, "SPLIT")
	assert.Contains(t, result, "REWORK")

	assert.Contains(t, result, "Guardrails")
}

func TestRenderMergeTasksPrompt(t *testing.T) {
	result, err := RenderMergeTasksPrompt()
	require.NoError(t, err)

	assert.Contains(t, result, "MERGE")
	assert.Contains(t, result, "ABSORB")
	assert.Contains(t, result, "SPLIT")
	assert.Contains(t, result, "REWORK")
	assert.Contains(t, result, "Re-verify")
	assert.Contains(t, result, "Guardrails")
}

func TestRenderGenerateTaskSummaryPrompt(t *testing.T) {
	result, err := RenderGenerateTaskSummaryPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	assert.Contains(t, result, ".snap/sessions/auth/tasks/TASKS.md")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")

	// Sections A through J.
	for _, section := range []string{"A.", "B.", "C.", "D.", "E.", "F.", "G.", "H.", "I.", "J."} {
		assert.Contains(t, result, section, "should contain section %s", section)
	}

	assert.Contains(t, result, "Guardrails")
	assert.Contains(t, result, "Completion")
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

func TestPreamblePrepended_CreateTasks(t *testing.T) {
	result, err := RenderCreateTasksPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "CreateTasks prompt should start with preamble")
}

func TestPreamblePrepended_GenerateTaskSummary(t *testing.T) {
	result, err := RenderGenerateTaskSummaryPrompt(".snap/sessions/auth/tasks")
	require.NoError(t, err)

	preamble, err := RenderPrinciplesPreamble()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, preamble), "GenerateTaskSummary prompt should start with preamble")
}

func TestRenderGenerateTaskFilePrompt(t *testing.T) {
	taskSpec := "| 3 | TASK3.md | Parallel batched task file generation | Epic 4 | TASK<N>.md files generated | Medium | M |"
	result, err := RenderGenerateTaskFilePrompt(".snap/sessions/auth/tasks", 3, taskSpec)
	require.NoError(t, err)

	// Contains tasksDir references.
	assert.Contains(t, result, ".snap/sessions/auth/tasks")

	// Contains task number.
	assert.Contains(t, result, "TASK3.md")

	// Contains task spec text.
	assert.Contains(t, result, "Parallel batched task file generation")

	// Contains 15-section format.
	assert.Contains(t, result, "0. Task Type and Placement")
	assert.Contains(t, result, "14. Follow-ups Unlocked")

	// Contains guardrails.
	assert.Contains(t, result, "Guardrails")

	// Contains preamble (via prependPreamble).
	assert.Contains(t, result, "simplest solution")

	// Contains codebase exploration context.
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "docs/context/")
}

func TestAllPrompts_ContainCodebaseExploration(t *testing.T) {
	tests := []struct {
		name   string
		render func() (string, error)
	}{
		{"PRD", func() (string, error) { return RenderPRDPrompt("tasks", "") }},
		{"Technology", func() (string, error) { return RenderTechnologyPrompt("tasks") }},
		{"Design", func() (string, error) { return RenderDesignPrompt("tasks") }},
		{"CreateTasks", func() (string, error) { return RenderCreateTasksPrompt("tasks") }},
		{"GenerateTaskSummary", func() (string, error) { return RenderGenerateTaskSummaryPrompt("tasks") }},
		{"GenerateTaskFile", func() (string, error) {
			return RenderGenerateTaskFilePrompt("tasks", 0, "spec")
		}},
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
