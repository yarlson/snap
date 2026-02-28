# CLI: Session Management

## Overview

Session management enables creation and organization of named project workspaces within a single repository. Each session maintains isolated task directories, independent state tracking, and separate workflow execution. Useful for managing multiple features or projects in parallel.

## Implementation

**Files**:

- `cmd/new.go` — Create session subcommand
- `cmd/delete.go` — Delete session subcommand (fully implemented)
- `cmd/delete_test.go` — Delete command integration tests
- `cmd/list.go` — List sessions subcommand (fully implemented)
- `cmd/list_test.go` — List command integration tests
- `cmd/plan.go` — Plan session subcommand (two-phase planning with resumption support)
- `cmd/plan_e2e_test.go` — End-to-end tests for plan command
- `cmd/status.go` — Status session subcommand (show session progress and task state)
- `cmd/status_test.go` — Status command tests
- `cmd/new_test.go` — Comprehensive E2E and integration tests for session commands
- `internal/session/session.go` — Session management logic (create, validate, delete, list, status, plan history tracking, artifact detection, artifact cleanup)
- `internal/session/session_test.go` — Session unit tests

### Session Directory Structure

When a session is created via `snap new <name>`:

```
.snap/
├── sessions/
│   └── <session-name>/
│       ├── tasks/
│       │   ├── PRD.md
│       │   ├── TASK1.md
│       │   └── ...
│       └── state.json (auto-created after first workflow run)
├── .gitignore (contains "sessions" or "*" to ignore session directories)
└── state.json (global default workflow state)
```

### New Session Command

Cobra command definition:

```go
var newCmd = &cobra.Command{
    Use:           "new <name>",
    Short:         "Create a new named session",
    Args:          cobra.ExactArgs(1),
    SilenceUsage:  true,
    SilenceErrors: true,
    RunE:          newRun,
}
```

**newRun function** (`cmd/new.go`):

1. Validates session name (alphanumeric, hyphens, underscores only)
2. Calls `session.Create(".", name)` from `internal/session` package
3. Creates directory structure at `.snap/sessions/<name>/tasks/`
4. Outputs success message with next steps:
   - `snap plan <name>` — Plan tasks for the session
   - `snap run <name>` — Run the session workflow
5. Returns error if session already exists

**Session validation** (`internal/session/session.go`):

- Name must match pattern: `^[a-zA-Z0-9_-]+$` (alphanumeric, hyphens, underscores)
- Session directory must not already exist
- Creates `.snap/sessions/<name>/tasks/` directory structure
- Initializes with template PRD.md and TASK1.md files

**EnsureDefault() function** (`internal/session/session.go`):

- Idempotent function that creates "default" session if it doesn't exist
- Returns immediately if "default" already exists (no error, no re-creation)
- Creates directory structure and initializes files same as `Create()`
- Used by multiple commands to auto-create "default" session on fresh projects (plan, status, run, show-state)

### Delete Session Command

Cobra command definition:

```go
var deleteCmd = &cobra.Command{
    Use:           "delete <name>",
    Short:         "Delete a session and all its files",
    Args:          cobra.ExactArgs(1),
    RunE:          deleteRun,
}
```

**deleteRun function** (`cmd/delete.go`):

1. Prompts for confirmation: `"Delete session '<name>' and all its files? (y/N) "`
2. Accepts `y` or `Y` as confirmation (case-insensitive)
3. Skips confirmation when `--force` flag is used
4. Calls `session.Delete(".", name)` to remove directory
5. Returns error if session not found

**Flags**:

- `--force` — Skip confirmation prompt and delete immediately

### List Sessions Command

Cobra command definition:

```go
var listCmd = &cobra.Command{
    Use:           "list",
    Short:         "List all sessions",
    RunE:          listRun,
}
```

**listRun function** (`cmd/list.go`):

1. Calls `session.List(".")` to retrieve all sessions
2. If no sessions exist: displays empty state via `ui.Info()` with help text
3. If sessions exist: calculates column widths for alignment
4. Displays formatted table with columns: Name (bold), Tasks (dim), Status (normal)
5. Uses `ui.ResolveStyle()` to apply styling codes directly to output

**Output format** (when sessions exist):

```
  auth       2 tasks (1 done)  paused at step 5
  api        0 tasks           planning
  cleanup    1 task            complete
```

