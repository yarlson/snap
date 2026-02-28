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
- `internal/input/` — Terminal input detection and reader configuration (TTY detection, input mode selection, structured reader)
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

**Interactive TTY input via tap components (`github.com/yarlson/tap`)**:

- `tap.Text(ctx, TextOptions)` — styled text input with validation, placeholder, and abort support (Ctrl+C, Escape)
  - Used in Phase 1 for requirements gathering and in conflict guard for session name entry
  - Validation callback rejects invalid input; tap displays error inline and keeps user in prompt (field content preserved)
  - Returns empty string on user abort (Ctrl+C or Escape) — convert to `context.Canceled` for graceful shutdown
- `tap.Select(ctx, SelectOptions[T])` — styled selection prompt with arrow-key navigation
  - Used in conflict guard for replan/new-session choice
  - First option pre-selected; user navigates with up/down arrows, confirms with Enter
  - Ctrl+C or Escape returns zero value — convert to `context.Canceled`

**Input mode selection**:

- Detect TTY: `input.IsTerminal(file)` checks if file is connected to terminal
- For plan conflict guard: use tap.Select + tap.Text for TTY, return error for non-TTY
- For plan Phase 1: dispatch to tap.Text (TTY) or bufio.Scanner (piped)
- For run command reader: use structured input mode when TTY detected, buffered input when piped

**Error handling in interactive input**:

- For tap.Text/tap.Select: empty/zero result with no context error signals user abort — convert to `context.Canceled`
- EOF from Scanner — transition to next phase or exit with graceful completion
- All other errors — propagate with context

**Testing tap components (mock pattern)**:

- Create mock I/O: `tap.NewMockReadable()`, `tap.NewMockWritable()`
- Set global terminal I/O: `tap.SetTermIO(in, out)` / `defer tap.SetTermIO(nil, nil)`
- Run component in goroutine to prevent deadlock; emit keypresses asynchronously with `time.Sleep` for sync
- Key emission: `in.EmitKeypress(str, tap.Key{Name: str})` for characters; `tap.Key{Name: "return"}` for Enter; `tap.Key{Name: "down"}` for arrow down; `tap.Key{Name: "backspace"}` for backspace; `tap.Key{Name: "c", Ctrl: true}` for Ctrl+C
- Tests using `SetTermIO` must NOT use `t.Parallel()` (global state)
- After validation error, tap keeps field content — emit backspace characters to clear before typing corrected input

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
