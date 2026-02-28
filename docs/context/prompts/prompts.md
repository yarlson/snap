# Prompts: LLM Prompt Templates

Package `internal/prompts` manages all embedded prompt templates used throughout the workflow. Each prompt is a Go text template that's embedded at compile time for use during workflow execution.

## Embedded Prompt Templates

### Implement

**File**: `implement.md`
**Purpose**: Generate implementation code for a task
**Parameters**: `PRDPath`, `TaskPath`, `TaskID`
**Function**: `Implement(ImplementData) (string, error)`
**Usage**: Step 1 of workflow iteration

### Ensure Completeness

**File**: `ensure_completeness.md`
**Purpose**: Verify task implementation covers all requirements
**Parameters**: `TaskPath`, `TaskID`
**Function**: `EnsureCompleteness(EnsureCompletenessData) (string, error)`
**Usage**: Step 2 of workflow iteration

### Lint and Test

**File**: `lint_and_test.md`
**Purpose**: Guide linting and testing validation
**Parameters**: None (plain string)
**Function**: `LintAndTest() string`
**Usage**: Step 3 of workflow iteration

### Code Review

**File**: `code_review.md`
**Purpose**: Perform automated code review with feedback
**Parameters**: None (plain string)
**Function**: `CodeReview() string`
**Usage**: Step 4 of workflow iteration

### Apply Fixes

**File**: `apply_fixes.md`
**Purpose**: Address code review feedback and fix issues
**Parameters**: None (plain string)
**Function**: `ApplyFixes() string`
**Usage**: Step 5 of workflow iteration

### Update Docs

**File**: `update_docs.md`
**Purpose**: Update user-facing documentation based on code changes
**Parameters**: None (plain string)
**Function**: `UpdateDocs() string`
**Usage**: Step 7 of workflow iteration

### Commit

**File**: `commit.md`
**Purpose**: Generate conventional commit messages
**Parameters**: None (plain string)
**Function**: `Commit() string`
**Usage**: Step 8 of workflow iteration

### Memory Update

**File**: `memory_update.md`
**Purpose**: Update `docs/context/` with current project state
**Parameters**: None (plain string)
**Function**: `MemoryUpdate() string`
**Usage**: Step 9 of workflow iteration

### Task Summary

**File**: `task_summary.md`
**Purpose**: Generate one-line task description (max 60 characters)
**Parameters**: `TaskContent` (task file content, truncated to 2000 bytes)
**Function**: `TaskSummary(TaskSummaryData) (string, error)`
**Usage**: Workflow runner displays task summary in header before iteration starts
**Output**: Single sentence, no jargon, plain language, max 60 characters

## Implementation Pattern

All templated prompts follow the same pattern:

1. Go file embeds markdown template using `//go:embed`
2. Prompt struct holds template parameters
3. Function parses template and executes with parameters
4. Result trimmed and returned as string

Non-templated prompts (LintAndTest, CodeReview, etc.) are returned as plain strings from their functions.

## Integration Points

- **Workflow Runner** — Uses all workflow prompts (Implement through Commit) in sequence per task
- **Task Summary** — Called during iteration setup to generate brief description
- **Step Runner** — Executes prompts via LLM and returns results