**Empty state output**:

```
No sessions found

To create a session:
  snap new <name>
```

Sessions are sorted alphabetically by name. Session names use bold styling, task counts use dim styling.

### Session Status Values

Status derived by `deriveStatus()` in `internal/session/session.go`:

- `"planning"` — Session has .plan-started marker but no completed tasks
- `"complete"` — All tasks completed (completedCount >= taskCount)
- `"paused at step N"` — Workflow interrupted at specific step (N = current_step)
- `"idle"` — Session with tasks but none started yet
- `"no tasks"` — Session has no task files and no planning marker
- `"unknown"` — State file corrupted or unreadable

### Session Info Retrieval

**session.Info struct** (`internal/session/session.go`):

```go
type Info struct {
    Name           string  // Session name
    TaskCount      int     // Total TASK*.md files in tasks/
    CompletedCount int     // Completed tasks from state.json
    Status         string  // Derived status
}
```

**List() function** (`internal/session/session.go`):

1. Scans `.snap/sessions/` directory
2. For each session: counts TASK\*.md files
3. Reads state.json (if present) for CompletedTaskIDs
4. Checks for .plan-started marker
5. Derives status from task counts and workflow state
6. Returns sorted slice (alphabetical by name)
7. Returns nil if `.snap/sessions/` doesn't exist

**Delete() function** (`internal/session/session.go`):

1. Validates session name
2. Checks session exists
3. Removes directory via `os.RemoveAll()`

### Plan Session Command

Cobra command definition:

```go
var planCmd = &cobra.Command{
    Use:           "plan [session]",
    Short:         "Plan tasks for a session",
    Args:          cobra.MaximumNArgs(1),
    SilenceUsage:  true,
    SilenceErrors: true,
    RunE:          planRun,
}
```

**planRun function** (`cmd/plan.go`):

1. Resolves session name (explicit arg or auto-detect)
   - If no sessions exist, auto-creates "default" session via `session.EnsureDefault()`
2. Detects if prior planning session exists via `session.HasPlanHistory()`
3. Creates planner with resumption support if applicable
4. Executes two-phase planning pipeline:
   - Phase 1: Interactive requirements gathering (or resume prior conversation)
   - Phase 2: Autonomous document generation (PRD, TECHNOLOGY, DESIGN, TASK files)
5. Writes `.plan-started` marker after first successful message via callback

**Session resolution**: Same logic as list/delete/status commands, with auto-creation of "default" when zero sessions exist

### Status Session Command

Cobra command definition:

```go
var statusCmd = &cobra.Command{
    Use:           "status [session]",
    Short:         "Show detailed status for a session",
    Args:          cobra.MaximumNArgs(1),
    SilenceUsage:  true,
    SilenceErrors: true,
    RunE:          statusRun,
}
```

**statusRun function** (`cmd/status.go`):

1. Resolves session name (explicit arg or auto-detect)
   - If no sessions exist and no legacy layout: auto-creates "default" session via `session.EnsureDefault()`
   - If legacy layout exists with no sessions: returns error directing user to create a session
2. Calls `session.Status()` to retrieve session details
3. Displays session name, path, and task list
4. Shows completion state for each task with current step if in progress
5. Displays summary counts (remaining and completed tasks)

**Output includes**:

- Task markers: `[x]` complete, `[~]` in progress, `[ ]` not started
- For in-progress tasks: current step number and step name
- Total task count and completion status

## Usage

**Create a new session:**

```bash
snap new my-project
```

**Add task files to the session:**

```bash
# Session directory created at: .snap/sessions/my-project/tasks/
# Add files: TASK1.md, TASK2.md, etc.
```

**Run the session workflow:**

```bash
snap run my-project
```

## Testing

**E2E tests** (`cmd/new_test.go`):

- `TestNewE2E_CreatesSession()` — Builds snap binary, runs `snap new test-session`, verifies output and directory structure
- `TestNewE2E_DuplicateSession()` — Attempts to create session with same name twice, verifies error on second attempt
- `TestNewE2E_InvalidName()` — Tests invalid session names (spaces, special chars), verifies error messages mention valid characters
- `TestNewE2E_NoNameArg()` — Tests `snap new` without name argument, verifies Cobra arg validation fails
- `TestRunE2E_SubcommandWorks()` — Verifies `snap run` subcommand routes to run logic
- `TestNewE2E_BareSnapStillWorks()` — Verifies bare `snap` command still invokes run logic (backward compat)
- `TestStubE2E_PlanNotImplemented()` — Verifies `snap plan` returns "not implemented"
- `TestGitignore_CoversSessionsDir()` — Verifies `.snap/.gitignore` contains pattern covering sessions directory
- `TestPlanE2E_*()` — Plan command E2E tests (session resolution, document generation, resumption)
- `TestStatusE2E_*()` — Status command E2E tests (session resolution, output formatting)

