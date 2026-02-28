# Plan Command

Interactive two-phase planning pipeline to generate task files from requirements.

## Command Signature

```bash
snap plan [session]
snap plan [session] --from <file>
```

## Session Resolution

- **Explicit session**: `snap plan auth-system` — uses specified session (must exist; error if not found)
- **Auto-detect**: `snap plan` — uses single existing session if exactly one exists; or auto-creates "default" session if none exist
- **Error cases**:
  - Multiple sessions: Lists available sessions with task counts, prompts user to specify one

Session resolution logic:

1. If session name provided as arg → validate and use
2. If no args → list all sessions
   - 0 sessions: auto-create "default" session via `session.EnsureDefault()`
   - 1 session: auto-use it
   - 2+ sessions: error with list and prompt

## Conflict Guard: Planning Artifacts Detection

After session resolution, `snap plan` checks for existing planning artifacts (TASK\*.md, PRD.md, TECHNOLOGY.md, DESIGN.md) in the session's tasks directory. If artifacts are found, the behavior depends on input mode:

**TTY (Interactive Terminal)**:

- Uses `tap.Select` from `github.com/yarlson/tap` to present a styled selection prompt:
  - Message: `Session "<name>" already has planning artifacts.`
  - Option 1: "Clean up and re-plan this session" (value: `"replan"`)
  - Option 2: "Create a new session" (value: `"new"`)
- First option is pre-selected; user navigates with arrow keys and confirms with Enter
- Ctrl+C or Escape cancels (returns `context.Canceled`)
- On "replan": Cleans all artifacts via `session.CleanSession()` and proceeds with re-planning (session directory and structure remain intact; only task files and state are removed)
- On "new": Transitions to `tap.Text` prompt for session name entry

- Session creation flow (choice "new"):
  - Uses `tap.Text` with inline validation callback
  - Message: "Session name", Placeholder: "Enter a name for the new session"
  - Validate function checks `session.ValidateName()` and `session.Exists()`
  - If invalid or exists: tap displays validation error inline and keeps user in prompt (field content is preserved)
  - If valid and new: creates session via `session.Create()` and proceeds with planning in new session
  - Ctrl+C or Escape cancels (returns `context.Canceled`)

**Non-TTY (Piped/Redirected Input)**:

- Returns error with instructions:

  ```
  session '<name>' already has planning artifacts

  To re-plan, clean up first:
    snap delete <name> && snap new <name>

  Or plan in a new session:
    snap new <name> && snap plan <name>
  ```

- Prevents accidental overwriting of planning artifacts in automated/CI scenarios

The conflict guard ensures users don't accidentally overwrite planning artifacts without explicit confirmation.

## Two-Phase Pipeline

### Phase 1: Interactive Requirements Gathering

- User chats with Claude via stdin/stdout
- Claude asks clarifying questions about requirements
- User types `/done` to signal completion and move to Phase 2
- Skipped if `--from` flag provides input file
- Skipped if resuming prior planning session (jumps directly to Phase 2 for continuation)

#### Interactive Input Modes

Planner supports two input modes:

**Interactive input (TTY)**: When a terminal is available (stdin is a TTY), Phase 1 uses `tap.Textarea` from the `github.com/yarlson/tap` library:

- Styled multiline text input component with prompt and placeholder text
- Message: "Your response"
- Placeholder: "Describe your requirements, or /done to finish"
- Single-key response to Ctrl+C or Escape (returns empty string, aborts planning with context.Canceled)
- Full line editing and multiline support (Shift+Return for new lines)
- Validation: rejects empty input with error message "enter a message, or /done to finish"

**Scanner input (piped/redirected)**: When stdin is not a TTY, Phase 1 uses buffered input via `bufio.Scanner`:

- Reads complete lines from pipe/redirect
- Standard EOF handling
- No terminal manipulation

#### Phase 1 Flow

1. Check for prior planning session via `session.HasPlanHistory()`
2. Display status message:
   - "Planning session '<name>'" for fresh start
   - "using <file> as input" if --from flag provided
   - "Resuming planning for session '<name>'" if resuming
3. If fresh start: Initialize chat with prompt template
4. If resuming: Executor call with `-c` flag continues prior conversation
5. Read/write interactively until user types `/done`:
   - If TTY: Use tap.Textarea with validation and placeholder (Ctrl+C or Escape → context.Canceled)
   - If piped: Use buffered Scanner (EOF → transition to Phase 2)
