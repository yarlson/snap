# Infrastructure: Release Automation

## Release Workflow

**File**: `.github/workflows/release.yml`

GitHub Actions workflow triggered on push to version tags (format: `v*`).

**Jobs** (sequential execution):

1. **lint** — `golangci-lint` validation (0 issues required)
2. **test** — `go test -race ./...` to detect race conditions
3. **release** — Depends on both lint and test; runs GoReleaser to build and publish binaries

**Trigger**: Push events with tags matching `v*` pattern (e.g., `v1.0.0`, `v2.1.3`)

**Permissions**: `contents: write` — Allows workflow to create and push release artifacts to GitHub Releases

## GoReleaser Configuration

**File**: `.goreleaser.yaml`

Automates binary build, packaging, and release distribution across platforms.

**Build Targets**:

- OS: linux, darwin (macOS)
- Architecture: amd64, arm64
- Binary name: `snap`
- CGO disabled (static binaries)

**Version Injection**:

- Linker flag: `-X github.com/yarlson/snap/cmd.Version={{.Version}}`
- Injects GoReleaser's version variable into binary at build time
- Accessible via `snap --version`

**Artifacts**:

- Format: tar.gz for linux, zip for darwin (macOS)
- Output directory: `dist/` with subdirectories per platform/arch (e.g., `dist/snap_linux_amd64_v1/`)
- Metadata: `artifacts.json`, `config.yaml`, `metadata.json`

## Release Testing

**Release workflow validation tests** (`cmd/root_test.go`):

- `TestRelease_WorkflowExistsAndValid()` — Verifies `.github/workflows/release.yml` exists and is valid YAML
- `TestRelease_TriggersOnVersionTagPush()` — Verifies workflow triggers on `v*` tag push
- `TestRelease_RunsLintAndTestBeforeGoreleaser()` — Verifies lint and test jobs run before release job
- `TestGoreleaser_ConfigExistsAndValid()` — Verifies `.goreleaser.yaml` exists and is valid YAML
- `TestGoreleaser_BuildTargets()` — Verifies linux/darwin and amd64/arm64 targets
- `TestGoreleaser_LdflagsInjectVersion()` — Verifies version injection via ldflags
- `TestGoreleaser_CheckPasses()` — Runs `goreleaser check` (requires goreleaser binary in PATH)
- `TestGoreleaser_SnapshotBuildSucceeds()` — Runs `goreleaser build --snapshot` (requires goreleaser binary)

**YAML parsing**: Uses `gopkg.in/yaml.v3` for workflow and config validation

## Dependencies

- `goreleaser/goreleaser-action@v6` — GitHub Actions action for GoReleaser (pinned to v6)
- `goreleaser` binary v2.8.2 — Specified in release job with `with.version`
- `gopkg.in/yaml.v3` — Direct dependency for YAML parsing in tests

## Manual Release Process

1. Tag commit: `git tag v1.2.3` (must start with `v`)
2. Push tag: `git push origin v1.2.3`
3. Workflow automatically triggers: lint → test → release
4. Release published to GitHub Releases with built binaries for all platforms/architectures
5. Access release artifacts via GitHub Releases page

See [`../../practices.md`](../../practices.md) for version management practices.