**Delete command tests** (`cmd/delete_test.go`):

- `TestDelete_WithForceFlag()` — Verifies force flag skips confirmation prompt
- `TestDelete_WithConfirmation()` — Verifies confirmation prompt blocks deletion on non-yes answer
- `TestDelete_NotFound()` — Verifies error when deleting non-existent session
- `TestDelete_InvalidName()` — Verifies error for invalid session names
- `TestDelete_Confirmation()` — Verifies default confirmation prompt is displayed and works correctly

**List command tests** (`cmd/list_test.go`):

- `TestList_EmptyOutput()` — Verifies "No sessions found" message when no sessions exist
- `TestList_SingleSession()` — Verifies correct output format with single session
- `TestList_MultipleSessionsAlphabetical()` — Verifies sessions are sorted alphabetically
- `TestList_SessionWithTasksAndProgress()` — Verifies task count and completion status display
- `TestList_SessionWithCorruptedState()` — Verifies "unknown" status for corrupted state.json
- `TestList_SessionWithPlanMarker()` — Verifies "planning" status when .plan-started exists

**Plan command tests** (`cmd/plan_e2e_test.go`):

- Session resolution (explicit, auto-detect, errors)
- Two-phase planning pipeline execution
- Document generation and placement
- Plan resumption behavior
- Signal interruption handling

**Status command tests** (`cmd/status_test.go`):

- Session resolution (explicit, auto-detect, errors)
- Task state display accuracy
- Step progress formatting
- Task count summary

**Session unit tests** (`internal/session/session_test.go`):

- Tests for `ValidateName()` — Valid/invalid name patterns
- Tests for `Create()` — Session directory creation
- Tests for `List()` — Session discovery and status derivation
- Tests for `Delete()` — Session removal
- Tests for `Status()` — Session status retrieval with task details
- Tests for `deriveStatus()` — Status calculation from task counts and state
- Tests for `HasPlanHistory()` — Plan marker detection
- Tests for `MarkPlanStarted()` — Marker file creation
- Tests for `Exists()` — Session existence checking
- Tests for `HasArtifacts()` — Planning artifact detection (TASK*.md, PRD.md, TECHNOLOGY.md, DESIGN.md)
- Tests for `CleanSession()` — Complete session cleanup (tasks directory and state files)

**Integration tests**:

- `TestNew_CreatesSessionDirectory()` — Verifies session directory creation and output messages
- `TestNew_DuplicateSessionErrors()` — Verifies error handling for duplicate session names

## Command Refactoring

The main run logic was refactored from `cmd/root.go` to `cmd/run.go` as an explicit subcommand:

- **`snap`** (bare command) — Still works for backward compatibility, routes to run logic via DefaultCmd in root
- **`snap run`** — Explicit subcommand version of the same workflow logic
- Both execute identical workflow (read tasks, iterate, implement, validate, review, commit)

## Design Notes

- **Session isolation**: Each session has separate task directory and state file, enabling parallel independent workflows
- **Backward compatibility**: Bare `snap` command still works without subcommand; `snap run` is explicit alternative
- **Name validation**: Sessions must use alphanumeric names with hyphens/underscores (1-64 chars); prevents filesystem issues
- **Gitignore coverage**: `.snap/sessions/` is git-ignored to prevent session state/artifacts from being committed
- **Session listing**: Reads session metadata (task counts, completion status) from directories and state.json; status derived from workflow state
- **Delete confirmation**: Default interactive prompt with --force flag to skip; prevents accidental deletion
- **Status derivation**: Combines multiple signals (state.json, task file count, .plan-started marker) to compute session status
- **Plan resumption**: Detects prior planning via marker file; uses -c flag for conversation continuity when resuming
- **Marker timing**: .plan-started marker created after first successful executor call, enabling clean plan startup tracking
