# CLI: Run Command with Session Support

## Overview

The `run` command orchestrates the task implementation workflow. It supports three invocation modes:

1. **Named session**: `snap run <name>` — runs a specific named session
2. **Auto-detection**: `snap run` (no args) — auto-selects if exactly one session exists, falls back to legacy layout
3. **Legacy mode**: Uses `docs/tasks/` directory (backward compatible)

## Implementation

**Files**:

- `cmd/run.go` — Run command definition and logic
- `cmd/run_test.go` — Unit tests for session resolution
- `cmd/run_e2e_test.go` — E2E tests for all workflows

## Command Signature

```
snap run [session] [flags]
```

**Arguments**:

- `[session]` (optional) — Named session to run; if omitted, auto-detects or falls back to legacy layout

**Flags**:

- `--tasks-dir <path>` — Tasks directory (default: `docs/tasks`); ignored if session is provided
- `--prd <path>` — Custom PRD file path (default: `<tasks-dir>/PRD.md`)
- `--fresh` — Ignore existing state, start fresh
- `--show-state` — Display workflow progress and exit
- `--show-state --json` — Output raw state JSON
- `--no-color` — Disable ANSI colors (via `NO_COLOR` env var)

## Session Resolution Logic

The `resolveRunConfig()` function determines the tasks directory, PRD path, display name, and state manager:

1. **If session name provided**: Resolve named session (error if not found)
2. **If no sessions exist**: Fall back to legacy layout (docs/tasks/ or --tasks-dir flag)
3. **If exactly one session exists**: Auto-select it
4. **If multiple sessions exist**: Error with list of available sessions

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

**Function**: `resolveRunConfig(sessionName, flagTasksDir, flagPRDPath)`

1. If session name explicitly provided → resolve as named session
2. List all sessions via `session.List(".")`
3. Switch based on count:
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

- **Named session**: Uses `state.NewManagerInDir(sessionDir)` → state.json at `.snap/sessions/<name>/state.json`
- **Auto-detected single session**: Same as named session
- **Legacy layout**: Uses `state.NewManager()` → state.json at `.snap/state.json`

State managers are independent: session state does not interfere with legacy state.

### Show-State Session Resolution

**Function**: `resolveStateManager(sessionName)` (used by `--show-state` flag)

- If session name provided: Resolves as named session and returns its state manager
- If no session name provided: Auto-detects via `session.List()`:
  - **1 session**: Returns its state manager
  - **0 sessions**:
    - If legacy layout exists (state.json or `docs/tasks` directory): Returns legacy state manager
    - If no legacy layout: Auto-creates "default" session and returns its state manager
  - **2+ sessions**: Returns legacy manager (does not auto-select, requires explicit name from command line)

This enables fresh projects to use `snap run --show-state` without requiring session creation first.

## Show State with Sessions

`snap run [session] --show-state` displays state for the resolved session:

1. Resolve session or legacy layout (same logic as normal run)
2. Load state from session-scoped or legacy manager
3. Format human-readable summary or JSON output
   - No state file → displays "No state file exists" with `ui.Info()` formatting

**Without session argument**: Auto-detects (same sequence as run)

## Display Name

The `displayName` field (in `runConfig`) is used in the startup summary:

- Named session: Shows session name (e.g., `snap: auth |`)
- Auto-detected session: Shows session name (e.g., `snap: api |`)
- Legacy layout: Shows tasks directory (e.g., `snap: docs/tasks |`)

## Output Formatting

- **PRD file warning** — When PRD file doesn't exist, displays warning message via `ui.Info()` (dimmed text)
- **No state file message** — When `--show-state` is used but no state exists, displays "No state file exists" via `ui.Info()`

## Path Validation

Security validation is applied only to user-supplied paths:

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
- `TestResolveRunConfig_SessionStateManager_IndependentFromLegacy` — Session/legacy isolation
- `TestResolveRunConfig_SessionWhilePlanning` — Sessions during planning phase
- `TestResolveRunConfig_FreshWithSession` — --fresh flag resets session state
- `TestResolveRunConfig_ShowStateWithSession` — State inspection on session
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
- **Session isolation**: Each session has completely independent state (no cross-contamination)
- **Auto-detection usefulness**: Most useful in single-project repos with one session; multi-session repos require explicit session name
- **Error clarity**: Messages always provide corrective actions (snap new, snap run <name>)
- **Display consistency**: Startup summary shows what's running (session name or tasks dir)
