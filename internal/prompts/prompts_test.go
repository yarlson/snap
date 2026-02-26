package prompts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/prompts"
)

func TestImplement_SpecificTask(t *testing.T) {
	data := prompts.ImplementData{
		PRDPath:  "docs/PRD.md",
		TaskPath: "docs/tasks/TASK1.md",
		TaskID:   "TASK1",
	}

	result, err := prompts.Implement(data)
	require.NoError(t, err)

	assert.Contains(t, result, "docs/PRD.md")
	assert.Contains(t, result, "docs/tasks/TASK1.md")
	assert.Contains(t, result, "Implement TASK1")
	assert.Contains(t, result, "TECHNOLOGY.md")
	assert.Contains(t, result, "TASKS.md")
	assert.Contains(t, result, "memory-map.md")
	assert.Contains(t, result, "Do not update the memory vault")
}

func TestImplement_AutoSelect(t *testing.T) {
	data := prompts.ImplementData{
		PRDPath: "docs/PRD.md",
	}

	result, err := prompts.Implement(data)
	require.NoError(t, err)

	assert.Contains(t, result, "docs/PRD.md")
	assert.Contains(t, result, "next unimplemented task")
	assert.NotContains(t, result, "docs/tasks/")
	assert.Contains(t, result, "Do not update the memory vault")
}

func TestImplement_QualityGuardrails(t *testing.T) {
	data := prompts.ImplementData{PRDPath: "PRD.md"}
	result, err := prompts.Implement(data)
	require.NoError(t, err)

	// Security guardrails
	assert.Contains(t, result, "parameterized queries")
	assert.Contains(t, result, "hardcoded secrets")
	assert.Contains(t, result, "Validate")

	// Reliability guardrails
	assert.Contains(t, result, "N+1")
	assert.Contains(t, result, "deterministically")
	assert.Contains(t, result, "edge cases")

	// TDD guardrails
	assert.Contains(t, result, "failing E2E or integration test")
	assert.Contains(t, result, "minimal code to pass")

	// Architecture guardrails
	assert.Contains(t, result, "business logic")
}

func TestImplement_ScopeRules(t *testing.T) {
	data := prompts.ImplementData{PRDPath: "PRD.md"}
	result, err := prompts.Implement(data)
	require.NoError(t, err)

	assert.Contains(t, result, "ONLY what the task defines")
	assert.Contains(t, result, "Do not refactor")
	assert.Contains(t, result, "acceptance criteria")
}

func TestImplement_DocsRuleDelegatedToStep(t *testing.T) {
	data := prompts.ImplementData{PRDPath: "PRD.md"}
	result, err := prompts.Implement(data)
	require.NoError(t, err)

	// Rule #9 about updating docs was removed — docs are handled by the dedicated "Update docs" step.
	assert.NotContains(t, result, "update the relevant docs")
	assert.NotContains(t, result, "CLI help text, usage examples, API docs")
}

func TestImplement_ContextLoading(t *testing.T) {
	data := prompts.ImplementData{PRDPath: "PRD.md"}
	result, err := prompts.Implement(data)
	require.NoError(t, err)

	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "AGENTS.md")
	assert.Contains(t, result, "existing source code")
}

func TestImplement_NoTrailingWhitespace(t *testing.T) {
	data := prompts.ImplementData{PRDPath: "PRD.md"}
	result, err := prompts.Implement(data)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestEnsureCompleteness(t *testing.T) {
	data := prompts.EnsureCompletenessData{
		TaskPath: "docs/tasks/TASK1.md",
		TaskID:   "TASK1",
	}
	result, err := prompts.EnsureCompleteness(data)
	require.NoError(t, err)

	assert.Contains(t, result, "fully implemented")
	assert.Contains(t, result, "docs/tasks/TASK1.md")
	assert.Contains(t, result, "TASK1")
	assert.Contains(t, result, "not verified")
	assert.Contains(t, result, "acceptance criterion")
	assert.Contains(t, result, "## Context")
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "memory-map.md")
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "Do not refactor")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestLintAndTest(t *testing.T) {
	result := prompts.LintAndTest()

	assert.Contains(t, result, "AGENTS.md")
	assert.Contains(t, result, "linters")
	assert.Contains(t, result, "tests")
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "zero issues")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestCodeReview(t *testing.T) {
	result := prompts.CodeReview()

	// Context loading (entry prompt)
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "AGENTS.md")
	assert.Contains(t, result, "memory-map.md")

	// Scope
	assert.Contains(t, result, "read-only")

	// Key review markers
	assert.Contains(t, result, "merge-base")
	assert.Contains(t, result, "Security Scan")
	assert.Contains(t, result, "CRITICAL")
	assert.Contains(t, result, "BLOCK")
	assert.Contains(t, result, "APPROVE")
	assert.Contains(t, result, "security")
	assert.Contains(t, result, "Finding")
	assert.NotContains(t, result, "Use the code-review skill")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestApplyFixes(t *testing.T) {
	result := prompts.ApplyFixes()

	assert.Contains(t, result, "Fix")
	assert.Contains(t, result, "issues")
	assert.Contains(t, result, "## Process")
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "CRITICAL")
	assert.Contains(t, result, "do not refactor")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestCommit(t *testing.T) {
	result := prompts.Commit()

	assert.Contains(t, result, "commit")
	assert.Contains(t, result, "conventional commit")
	assert.Contains(t, result, "co-author")
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "do not modify any code")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestUpdateDocs(t *testing.T) {
	result := prompts.UpdateDocs()

	// Context loading
	assert.Contains(t, result, "CLAUDE.md")
	assert.Contains(t, result, "AGENTS.md")
	assert.Contains(t, result, ".memory/")

	// Core behavior: diff-based doc update
	assert.Contains(t, result, "merge-base")
	assert.Contains(t, result, "README.md")
	assert.Contains(t, result, "user-facing")

	// Scope
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "Do not modify source code")

	// Skip case
	assert.Contains(t, result, "nothing")

	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestMemoryUpdate(t *testing.T) {
	result := prompts.MemoryUpdate()

	// Scope section
	assert.Contains(t, result, "## Scope")
	assert.Contains(t, result, "Do not modify any source code")

	// Key vault markers
	assert.Contains(t, result, ".memory/")
	assert.Contains(t, result, "summary.md")
	assert.Contains(t, result, "terminology.md")
	assert.Contains(t, result, "practices.md")
	assert.Contains(t, result, "memory-map.md")
	assert.Contains(t, result, "code is truth")

	// No persona filler or delegation text
	assert.NotContains(t, result, "You are the Memory Vault curator")
	assert.NotContains(t, result, "Update the memory vault.")
	assert.Equal(t, strings.TrimSpace(result), result)
}
