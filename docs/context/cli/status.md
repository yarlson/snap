# CLI: Status Command

Display detailed progress for a named session, showing task list with completion state and current workflow progress.

## Command Signature

```bash
snap status [session]
```

## Session Resolution

- **Explicit session**: `snap status auth-system` — shows status for specified session
- **Auto-detect**: `snap status` — uses single existing session if exactly one exists; error with session list if multiple or none exist
- **Error cases**:
  - No sessions: "no sessions found\n\nTo create a session:\n snap new <name>"
  - Multiple sessions: Lists available sessions with task counts, prompts user to specify one

Session resolution logic:

1. If session name provided as arg → use it
2. If no args → list all sessions
   - 0 sessions: error
   - 1 session: auto-use it
   - 2+ sessions: error with list and prompt

## Output Format

Displays formatted output using `internal/ui` functions:

- Session name and tasks directory path (via `ui.KeyValue()`)
- List of all tasks with completion state (via `ui.TaskDone()`, `ui.TaskActive()`, `ui.TaskPending()`):
  - `[x]` — Task completed (success color + bold, text dimmed)
  - `[~]` — Task in progress (secondary color + bold, shows current step and total steps in dimmed suffix)
  - `[ ]` — Task not started (entire line dimmed)
- Section header "Tasks:" (via `ui.Info()`)
- Summary counts message (via `ui.Info()`)

Example output:

```
Session: auth-system
Path:    .snap/sessions/auth-system/tasks

Tasks:
  [x] TASK1
  [~] TASK2 (step 5/10: Apply fixes)
  [ ] TASK3

2 tasks remaining, 1 complete
```

## Step Display Format

For in-progress tasks, displays:

- Current step number and total steps
- Step name from workflow step definitions

Format: `(step N/TOTAL: StepName)`

## Status Derivation

Task status derived from `internal/session/Status()`:

- **Completed**: Task ID in `completed_task_ids` from state.json
- **Active**: Task ID matches `current_task_id` and `current_step > 0`
- **Not started**: All other tasks

## Integration Points

- **session package**: `Status()` returns structured session status with task details
- **workflow package**: `StepName()` returns human-readable step names
- **internal/session/session.go**: `Status()` function that aggregates session metadata and state
- Task file discovery via session tasks directory

## Implementation

Located in `cmd/status.go`:

- `statusCmd` — Cobra command definition
- `statusRun()` — Entry point that resolves session and displays status
- `resolveStatusSession()` — Session name resolution logic
- `formatMultipleStatusSessionsError()` — Error message formatting for multiple sessions

## Testing

- Unit tests in `cmd/status_test.go`:
  - Session resolution (explicit, auto-detect, errors)
  - Output formatting with various task states
  - Task count accuracy
  - Step progress display

## Design Notes

- **Auto-detection**: Enables single-session workflows where users don't need to specify session name
- **Clear status**: Task markers ([ ], [~], [x]) provide at-a-glance progress visibility
- **Step context**: Current step display helps users understand workflow position
- **Error messages**: List available sessions to help users disambiguate multiple-session scenarios
