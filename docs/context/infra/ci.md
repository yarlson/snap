# Infrastructure: CI & Continuous Integration

## CI Workflow

**File**: `.github/workflows/ci.yml`

GitHub Actions workflow triggered on push to `main` and pull requests to `main`.

**Jobs**:

- **lint** — Runs `golangci-lint` action to enforce code quality (0 issues required)
- **test** — Runs `go test` with `-race` flag to detect race conditions

**Integration**:

- Runs on every push to `main` and on PR creation/updates
- Both jobs must pass for commits to be merged

## Testing & Validation

**CI validation tests** (`cmd/root_test.go`):

- `TestCI_WorkflowExistsAndValid()` — Verifies `.github/workflows/ci.yml` exists and is valid YAML
- `TestCI_TriggersOnMainPushAndPR()` — Verifies triggers on `main` branch for push and PR events
- `TestCI_RunsLintAndRaceTests()` — Verifies workflow includes golangci-lint and race-condition tests

**YAML parsing**: Uses `gopkg.in/yaml.v3` to load and validate workflow structure

## Dependency

- `gopkg.in/yaml.v3` — Direct dependency (promoted from indirect in `go.mod`)

## Local Equivalent

Development workflow mirrors CI requirements:

- `golangci-lint run` — Local linting (must pass)
- `go test -race ./...` — Local race detection (must pass)

See [`../practices.md`](../../practices.md) for quality check requirements.
