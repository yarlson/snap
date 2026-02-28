# CLI: Provider Validation

## Overview

Pre-flight validation ensures the selected LLM provider's CLI binary is available in PATH before attempting workflow execution. This prevents cryptic errors during task implementation and provides helpful installation guidance.

## Implementation

**Files**:

- `internal/provider/factory.go` — Validation logic and provider metadata
- `cmd/root.go` — Integration point (line ~74, before workflow starts)

### ValidateCLI Function

```go
func ValidateCLI(providerName string) error
```

Checks that the provider's CLI binary exists in PATH. If not found, returns formatted error with:

- Provider name (e.g., "claude")
- Installation URL (provider-specific documentation link)
- Alternative provider suggestion
- Installation instructions

### Provider Metadata

Map `providers` in `internal/provider/factory.go` defines:

- **Binary**: CLI command name (`claude`, `codex`)
- **InstallURL**: Provider documentation link
- **DisplayName**: User-facing name ("Claude CLI", "Codex CLI")
- **Alternative**: Alternative provider to suggest (if claude missing, suggest codex; vice versa)

Current providers:

- **Claude**: `https://docs.anthropic.com/en/docs/claude-cli`
- **Codex**: `https://github.com/openai/codex`

### ResolveProviderName Function

```go
func ResolveProviderName() string
```

Returns normalized provider name from `SNAP_PROVIDER` environment variable (lowercased, trimmed). Defaults to "claude" if unset.

## Integration

Validation called in `cmd/root.go:run()` after provider resolution:

```go
providerName := provider.ResolveProviderName()
if err := provider.ValidateCLI(providerName); err != nil {
    return err  // Error output sent to CLI user
}
```

Executes **before** workflow starts, blocking execution if provider unavailable.

## Error Format

Follows DESIGN.md user-facing error pattern:

```
Error: claude not found in PATH

snap requires the Claude CLI to run. Install it:
  https://docs.anthropic.com/en/docs/claude-cli

Or use a different provider:
  SNAP_PROVIDER=codex snap
```

## Testing

**Unit tests** (`internal/provider/factory_test.go`):

- `TestValidateCLI_ClaudeMissing()` — Validates error format for missing claude
- `TestValidateCLI_CodexMissing()` — Validates error format for missing codex
- `TestValidateCLI_BinaryExists()` — Confirms no error when binary exists
- `TestValidateCLI_UnknownProvider()` — Rejects unsupported providers
- `TestValidateCLI_ProviderBinaryMapping()` — Verifies all supported providers mapped
- `TestValidateCLI_ErrorFormat()` — Validates user-facing error structure
- `TestProviderMapMatchesExecutorFactory()` — Ensures ValidateCLI and NewExecutorFromEnv stay in sync

**E2E test** (`cmd/root_test.go`):

- `TestPreflightProviderCLI_MissingBinary()` — End-to-end: builds snap binary, removes provider from PATH, verifies helpful error

## GitHub CLI Validation

**Function**: `ValidateGH()` in `internal/provider/factory.go`

Checks that the `gh` CLI binary exists in PATH. Required when workflow targets a GitHub remote and intends to create PRs or interact with CI (see [`../infra/postrun.md`](../infra/postrun.md)).

Called during pre-flight checks in `cmd/run.go` if git remote is detected as GitHub.

### Error Format

```
Error: gh not found in PATH

GitHub features require the gh CLI. Install it:
  https://cli.github.com/

Or use a non-GitHub remote to skip GitHub features
```

### Testing

**Unit tests** (`internal/provider/factory_test.go`):

- `TestValidateGH_Missing()` — Validates error format when gh not in PATH
- `TestValidateGH_Exists()` — Confirms no error when gh binary exists

## Design Notes

- **Guard against drift**: `TestProviderMapMatchesExecutorFactory()` ensures every provider in the ValidateCLI map is also supported by NewExecutorFromEnv
- **Cross-platform**: Uses `exec.LookPath()` for binary discovery, handles Windows `.exe` extensions automatically
- **Helpful errors**: Provides installation link and alternative provider without requiring user investigation
- **Early failure**: Validates before workflow starts, avoiding wasted time on provider unavailability
