# Plan Command

Interactive two-phase planning pipeline to generate task files from requirements.

## Command Signature

```bash
snap plan [session]
snap plan [session] --from <file>
```

## Session Resolution

- **Explicit session**: `snap plan auth-system` — uses specified session (must exist; error if not found)
- **Auto-detect**: `snap plan` — uses single existing session if exactly one exists; error with session list if multiple or none exist
- **Error cases**:
  - No sessions: "no sessions found\n\nTo create a session:\n snap new <name>"
  - Multiple sessions: Lists available sessions with task counts, prompts user to specify one

Session resolution logic:

1. If session name provided as arg → validate and use
2. If no args → list all sessions
   - 0 sessions: error
   - 1 session: auto-use it
   - 2+ sessions: error with list and prompt

## Two-Phase Pipeline

### Phase 1: Interactive Requirements Gathering

- User chats with Claude via stdin/stdout
- Claude asks clarifying questions about requirements
- User types `/done` to signal completion and move to Phase 2
- Skipped if `--from` flag provides input file
- Skipped if resuming prior planning session (jumps directly to Phase 2 for continuation)

#### Interactive Input Modes

Planner supports two input modes:

**Raw-mode input (TTY)**: When a terminal is available (stdin is a TTY), Phase 1 uses interactive raw-mode input via `input.ReadLine()`:

- Terminal runs in raw mode (no canonical line buffering)
- Provides single-key response to Ctrl+C (returns `input.ErrInterrupt`, aborts planning with context.Canceled)
- Handles escape sequences properly (arrow keys, Home, End consumed without character pollution)
- Backspace/Ctrl+U/Ctrl+W for line editing
- Prompt: "snap plan> "

**Scanner input (piped/redirected)**: When stdin is not a TTY, Phase 1 uses buffered input via `bufio.Scanner`:

- Reads complete lines from pipe/redirect
- Standard EOF handling
- No raw mode terminal manipulation

#### Phase 1 Flow

1. Check for prior planning session via `session.HasPlanHistory()`
2. Display status message:
   - "Planning session '<name>'" for fresh start
   - "using <file> as input" if --from flag provided
   - "Resuming planning for session '<name>'" if resuming
3. If fresh start: Initialize chat with prompt template
4. If resuming: Executor call with `-c` flag continues prior conversation
5. Read/write interactively until user types `/done`:
   - If TTY: Use raw-mode ReadLine (Ctrl+C → context.Canceled)
   - If piped: Use buffered Scanner (EOF → transition to Phase 2)
6. Extract brief from conversation history
7. Transition to Phase 2

### Phase 2: Autonomous Document Generation

- Claude generates 4 planning documents based on brief:
  1. **PRD.md** — Product requirements document (features, acceptance criteria, scope)
  2. **TECHNOLOGY.md** — Technology decisions and architecture overview
  3. **DESIGN.md** — High-level design specification
  4. **TASK\*.md** — Individual task files (TASK1.md, TASK2.md, etc.)
- Each document generated via LLM call with specialized prompt template
- Documents written to `.snap/sessions/<session>/tasks/`
- File listing printed after completion

Phase 2 flow:

1. For each plan step (PRD, Technology, Design, Tasks):
   - Render specialized prompt template
   - Call LLM executor
   - Write output to tasks directory with appropriate filename
   - Display step completion
2. Print file listing showing generated files
3. Print "Run: snap run <session>" suggestion

## --from Flag

**Usage**: `snap plan [session] --from requirements.md`

- Reads file content and uses it as brief instead of Phase 1 interactive gathering
- Skips Phase 1 entirely, jumps directly to Phase 2
- File path can be relative or absolute
- Error if file not found or unreadable
- Filename (basename) displayed in status message

## Provider Integration

- Pre-flight validation: `provider.ValidateCLI()` ensures provider CLI is in PATH
- Creates executor via `provider.NewExecutorFromEnv()`
- Supports multi-provider (Claude default, Codex via env var)

## Signal Handling

- Registers signal handlers for SIGINT, SIGTERM
- User can interrupt (Ctrl+C) at any point
- On signal:
  - Cancel context → triggers graceful shutdown in planner
  - Planner prints "Planning aborted"
  - Exit with context error
- No partial files left in inconsistent state

## Plan Resumption

Plans can be resumed from a prior planning session:

- **Detect prior session**: Check for `.plan-started` marker in session directory via `session.HasPlanHistory()`
- **Resume mode**: When resuming, `Planner` is configured with `WithResume(true)`
- **Conversation continuity**: First executor call uses `-c` flag when resuming to continue prior conversation thread
- **Marker timing**: `.plan-started` marker created after first successful executor call (not before), via `WithAfterFirstMessage()` callback
- **Fresh start**: First planning attempt for a session does not use `-c` flag

Marker lifecycle:

- Created: After first message in planning session (indicates planning started)
- Persists: Remains to distinguish planning-in-progress from completed sessions
- Used for: Status derivation, resumption detection, session status display

Session markers:

- Marker file: `.snap/sessions/<session>/.plan-started`
- Location: `.snap/sessions/<session>/` directory
- Lifetime: Persists across plan resumptions

## Output

### Terminal Output Formatting

When stdin is a terminal (TTY):

- Plan output wrapped with `ui.NewSwitchWriter(os.Stdout, ui.WithLFToCRLF())`
- Converts all LF line endings to CRLF for proper Windows terminal display
- Applied transparently; user sees normal formatted output

When stdin is piped/redirected:

- Output writes directly to stdout
- No line-ending conversion applied

### Completion Output

After successful completion:

- **Phase status messages** — Uses `ui.Step()` formatting for "Planning session", "Gathering requirements", "Generating planning documents"
- **File listing** — Formatted with `ui.Info()`:
  ```
  Files in .snap/sessions/<session>/tasks:
    PRD.md
    TECHNOLOGY.md
    DESIGN.md
    TASK1.md
    TASK2.md
    ...
  ```
- **Run instruction** — Formatted with `ui.Info()`: "Run: snap run <session>"

## Error Handling

- Session resolution errors → formatted with available sessions
- File I/O errors → "failed to read input file: ..." or "failed to write plan marker: ..."
- Provider validation errors → delegated to provider package
- Context cancellation → graceful abort with "Planning aborted" message

## Integration Points

- **session package**: `Resolve()`, `List()`, `Dir()`, `TasksDir()`, `HasPlanHistory()`, `MarkPlanStarted()`
- **provider package**: `ResolveProviderName()`, `ValidateCLI()`, `NewExecutorFromEnv()`
- **workflow package**: `Executor` interface (LLM calls)
- **ui package**: `Interrupted()` formatting function
- **model package**: Task and document models
- **internal/input package** (raw-mode input):
  - `IsTerminal(*os.File)` — checks if file descriptor is a TTY
  - `WithRawMode(fd, fn)` — enters raw terminal mode, executes fn, guarantees restoration on return
  - `ReadLine(r, w, prompt)` — interactive line input with styled prompt, full cursor movement (arrow left/right), mid-line insertion/deletion, Ctrl+U (clear), Ctrl+W (delete word), Ctrl+C, and escape sequence handling
  - `ErrInterrupt` — error returned by ReadLine when user presses Ctrl+C
- **internal/plan package**:
  - Planner implementation, prompt rendering, Phase 1/2 logic
  - Options: `WithResume()`, `WithAfterFirstMessage()`, `WithBrief()`, `WithInput()`, `WithOutput()`, `WithTerminal()`
  - Phase 1 methods: `gatherRequirements()` (dispatches to raw-mode or scanner), `gatherRequirementsRaw()`, `gatherRequirementsScanner()`
  - Callback: `onFirstMessage()` fires after first successful executor call

## Testing

- E2E tests: `cmd/plan_e2e_test.go`
  - Session resolution (explicit, auto-detect, errors)
  - File generation and placement
  - Signal interruption behavior
- Unit tests: `internal/plan/planner_test.go`, `internal/plan/prompt_test.go`
  - Phase 1 interactive chat flow (raw-mode and scanner modes)
  - Phase 2 document generation
  - Prompt rendering with template variables
  - Option application (WithBrief, WithOutput, WithInput, WithTerminal)
- Raw-mode input tests: `internal/input/rawmode_test.go`, `internal/input/readline_test.go`
  - Raw terminal mode enter/exit and restoration
  - Line reading with Ctrl+C handling
  - Backspace and line editing
  - Escape sequence consumption

## Command Implementation

Located in `cmd/plan.go`:

- `planCmd` — Cobra command definition
- `planRun()` — Entry point orchestrating full pipeline
- `resolvePlanSession()` — Session name resolution logic
- `formatMultiplePlanSessionsError()` — Error message formatting
- `printFileListing(w io.Writer, tasksDir string)` — Directory listing after completion with formatted output via io.Writer
- Signal handler setup in `planRun()`