6. Extract brief from conversation history
7. Transition to Phase 2

### Phase 2: Autonomous Document Generation

- Claude generates task files based on brief via a 6-step pipeline:
  1. **PRD.md** — Product requirements document (features, acceptance criteria, scope)
  2. **TECHNOLOGY.md** and **DESIGN.md** — Technology decisions and design specification (generated in parallel)
  3. **Task list creation** — Initial rough task breakdown based on PRD, TECHNOLOGY, and DESIGN
  4. **Task assessment** — Evaluates each task against anti-pattern criteria (horizontal slices, infrastructure-only, too broad, too narrow, non-demoable)
  5. **Task refinement** — Fixes flagged tasks via merging, splitting, or reworking to create demoable vertical slices
  6. **TASKS.md generation** — Writes final task summary with sections A–J (overview, assumptions, principles, CUJs, epics, capability map, task list, dependencies, risks, coverage)
- Each document generated via LLM call with specialized prompt template
- **Engineering principles preamble**: All Phase 2 prompts are prepended with shared engineering principles (KISS, DRY, SOLID, YAGNI) to guide consistent decision-making across all generated documents
- Documents written to `.snap/sessions/<session>/tasks/`
- File listing printed after completion

Phase 2 flow:

1. **Step 1 (Sequential)**: Generate PRD
   - Render PRD prompt template
   - Prepend engineering principles preamble
   - Call LLM executor
   - Write PRD.md to tasks directory
   - Display step completion
2. **Step 2 (Parallel)**: Generate TECHNOLOGY.md and DESIGN.md concurrently
   - Render TECHNOLOGY and DESIGN prompt templates
   - Prepend engineering principles preamble to both
   - Call LLM executor for both concurrently via errgroup
   - Write TECHNOLOGY.md and DESIGN.md to tasks directory
   - Display individual sub-step completions with timing
3. **Step 3 (Sequential)**: Create task list
   - Render create-tasks prompt template (reads PRD, TECHNOLOGY, DESIGN from conversation)
   - Prepend engineering principles preamble
   - Call LLM executor in fresh conversation (no -c flag)
   - Produces rough task list in conversation (no files written yet)
   - Display step completion
4. **Step 4 (Sequential)**: Assess tasks
   - Render assess-tasks prompt template
   - Call LLM executor with -c flag (continues create-tasks conversation)
   - No preamble (already in conversation context from step 3)
   - Scores each task against 5 anti-patterns (horizontal, infrastructure-only, too broad, too narrow, non-demoable)
   - Output in conversation (no files written yet)
   - Display step completion
5. **Step 5 (Sequential)**: Refine tasks
   - Render merge-tasks prompt template
   - Call LLM executor with -c flag (continues conversation chain)
   - No preamble (operates on assessed task list from conversation)
   - Fixes flagged tasks via merging, splitting, absorbing, or reworking
   - Performs self-check re-verification against anti-patterns
   - Output in conversation (no files written yet)
   - Display step completion
6. **Step 6 (Sequential)**: Generate task summary
   - Render generate-task-summary prompt template
   - Prepend engineering principles preamble
   - Call LLM executor with -c flag (continues conversation chain)
   - Write TASKS.md with sections A–J to tasks directory
   - Display step completion
7. Print file listing showing generated files
8. Print "Run: snap run <session>" suggestion

### Engineering Principles

All planning prompts (PRD, Technology, Design, Tasks) are guided by shared engineering principles defined in `internal/plan/prompts/principles.md`:

- **KISS** — Prefer simplest solution; avoid premature abstraction, unnecessary indirection, speculative generality
- **DRY** — Single source of truth; eliminate duplication of decisions and intent
- **SOLID** — Single responsibility per module; open for extension, closed for modification; substitutable abstractions; narrow interfaces; depend on abstractions
- **YAGNI** — Build only what's needed now; no hypothetical extension points or configuration for non-existent scenarios

Conflicts resolved in favor of simplicity: straightforward solutions that work today are better than elegant abstractions that anticipate tomorrow.

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

