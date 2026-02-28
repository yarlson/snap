# CLI: Version Flag Support

## Implementation

**File**: `cmd/root.go`

Version flag support via Cobra's built-in `--version` mechanism:

```go
var Version = "dev"  // Set at build time via ldflags

func init() {
    rootCmd.Version = Version
    rootCmd.SetVersionTemplate("snap {{.Version}}\n")
}
```

**Build-time injection**:

```bash
go build -ldflags "-X github.com/yarlson/snap/cmd.Version=v0.1.0"
```

## Testing

**Unit tests** (`cmd/root_test.go`):

- `TestVersion_DefaultValue()` — Verifies `Version` defaults to "dev"
- `TestVersion_FlagRecognized()` — Verifies `--version` output format

**E2E test**:

- `TestVersion_LdflagsInjection()` — Builds binary with custom version via ldflags, verifies output

## Usage

**Default (dev build)**:

```
$ snap --version
snap dev
```

**Release build**:

```
$ go build -ldflags "-X github.com/yarlson/snap/cmd.Version=v1.2.3" .
$ ./snap --version
snap v1.2.3
```

## Integration

- Cobra's built-in `-v` shorthand not bound; only `--version` available
- Version output sent to stdout, exits with code 0
- No impact on other CLI flags or workflow
