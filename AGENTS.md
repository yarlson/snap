# Project Context

This is a Go-based CLI tool. For detailed project context (architecture, terminology, conventions, domain knowledge), see [`docs/context/`](docs/context/) — start with [`context-map.md`](docs/context/context-map.md).

## Structure

`module github.com/yarlson/snap`

- `cmd/` - CLI commands (Cobra-based)
- `internal/` - Core business logic
- `docs/context/` - Project context (architecture, domain knowledge, conventions)
- `main.go` - Entry point

## Dependency Management

When adding dependencies:

- Use `go get package@latest` (not direct edits to `go.mod`)
- Don't pin versions during installation
- Run `go mod tidy` after adding dependencies

## Development Workflow (Required)

**Test-Driven Development:**
Write tests BEFORE implementation. No exceptions.

1. Write failing test
2. Implement minimal code to pass
3. Refactor if needed
4. Run all checks

## Quality Checks (Required)

After every code change, you must run:

- `golangci-lint run` - linting (must show 0 issues)
- `go test ./...` - all tests (must pass)

This is non-negotiable. No commits without passing checks.

## Visual Validation

For CLI/TUI output, use the `scr` skill to validate visual presentation:

- Terminal output formatting and layout
- Color schemes and visual aesthetics
- Progress indicators and spinners
- Table rendering and alignment
- Error message display

Generate screenshots to verify the user-facing experience looks correct.

## Language-Specific Conventions

- For Go conventions, see [docs/GO.md](docs/GO.md)
