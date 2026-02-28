# CLI: Show State & Inspection

## Overview

The `--show-state` flag displays workflow progress at any time, with support for both human-readable and JSON output formats. Works with explicit session names or auto-detects session/legacy layout.

## Flags

**`--show-state`** — Display current workflow state and exit

- Loads state from session-scoped (`.snap/sessions/<name>/state.json`) or legacy (`.snap/state.json`) location
- Auto-detects session/legacy layout via `resolveStateManager()` if none named explicitly:
  - Single existing session: uses it
  - Multiple sessions: returns error (relies on explicit session name from `snap run <name> --show-state`)
  - No sessions, legacy exists (state.json or `docs/tasks` directory): uses legacy manager
  - No sessions, no legacy: auto-creates "default" session via `session.EnsureDefault()`
- Displays human-readable summary by default
- Can output raw JSON with `--json` flag
- Exits immediately without running workflow

**`--json`** (only with `--show-state`) — Output raw JSON instead of summary

- Ignored if `--show-state` is not specified
- Outputs complete state object with all fields
- Useful for script/tool integration

## Output Formats

**Human-Readable Summary** (default):

- Single-line, terminal-friendly format
- Shows current task and progress (if active)
- Displays completed task count
- Example: `TASK2 in progress — step 5/10: Apply fixes — 1 task completed`
- When idle: `No active task — 2 tasks completed`

**JSON Format** (with `--json`):

- Complete state object with structure:
  - `tasks_dir` — Path to tasks directory
  - `current_task_id` — Current task ID (null if idle)
  - `current_task_file` — Current task filename
  - `current_step` — Current step number (1-indexed)
  - `total_steps` — Total workflow steps (10)
  - `completed_task_ids` — Array of completed task IDs

## Implementation

**State.Summary()** method (`internal/state/types.go`):

- Accepts a `stepName` function to map step numbers to display names
- Returns formatted string based on whether a task is active
- Singular/plural logic for "task" vs "tasks"

**Step Names** (`internal/workflow/steps.go`):

- `stepNames` array maps 1-indexed step numbers to display names
- Names match the 10-step iteration workflow (Implement, Ensure Completeness, Lint & Test, Code Review, Apply Fixes, Verify Fixes, Update Docs, Commit Code, Update Memory, Commit Memory)
- `StepName()` function returns the display name for a given step number

**Root Command Handler** (`cmd/root.go`):

- New `jsonOutput` flag stored in module-level variable
- `handleShowState()` checks `jsonOutput` flag:
  - If true: calls `json.MarshalIndent()` and outputs raw JSON
  - If false: calls `workflowState.Summary()` and outputs human-readable summary

## Use Cases

**User inspection** — Check workflow progress without running it
**Script integration** — Extract state data via JSON output
**CI/CD debugging** — Validate state before resuming or restarting
