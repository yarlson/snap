# Terminology

## Core Concepts

**Task** — Individual feature or requirement to implement, stored as a separate file (TASK1.md, TASK2.md, etc.) in `docs/tasks/` directory. Filenames are case-sensitive (uppercase required). Files are discovered and ordered numerically by scanner.

**Task Discovery Diagnostics** — Automated checks that identify common task discovery failures: detects case-mismatched filenames (task1.md vs TASK1.md) and PRD-embedded task headers (## TASK1: Feature in PRD.md). Returns user-friendly hints with corrective actions. Triggered when task scanner finds no valid files.

**PRD** (Product Requirements Document) — Optional overview document at `docs/tasks/PRD.md` providing context and scope. Tasks are defined as individual files, not as sections within PRD.

**Iteration** — Single execution of the multi-step workflow for one task, spanning code implementation, validation, review, documentation, and commit phases. See [`workflow/runner.md`](workflow/runner.md) for the complete step sequence.

**Step** — One atomic operation within an iteration (e.g., "Implement", "Validate", "Code review").

**Workflow** — Complete orchestration of multiple iterations (one per task) until all tasks complete or user stops.

**State** — Current progress snapshot saved to `.snap/state.json` (default workflow) or `.snap/sessions/<name>/state.json` (named session), enables resumable execution across interruptions.

**Session** — Named project workspace with isolated task directory, separate state tracking, and independent workflow execution. Each session has its own task files at `.snap/sessions/<name>/tasks/` and state at `.snap/sessions/<name>/state.json`.

**Named Session** — Session created via `snap new <name>` command, allowing multiple independent projects or features to be managed within a single repository.

**Default Session** — Special session named "default" automatically created by `snap plan` when no sessions exist on a fresh project. Enables planning workflow without requiring explicit `snap new <name>` first. Created via `session.EnsureDefault()` which is idempotent.

**Auto-detection** — Feature of `snap run` (without session name) that automatically selects the workflow session when exactly one session exists. If zero sessions exist, falls back to legacy layout. If multiple sessions exist, returns error with list.

**Legacy fallback** — Workflow mode used when no named sessions exist, falling back to task files in `docs/tasks/` directory or `--tasks-dir` flag. Uses global `.snap/state.json` for state tracking, not session-scoped state.

**Display Name** — Session name or tasks directory path shown in startup summary. For named sessions: shows session name (e.g., "auth"). For legacy layout: shows tasks directory path (e.g., "docs/tasks"). Used in startup summary and --show-state output.

## Execution & Control

**Context** — `docs/context/` directory storing project context, conventions, terminology, and architecture decisions persistent across workflow runs.

**Provider** — LLM service used to generate implementations (Claude or Codex, set via `SNAP_PROVIDER` env var).

**Provider CLI validation** — Pre-flight check that verifies the selected provider's CLI binary exists in PATH before workflow execution. Fails early with helpful error message including installation link and alternative provider suggestion.

**Resume** — Ability to continue from exact step where workflow was interrupted, without repeating completed work.

**Startup Summary** — Plain-text line displayed at workflow start, showing display name (session name or tasks directory), provider name, total and completed task counts, and current action (starting/resuming). Format: `snap: <display-name> | <provider> | <N> tasks (<M> done) | <action>`.

**Prompt Hint** — Informational message shown on fresh workflow starts (TTY-only) reminding user they can type directives between steps. Suppressed on resume and in non-interactive environments.

**Plan command** — `snap plan [session]` generates planning documents (PRD, TECHNOLOGY, DESIGN, TASK files) for a session through a two-phase pipeline: Phase 1 (interactive requirements gathering) and Phase 2 (autonomous document generation).

**Two-phase planning** — Planning pipeline consisting of Phase 1 (interactive chat with Claude) and Phase 2 (autonomous document generation). Phase 1 gathers requirements, Phase 2 generates structured planning documents based on requirements.

**Phase 1 (Requirements gathering)** — Interactive phase of planning where user chats with Claude. User types messages to provide context and answer clarifying questions, then types `/done` to complete Phase 1 and advance to Phase 2. Skipped if `--from` flag provides input file.

**Phase 2 (Document generation)** — Autonomous phase of planning where Claude generates four structured documents: PRD.md (requirements), TECHNOLOGY.md (architecture), DESIGN.md (specifications), and TASK\*.md (individual tasks). Documents are generated sequentially and written to `.snap/sessions/<name>/tasks/`.

**Plan marker** — Hidden file `.plan-started` created in session directory when plan command starts, used for session status tracking to distinguish planning-in-progress from completed planning.

**Planning Artifacts** — Generated planning documents stored in a session's tasks directory: TASK*.md (task files), PRD.md (product requirements), TECHNOLOGY.md (technology decisions), DESIGN.md (design specifications). Detected by `HasArtifacts()` function to prevent accidental overwriting.

**Artifact Conflict** — Occurs when `snap plan` is run on a session that already contains planning artifacts. Prevents accidental overwriting of existing planning documents. In TTY mode, prompts user to choose between [1] clean up and re-plan the session, or [2] create a new session. In non-TTY mode, returns error with cleanup instructions.

**Conflict Guard** — Safety mechanism in `snap plan` that detects existing planning artifacts before starting or resuming planning. Implemented via `checkPlanConflict()` function. Handles TTY vs. non-TTY scenarios differently to prevent automated environments from silently overwriting artifacts. In TTY mode, presents two options to user: clean up existing artifacts or create a new session.

**Raw mode** — Terminal mode where input is not line-buffered; each keystroke is immediately available to the application. Implemented via `termios` on Unix. Allows interactive features like single-key response to Ctrl+C and immediate escape sequence handling.

**Interactive input** — User input from a TTY terminal in raw mode, character-by-character via `input.ReadLine()`. Supports Ctrl+C (abort), Backspace/Ctrl+U/Ctrl+W (editing), and proper escape sequence consumption (arrow keys, Home, End).

**ReadLine** — Function that displays a styled prompt, reads one line of input in raw mode, and returns the submitted text. Supports full cursor movement and mid-line editing: arrow keys move cursor within the line, Backspace deletes at any position, printable characters insert at cursor, Enter submits regardless of cursor position, Ctrl+U clears the line, Ctrl+W deletes the word before cursor. Handles Ctrl+C (`ErrInterrupt`), escape sequences (consumed), and UTF-8 multi-byte characters. Prompt styling uses ColorSecondary + WeightBold, respecting NO_COLOR.

**TAP Text** — Component from `github.com/yarlson/tap` library providing styled interactive text input with validation. Supports Ctrl+C (cancel), Escape (cancel), validation callbacks, and context cancellation. Tested as alternative input mechanism for Phase 1 planning via mock-based integration tests in `internal/plan/tap_integration_test.go`.

**ErrInterrupt** — Error returned by `ReadLine()` when user presses Ctrl+C in raw mode, used to signal planning abort (converted to `context.Canceled`).

**Brief** — Extracted requirements from Phase 1 conversation or provided via `--from` file, used as input to Phase 2 document generation prompts.

**Snapshot** — Git stash checkpoint created after each workflow step, capturing working tree state for debugging and recovery (optional, disabled by default).

**Signal handling** — OS signal (SIGINT from Ctrl+C, SIGTERM from system) interrupts the workflow. Runner writes interrupt message via `SwitchWriter.Direct()` to ensure visibility, then cancels context. Root-level handler maps `context.Canceled` to exit code 130 (standard SIGINT convention). All deferred cleanup runs before process exit, preserving state for resumability.

## UI & Output

**Styling** — Terminal text formatting using ANSI color codes and weight styles (bold, dim, normal).

**Duration** — Elapsed time for completed task, formatted as "Xh Ym" or "Xm Ys" and displayed in task completion message.

**Task Summary** — One-line description of a task, max 60 characters, generated by fast LLM from task content and displayed below task header in dim styling.

**State Display** — Human-readable one-line summary of workflow progress (current task, step, completion counts). Can be inspected at any time via `--show-state` flag or output as raw JSON with `--json`. Format: "TASK_ID in progress — step X/Y: StepName — Z tasks completed" or "No active task — Z tasks completed".

**Color codes** — ANSI escape sequences for terminal colors (Primary, Secondary, Success, Error, Celebrate, Dim).

## Build & Release

**Version** — Semantic version string displayed via `--version` flag. Set to "dev" by default, injected at build time via `go build -ldflags "-X github.com/yarlson/snap/cmd.Version=vX.Y.Z"`.

**ldflags** — Linker flags passed to `go build` to inject version and other compile-time constants into the binary.

**Version Tags** — Git tags matching `v*` pattern (e.g., `v1.0.0`, `v2.1.3`). Push of version tag triggers automated release workflow.

**Release Workflow** — GitHub Actions workflow (`.github/workflows/release.yml`) triggered by version tag push. Runs lint, test, then GoReleaser to build and publish binaries.

**GoReleaser** — Tool that automates binary cross-compilation, packaging, and GitHub Releases publishing. Configured via `.goreleaser.yaml`. Builds for multiple OS/architecture combinations.

**Release Job** — Step in release workflow that runs GoReleaser after lint and test jobs pass. Builds binaries for linux/darwin on amd64/arm64 and publishes to GitHub Releases.

## Color Output Control

**NO_COLOR** — Environment variable following the [NO_COLOR](https://no-color.org/) standard. When set to any non-empty value, disables ANSI color output. Useful for CI/CD pipelines, log files, and environments that don't support color. Respected by all output formatting functions. Automatically disabled in non-TTY environments (piped output, file redirection).

**TTY Detection** — Automatic detection of whether output is connected to a terminal (TTY) or piped/redirected. Colors are automatically disabled in non-TTY contexts, enabling clean output for log aggregation and file storage without requiring environment variable configuration.
