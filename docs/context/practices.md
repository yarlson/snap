# Practices & Conventions

## Development Workflow

**Test-Driven Development (TDD) — Required**

- Write failing test BEFORE implementation (non-negotiable)
- Implement minimal code to pass test
- Refactor if needed
- All tests must pass before committing

**Quality Checks — Required**

- `golangci-lint run` must show 0 issues before any commit
- `go test ./...` must pass before any commit
- No exceptions — these are blocking requirements

## Dependency Management

- Use `go get package@latest` (never edit `go.mod` directly)
- Do not pin versions during installation
- Run `go mod tidy` after adding dependencies

## Code Organization

**Package structure**:

- `cmd/` — CLI commands (Cobra-based):
  - `root.go` — Main command entry point
  - `run.go` — Run subcommand with session support (named sessions, auto-detection, legacy fallback)
  - `run_test.go` — Unit tests for session resolution logic
  - `run_e2e_test.go` — End-to-end tests for all run workflows
  - `new.go` — New session subcommand (create named session)
  - `delete.go` — Delete session subcommand (remove session with confirmation)
  - `list.go` — List sessions subcommand (show all sessions with status)
  - `plan.go` — Plan session subcommand (two-phase interactive/autonomous planning pipeline with resumption support)
  - `plan_e2e_test.go` — End-to-end tests for plan command (session resolution, document generation, signal handling)
  - `status.go` — Status subcommand (show detailed session progress and task state)
  - `status_test.go` — Tests for status command (session resolution, output formatting)
- `internal/` — Core business logic (unexported, private to module)
- `internal/session/` — Session management (create, validate, list, delete, status derivation, path resolution)
- `internal/state/` — State persistence with session-scoped support via `NewManagerInDir()`
- `internal/input/` — Terminal input handling (raw mode, interactive readline with Ctrl+C support, escape sequence handling)
- `internal/plan/` — Planning pipeline (Planner orchestration, Phase 1 requirements gathering with TTY/pipe dispatch, Phase 2 document generation, prompt rendering, template-based generation)
- `internal/plan/prompts/` — Markdown templates for PRD, TECHNOLOGY, DESIGN, and task splitting prompts
- `main.go` — Entry point

**Module**: `github.com/yarlson/snap`

**Command structure**:

- Bare command: `snap` — Invokes run logic via defaultCmd (backward compatible)
- Explicit subcommands: `snap run`, `snap plan`, `snap new`, `snap delete`, `snap list`, `snap status`
- All workflow commands inherit global `--tasks-dir` persistent flag

## Documentation & Memory

**Context (`docs/context/`)**:

- Current state documentation, not change history
- Updated after major features complete or architectural decisions made
- No timestamps, commit hashes, or status tracking

**Code conventions**:

- Follow Go idioms per `docs/GO.md`
- Use testify for assertions and mocks

## UI & Output Conventions

**Terminal styling**:

- Use color codes from `internal/ui/` package (Primary, Secondary, Success, Error, Celebrate, Dim)
- Use weight styles (Bold, Dim, Normal) for emphasis
- Use box-drawing characters for visual containers
- Maintain consistent spacing and indentation

**Color output control**:

- Respect NO_COLOR environment variable (set to any non-empty value to disable colors)
- Automatically disable colors in non-TTY environments (piped output, CI/CD)
- Colors evaluated at runtime, not at build time, to respect mode changes
- All formatting functions use `ResolveColor()` and `ResolveStyle()` which respect color settings
- E2E tests validate NO_COLOR behavior and non-TTY scenarios

**Task duration display**:

- Format as "Xh Ym" for durations ≥1 hour, omit zero components
- Format as "Xm Ys" for durations <1 hour, omit zero seconds if present
- Show "0s" for sub-second durations
- Display right-aligned in completion messages with dim styling

## Terminal Input Handling

**Interactive TTY input for plan command (Phase 1)**:

- Use `tap.Text(ctx, TextOptions)` from `github.com/yarlson/tap` library for styled interactive text input
- Provides placeholder text, validation callbacks, and abort support (Ctrl+C, Escape)
- Dispatch based on TTY detection: use tap.Text for TTY, bufio.Scanner for piped input
- Validation callback returns error for empty input: "enter a message, or /done to finish"
- Returns empty string on user abort (Ctrl+C or Escape) — convert to `context.Canceled` for graceful shutdown

**Raw-mode input for run command reader**:

- Use `input.WithRawMode(fd, fn)` to enter/exit raw terminal mode safely (used by `cmd/run.go`)
- Raw mode ensures terminal restoration on function return, panic, or signal
- Implements signal handlers for SIGINT/SIGTERM to restore terminal before process exit
- Use within WithRawMode: `input.ReadLine(r, w, prompt)` for interactive line input
- ReadLine handles:
  - Arrow keys (left/right) for cursor movement within the line
  - Backspace for rune-aware deletion at any cursor position
  - Ctrl+U to clear the entire line
  - Ctrl+W to delete the word before the cursor
  - Ctrl+C (returns `input.ErrInterrupt`) to abort
  - ESC sequences (consumed to prevent garbage output)
  - UTF-8 multi-byte characters with proper cursor positioning
  - Styled prompts (ColorSecondary + WeightBold, respecting NO_COLOR)

**Input mode selection**:

- Detect TTY: `input.IsTerminal(file)` checks if file is connected to terminal
- For plan Phase 1: dispatch to tap.Text (TTY) or bufio.Scanner (piped)
- For run command reader: use raw-mode input when TTY detected
- Use buffered input (bufio.Scanner) when not TTY (piped/redirected input)

**Error handling in interactive input**:

- For tap.Text: empty string result with no context error signals user abort — convert to `context.Canceled`
- For raw-mode ReadLine: `input.ErrInterrupt` signals user abort (Ctrl+C) — convert to `context.Canceled`
- EOF from ReadLine or Scanner — transition to next phase or exit with graceful completion
- All other errors — propagate with context

## Visual Validation

Use `scr` skill to validate:

- Terminal output formatting and layout
- Color schemes and visual aesthetics
- Progress indicators and spinners
- Table rendering and alignment
- Error message display
- Interactive input prompts and line editing feedback

## Build & Release

**Version management**:

- Version variable in `cmd/root.go` defaults to "dev"
- Development builds: version set to "dev"
- Production builds inject version via ldflags: `go build -ldflags "-X github.com/yarlson/snap/cmd.Version=vX.Y.Z"`
- Version accessible via `snap --version` CLI flag

**CI/CD pipeline**:

- GitHub Actions workflow (`.github/workflows/ci.yml`) runs on every push to `main` and PRs
- Linting job: `golangci-lint` (0 issues required)
- Test job: `go test -race ./...` (must pass)
- CI validation tests in `cmd/root_test.go` ensure workflow configuration stability

**Release automation**:

- GitHub Actions workflow (`.github/workflows/release.yml`) triggered on version tag push (`v*` pattern)
- Release workflow runs lint and test jobs first, then builds and publishes via GoReleaser
- GoReleaser configuration (`.goreleaser.yaml`) defines build targets: linux/darwin, amd64/arm64
- Binaries are static (CGO disabled), version injected at build time via ldflags
- Artifacts published to GitHub Releases (tar.gz for linux, zip for darwin)
- Release validation tests in `cmd/root_test.go` ensure workflow and config stability
