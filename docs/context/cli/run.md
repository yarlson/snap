# CLI: Run Command with Session Support

## Overview

The `run` command orchestrates the task implementation workflow. It supports four invocation modes:

1. **Named session**: `snap run <name>` — runs a specific named session
2. **Auto-detection**: `snap run` (no args) — auto-selects if exactly one session exists, falls back to legacy layout
3. **Legacy mode**: Uses `docs/tasks/` directory (backward compatible)
4. **Ad hoc single-task mode**: `snap run --task-file <path>` — runs one task file directly, with no PRD or session required

## Implementation

**Files**:

- `cmd/run.go` — Run command definition and logic (includes pre-flight git remote detection and gh CLI validation)
- `cmd/run_test.go` — Unit tests for session resolution
- `cmd/run_e2e_test.go` — E2E tests for all workflows

## Command Signature

```bash
snap run [session] [flags]
snap run --task-file path/to/task.md [flags]
```

**Arguments**:

- `[session]` (optional) — Named session to run; if omitted, auto-detects or falls back to legacy layout

**Flags**:

- `--tasks-dir <path>` — Tasks directory (default: `docs/tasks`); ignored if session is provided
- `--prd <path>` — Custom PRD file path (default: `<tasks-dir>/PRD.md`)
- `--task-file <path>` — Run a single task file directly; incompatible with session arg, `--tasks-dir`, and `--prd`
- `--fresh` — Ignore existing state, start fresh
- `--show-state` — Display workflow progress and exit
- `--show-state --json` — Output raw state JSON
- `--no-color` — Disable ANSI colors (via `NO_COLOR` env var)

## Pre-flight Checks

Before starting the workflow, `run` performs:

1. **Git remote detection** — Detects the URL for `origin` remote (empty if not in a git repo or no remote configured)
2. **GitHub validation** — If remote is GitHub, validates that `gh` CLI is available in PATH (see [`provider.md`](provider.md#gh-cli-validation))
3. **Provider validation** — Validates selected LLM provider CLI is available (see [`provider.md`](provider.md))

## Session Resolution Logic

The `resolveRunConfig()` function determines the tasks directory, PRD path, task file path, display name, and state manager:

1. **If `--task-file` provided**: Resolve ad hoc single-task mode
2. **If session name provided**: Resolve named session (error if not found)
3. **If no sessions exist**: Fall back to legacy layout (docs/tasks/ or --tasks-dir flag)
4. **If exactly one session exists**: Auto-select it
5. **If multiple sessions exist**: Error with list of available sessions

### Ad Hoc Single-Task Resolution

**Function**: `resolveTaskFileRun(path)`

- Accepts any file path, relative or absolute, including paths outside the repository
- Resolves the path to an absolute path and validates that it is a regular file
- Uses the task file's parent directory as `tasksDir` for local context lookups
- Sets `prdPath` to empty string (no PRD required)
- Sets `taskFile` to the resolved absolute path
- Display name: absolute task file path (shown in startup summary)
- State manager: Ad hoc scoped state under `.snap/adhoc/<sha256(taskFile)>/state.json`

**Behavioral notes**:

- The task file can have any filename, not just `TASK<N>.md`
- The runner synthesizes a single task inventory from that file
- The 10-step workflow is unchanged
- If `PRD.md`, `TECHNOLOGY.md`, `DESIGN.md`, or `TASKS.md` exist near the task file, prompts may still read them opportunistically

### Named Session Resolution

**Function**: `resolveNamedSession(name)`

- Validates session exists via `session.Resolve(".", name)`
- Constructs tasks dir: `.snap/sessions/<name>/tasks/`
- PRD path: `.snap/sessions/<name>/tasks/PRD.md`
- Display name: `<name>` (shown in startup summary)
- State manager: Session-scoped (`state.NewManagerInDir(session.Dir(...))`)
- State file location: `.snap/sessions/<name>/state.json`

**Returns error** if session not found with hint: `snap new <name>`

### Legacy Fallback Resolution

**Function**: `resolveLegacyFallback(flagTasksDir, flagPRDPath)`

- Accepts tasks dir from `--tasks-dir` flag (default: `docs/tasks`)
- Checks if legacy layout exists:
  - Tasks directory is readable, OR
  - Legacy state.json exists at `.snap/state.json`
- If legacy layout found: Uses it with global legacy manager
- If no legacy layout: Auto-creates "default" session via `session.EnsureDefault()` and resolves as named session
- Display name: tasks directory path (legacy) or "default" (auto-created)
- State manager: Global legacy manager (legacy) or session-scoped manager (default session)
- State file location: `.snap/state.json` (legacy) or `.snap/sessions/default/state.json` (default session)

### Auto-Detection Sequence

**Function**: `resolveRunConfig(sessionName, flagTasksDir, flagPRDPath, flagTaskFile)`

1. If `--task-file` explicitly provided → resolve as ad hoc single-task mode
2. If session name explicitly provided → resolve as named session
3. List all sessions via `session.List(".")`
4. Switch based on count:
   - **0 sessions, legacy layout exists**: Fall back to legacy layout (docs/tasks/ or --tasks-dir)
   - **0 sessions, no legacy layout**: Auto-create "default" session and use it
   - **1 session**: Auto-select that session
   - **2+ sessions**: Error with list and hint to specify `snap run <name>`

**Error messages**:

- Named session not found: "Session 'auth' not found\n\nTo create it:\n snap new auth"
- Multiple sessions without name: Lists each session with task counts
- Legacy layout exists: Uses legacy layout if no sessions created yet

## State Manager Selection

Session-scoped vs. legacy state is determined by what's resolved:

- **Ad hoc single-task mode**: Uses `state.NewManagerInDir(".snap/adhoc/<hash>")` → state.json at `.snap/adhoc/<hash>/state.json`
- **Named session**: Uses `state.NewManagerInDir(sessionDir)` → state.json at `.snap/sessions/<name>/state.json`
- **Auto-detected single session**: Same as named session
- **Legacy layout**: Uses `state.NewManager()` → state.json at `.snap/state.json`

State managers are independent: ad hoc task state does not interfere with session or legacy state.

### Show-State Session Resolution

**Function**: `resolveStateManager(sessionName, taskFilePath)` (used by `--show-state` flag)

- If `--task-file` provided: Resolves to that task file's ad hoc state manager
- If session name provided: Resolves as named session and returns its state manager
- If no session name provided: Auto-detects via `session.List()`:
  - **1 session**: Returns its state manager
  - **0 sessions**:
    - If legacy layout exists (state.json or `docs/tasks` directory): Returns legacy state manager
    - If no legacy layout: Auto-creates "default" session and returns its state manager
  - **2+ sessions**: Returns legacy manager (does not auto-select, requires explicit name from command line)

This enables fresh projects to use `snap run --show-state` without requiring session creation first.

## Show State with Sessions

`snap run [session] --show-state` and `snap run --task-file <path> --show-state` display state for the resolved workflow target:

1. Resolve ad hoc task, session, or legacy layout (same logic as normal run)
2. Load state from ad hoc, session-scoped, or legacy manager
3. Format human-readable summary or JSON output
   - No state file → displays "No state file exists" with `ui.Info()` formatting

**Without session argument**: Auto-detects (same sequence as run)

## Display Name

The `displayName` field (in `runConfig`) is used in the startup summary:

- Ad hoc single-task mode: Shows resolved task file path
- Named session: Shows session name (e.g., `snap: auth |`)
- Auto-detected session: Shows session name (e.g., `snap: api |`)
- Legacy layout: Shows tasks directory (e.g., `snap: docs/tasks |`)

## Output Formatting

- **PRD file warning** — When PRD file doesn't exist, displays warning message via `ui.Info()` (dimmed text)
- **No state file message** — When `--show-state` is used but no state exists, displays "No state file exists" via `ui.Info()`

In ad hoc single-task mode, there is no PRD warning because `PRDPath` is intentionally empty.

## Path Validation

Security validation is applied only to user-supplied paths:

- `--task-file` paths allow locations outside the repository; validation checks newline safety, resolves absolute path, and requires a regular file
- Named session paths are constructed from validated session names
- Legacy paths may come from `--tasks-dir` flag → validated if present
- Auto-detected paths are constructed safely → no validation needed

## Testing

**Unit tests** (`cmd/run_test.go`):

- `TestResolveRunConfig_NamedSession_Exists` — Named session resolution
- `TestResolveRunConfig_NamedSession_NotFound` — Error handling for missing session
- `TestResolveRunConfig_NoName_ZeroSessions_NoLegacy` — Auto-creates "default" session when no sessions/legacy
- `TestResolveRunConfig_NoName_ZeroSessions_LegacyTaskFiles` — Legacy fallback with tasks (prevents auto-create)
- `TestResolveRunConfig_NoName_ZeroSessions_LegacyStateFile` — Legacy fallback with state.json (prevents auto-create)
- `TestResolveRunConfig_NoName_OneSession` — Auto-detection of single session
- `TestResolveRunConfig_NoName_MultipleSessions` — Error with multiple sessions
- `TestResolveRunConfig_TaskFile_AnywhereOnDisk` — Ad hoc task-file resolution outside repo
- `TestResolveRunConfig_SessionStateManager_IndependentFromLegacy` — Session/legacy isolation
- `TestResolveRunConfig_SessionWhilePlanning` — Sessions during planning phase
- `TestResolveRunConfig_FreshWithSession` — --fresh flag resets session state
- `TestResolveRunConfig_ShowStateWithSession` — State inspection on session
- `TestResolveStateManager_TaskFile_UsesIsolatedAdhocState` — Ad hoc state manager isolation
- `TestResolveStateManager_ZeroSessions_CreatesDefault` — Auto-create default for show-state
- `TestResolveStateManager_ZeroSessions_LegacyTaskFiles` — Legacy fallback prevents auto-create for show-state

**E2E tests** (`cmd/run_e2e_test.go`):

- `TestE2E_CUJ1_CreateAndRunSession` — Session creation and startup summary
- `TestE2E_CUJ3_SwitchBetweenSessions` — Running multiple sessions with independent state
- `TestE2E_CUJ4_AutoDetectSingleSession` — Auto-detection when one session exists
- `TestE2E_CUJ5_LegacyFallback` — Legacy layout without sessions
- `TestE2E_RunMultipleSessionsError` — Error when multiple sessions and no name given
- `TestE2E_RunFreshProject` — Auto-creates "default" session when no sessions/legacy
- `TestE2E_ShowStateFreshProject` — Show-state auto-creates "default" session on fresh project
- `TestE2E_RunNonexistentSession` — Error for nonexistent session
- `TestE2E_ShowStateWithSession` — Show-state with session name
- `TestE2E_FreshWithSessionState` — --fresh flag behavior
- `TestE2E_ResumeAcrossSessionRuns` — State persistence across runs

## Design Notes

- **Backward compatibility**: Bare `snap` command (via defaultCmd in root) still routes to run logic
- **Ad hoc isolation**: Each `--task-file` path gets its own independent state directory under `.snap/adhoc/`
- **Session isolation**: Each session has completely independent state (no cross-contamination)
- **Auto-detection usefulness**: Most useful in single-project repos with one session; multi-session repos require explicit session name
- **Error clarity**: Messages always provide corrective actions (snap new, snap run <name>)
- **Display consistency**: Startup summary shows what's running (task file, session name, or tasks dir)