- **session package**: `Resolve()`, `List()`, `Dir()`, `TasksDir()`, `EnsureDefault()`, `HasPlanHistory()`, `MarkPlanStarted()`, `HasArtifacts()`, `CleanSession()`
- **provider package**: `ResolveProviderName()`, `ValidateCLI()`, `NewExecutorFromEnv()`
- **workflow package**: `Executor` interface (LLM calls)
- **ui package**: `Interrupted()` formatting function
- **model package**: Task and document models
- **tap package** (`github.com/yarlson/tap`):
  - `Select(ctx, SelectOptions[T])` — styled selection prompt with arrow-key navigation; used in conflict guard for replan/new-session choice
  - `Text(ctx, TextOptions)` — interactive text input with validation, placeholder, and abort support (Ctrl+C, Escape); used in conflict guard for new session name entry
  - `Textarea(ctx, TextareaOptions)` — interactive multiline text input with validation, placeholder, and abort support (Ctrl+C, Escape); used in Phase 1 requirements gathering
  - `SelectOptions[T]` — configuration with Message and Options ([]SelectOption[T] with Value and Label)
  - `TextOptions` — configuration with Message, Placeholder, and Validate callback
  - `TextareaOptions` — configuration with Message, Placeholder, and Validate callback
- **internal/input package**:
  - `IsTerminal(*os.File)` — checks if file descriptor is a TTY (used by both plan and run for input mode detection)
  - `NewReader()`, `NewMode()` — input handling for run command reader configuration
- **internal/plan package**:
  - Planner implementation, prompt rendering, Phase 1/2 logic
  - Options: `WithResume()`, `WithAfterFirstMessage()`, `WithBrief()`, `WithInput()`, `WithOutput()`, `WithInteractive()`
  - Phase 1 methods: `gatherRequirements()` (dispatches to interactive or scanner), `gatherRequirementsInteractive()` (uses tap.Textarea), `gatherRequirementsScanner()`
  - Callback: `onFirstMessage()` fires after first successful executor call

## Testing

- E2E tests: `cmd/plan_e2e_test.go`
  - Session resolution (explicit, auto-detect, errors)
  - File generation and placement
  - Signal interruption behavior
- Conflict guard tests: `cmd/plan_test.go`
  - Uses tap mock pattern: `tap.NewMockReadable()`, `tap.NewMockWritable()`, `tap.SetTermIO(in, out)`
  - Runs `checkPlanConflict` in goroutine, emits keypresses asynchronously with `time.Sleep` for sync
  - Tests: empty session (no prompt), non-TTY error, choice 1 (Enter selects pre-selected replan), Ctrl+C cancellation, choice 2 with valid name (down arrow + Enter to select, then type name), choice 2 with invalid-then-valid name (backspace to clear after validation error), choice 2 with existing-then-new name, Ctrl+C during name input
  - Note: tap keeps field content after validation error, so tests must emit backspace characters to clear before typing corrected input
- Unit tests: `internal/plan/planner_test.go`, `internal/plan/prompt_test.go`
  - Phase 1 interactive chat flow via tap.Textarea (TTY mode) and scanner (piped mode)
  - Phase 2 document generation
  - Prompt rendering with template variables
  - Option application (WithBrief, WithOutput, WithInput, WithInteractive)
  - Resume mode with interactive input
  - Interactive abort paths: Ctrl+C, Escape, context cancellation
  - Multiple message exchanges in interactive mode
- Interactive input tests (tap.Textarea): `internal/plan/planner_test.go`
  - User message submission with /done command
  - Case-insensitive /done handling
  - Empty input validation (rejected by Validate func)
  - Ctrl+C and Escape abort support
  - Context cancellation
  - Multiple message exchanges
  - Resume mode behavior
- TAP Textarea integration tests: `internal/plan/tap_integration_test.go`
  - Validates tap.Textarea input component from `github.com/yarlson/tap` library
  - Tests via mock I/O: submit with content, validation rejection (empty input), Ctrl+C abort, Escape abort, context cancellation

## Command Implementation

Located in `cmd/plan.go`:

- `planCmd` — Cobra command definition
- `planRun()` — Entry point orchestrating full pipeline
- `resolvePlanSession()` — Session name resolution logic
- `checkPlanConflict(ctx, sessionName, isTTY)` — Conflict guard: detects existing artifacts; uses `tap.Select` for TTY choice (replan or new session), returns error for non-TTY
- `promptNewSession(ctx)` — Creates new session with user-provided name via `tap.Text`; inline validation checks name format and uniqueness; returns new session name
- `formatMultiplePlanSessionsError()` — Error message formatting
- `printFileListing(w io.Writer, tasksDir string)` — Directory listing after completion with formatted output via io.Writer
- Signal handler setup in `planRun()`
