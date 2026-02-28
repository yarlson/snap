# Workflow: Task Runner & Orchestration

## Runner Overview

**Runner** (`internal/workflow/runner.go`) orchestrates the complete multi-task workflow, managing:

- Startup summary display (shows workflow state, provider, task counts)
- Prompt hint display (on fresh starts with TTY, suppressed on resume)
- Task selection and iteration
- State persistence and resumability
- Workflow control signals (interrupt handling)
- Delegation to StepRunner for per-step execution

**Invocation**: Runner is invoked via `run` function in `cmd/run.go` (previously in `cmd/root.go` before refactoring). The run logic is shared by both bare `snap` command (via defaultCmd) and explicit `snap run` subcommand.

## Startup Sequence

When workflow starts:

1. **Resolve startup action** — Check state: resume existing task or select new one
2. **Print startup summary** — Display `FormatStartupSummary()` line showing:
   - Display name (session name or tasks directory)
   - Provider name
   - Total task count and completed count
   - Action: "starting TASK_X" or "resuming TASK_X from step N"
3. **Print prompt hint** (fresh start with TTY only) — Display "Type a directive and press Enter to queue it between steps"
   - Suppressed on resume (user already knows this)
   - Suppressed when not a TTY (e.g., in CI/non-interactive mode)
4. **Run iteration workflow** — Begin 10-step iteration

## Iteration Workflow (10 Steps)

Before iteration starts, a **task summary** is generated:

- Reads task file content (truncated to 2000 bytes max)
- Calls `TaskSummary()` to generate one-line description via fast model
- Description shown below task header in dim styling for context

Each task executes the following sequence in `runIteration()`:

1. **Implement** — LLM generates implementation code
2. **Ensure Completeness** — Verifies task fully implements requirements
3. **Lint & Test** — Runs linters (`golangci-lint`) and tests (`go test`)
4. **Code Review** — LLM code-review step with feedback
5. **Apply Fixes** — Addresses any review feedback
6. **Verify Fixes** — Re-runs linters and tests on fixed code
7. **Update Docs** — Reviews code changes, updates user-facing documentation
8. **Commit Code** — Stages and commits implementation with conventional message
9. **Update Context** — Updates `docs/context/` with project context
10. **Commit Context** — Commits context changes

**After each step**: Snapshot capture (if enabled) — creates git stash with step state; displays "snapshot saved" or "snapshot skipped: <error>" via `ui.Info()` formatting. Skips Commit steps (tree is clean). See [`../snapshot/snapshots.md`](../snapshot/snapshots.md).

After iteration 10 completes, loop restarts at step 1 for next task.

## Task Duration Tracking

**Implementation** (`runIteration()` in runner.go):

- Captures `taskStart := time.Now()` at iteration start
- Passes `time.Since(taskStart)` to `CompleteWithDuration()` on completion
- Formatted duration displayed in right-aligned dim styling

**Duration formatting** (see [`ui/formatting.md`](../ui/formatting.md#duration-functions)):

- `<60s` → "45s"
- `1m–59m` → "2m 34s"
- `≥60m` → "1h 12m"

## State Management

**StateManager interface** enables:

- Save/load state to/from `.snap/state.json`
- Resumable execution across interruptions
- State persists after every step

**Resumability flow**:

- Detect interrupt signal (Ctrl+C, SIGTERM)
- Save current state
- On restart, load state and resume from exact next step
- No completed work is re-executed

## Prompt Queue Processing

**Between-step prompt handling**:

- Drains queued user prompts between each step
- Failed prompts → displays error count with `ui.DimError()` formatting: "<N> queued prompt(s) failed"

## Control Flow

**Task selection and completion**:

- `selectIdleTask()` — Pick next incomplete task from state
- Scans task directory for TASK\*.md files (see [`tasks.md`](tasks.md))
- If no tasks found, runs diagnostics to detect case-mismatched files or PRD-embedded headers
- When all tasks complete, calls `postrun.Run()` to handle post-completion actions (see [`../infra/postrun.md`](../infra/postrun.md))
- Idle tasks polled until one available

**Signal handling & interruption**:

- Signal handler (SIGINT, SIGTERM) writes interrupt message via `SwitchWriter.Direct()` to bypass paused buffers
- Message shows step context: "State saved at step X/Y — resume with 'snap'"
- Context is cancelled (defer-based), triggering graceful shutdown through normal defer chain
- All deferred cleanup runs (terminal restore, signal cleanup) before process exit
- Context cancellation is checked before each step execution to exit early if needed
- Root-level handler in `cmd/root.go` maps `context.Canceled` errors to exit code 130 (standard SIGINT convention) for all command invocations

**Completion**:

- When all tasks complete, workflow stops
- User can press Ctrl+C to interrupt mid-workflow
