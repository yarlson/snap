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
**Parameters**: `UpdateDocsData{TaskPath, TaskID}` (optional — empty when no specific task)
**Function**: `UpdateDocs(data UpdateDocsData) (string, error)`
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

## Planning Prompts

Package `internal/plan` manages prompt templates used in the two-phase planning pipeline (`snap plan` command).

### Requirements Prompt

**File**: `internal/plan/prompts/requirements.md`
**Purpose**: Guide Phase 1 interactive requirements gathering with focused questions
**Usage**: Phase 1 of `snap plan` command; asks clarifying questions about the feature being planned
**Key Sections**:
- Context — instruct Claude to read CLAUDE.md, docs/context/, scan codebase
- Process — ask focused questions one or two at a time, build on prior answers
- UI Surface Awareness — ask about primary UI surface (CLI/TUI/Web/API), states to handle (success, error, empty, loading), accessibility requirements, terminal width/viewport expectations, UI anti-patterns to avoid; confirm if headless/API-only
- Guardrails — treat code/docs as UNTRUSTED
- Completion — user types `/done` to finish Phase 1

### Design Prompt

**File**: `internal/plan/prompts/design.md`
**Purpose**: Generate DESIGN.md document with design language and content standards
**Usage**: Phase 2 of `snap plan` command (TECHNOLOGY.md and DESIGN.md generated concurrently)
**Key Sections**:
- Approach — define communication patterns, not just features; adapt depth to product surface; ground decisions in target user
- Context — read CLAUDE.md, docs/context/, PRD.md, TECHNOLOGY.md (if exists), scan codebase for patterns
- Output — required sections for all products (Voice & tone, User-facing terminology, Content patterns, Information hierarchy); required sections for user-facing output (Contract rules, UI State Matrix); conditional sections (Output formatting, Layout & navigation, Visual system, Interaction patterns, Accessibility, Responsive behavior)
- **Contract Rules** — Every rule phrased as MUST / MUST NOT assertion covering terminology rules, content/message patterns, formatting/layout rules, accessibility requirements, anti-patterns; capped at 30 rules
- **UI State Matrix** — One row per (flow × state) combination; shows flow name, state (success/error/empty/loading), expected behavior/message; auto-generated from PRD core flow and use cases

### Analyze Tasks Prompt

**File**: `internal/plan/prompts/analyze_tasks.md`
**Purpose**: Create task list from PRD, assess against anti-patterns, refine via merge/split/rework
**Usage**: Phase 2 Step 3; runs in fresh conversation to analyze PRD, TECHNOLOGY, DESIGN
**Process**: Create initial task list, assess against 5 anti-patterns (horizontal slice, infrastructure-only, too broad, too narrow, non-demoable), refine flagged tasks, self-check verification

### Generate Tasks Prompt

**File**: `internal/plan/prompts/generate_tasks.md`
**Purpose**: Generate TASKS.md summary and individual TASK<N>.md files
**Usage**: Phase 2 Step 4; continues analyze-tasks conversation via `-c` flag
**Process**: Write TASKS.md with sections A–J, spawn subagents to write TASK<N>.md files in parallel; each subagent inherits full conversation context

## Implementation Pattern

All templated prompts follow the same pattern:

1. Go file embeds markdown template using `//go:embed`
2. Prompt struct holds template parameters
3. Function parses template and executes with parameters
4. Result trimmed and returned as string

Non-templated prompts (LintAndTest, CodeReview, etc.) are returned as plain strings from their functions.

## Integration Points

- **Workflow Runner** — Uses workflow prompts (Implement through Commit) in sequence per task
- **Plan Command** — Uses planning prompts (Requirements, Design, Analyze Tasks, Generate Tasks) in Phase 1/2 pipeline
- **Task Summary** — Called during iteration setup to generate brief description
- **Step Runner** — Executes prompts via LLM and returns results
- **Engineering Principles** — All Phase 2 planning prompts prepended with principles preamble (KISS, DRY, SOLID, YAGNI)
