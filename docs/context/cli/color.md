# CLI: Color Output Control

## Overview

snap respects user preferences and environment constraints for color output through the NO_COLOR standard and automatic TTY detection. Colors are intelligently disabled to ensure clean output in CI/CD pipelines, log aggregation systems, and non-interactive environments.

## Implementation

**Files**:

- `internal/ui/tokens.go` — Color and style resolution functions
- `internal/input/inputmode.go` — Dynamic prompt prefix evaluation
- `cmd/root_test.go` — E2E tests for color behavior

## Color Disabling Mechanisms

### NO_COLOR Environment Variable

Follows the [NO_COLOR](https://no-color.org/) standard.

**Behavior**:

- When `NO_COLOR` is set to any non-empty value, all ANSI color codes are disabled
- Usage: `NO_COLOR=1 snap` or `NO_COLOR=true snap`
- Applies globally to all output, including prompt styling and status messages

**Implementation**:

- Checked in `ResolveColor()` and `ResolveStyle()` functions
- Returns empty string (no ANSI code) when NO_COLOR is set

### TTY Detection

Automatic color disabling in non-interactive environments.

**Behavior**:

- Colors automatically disabled when stdout is piped or redirected (non-TTY)
- Example: `snap > output.log` disables colors
- Example: `snap | less` disables colors
- Example: GitHub Actions CI automatically detects non-TTY and disables colors

**Implementation**:

- `internal/ui/tokens.go` calls `isatty.IsTerminal(os.Stdout.Fd())` to detect TTY
- Returns empty string (no ANSI code) for non-TTY output

### Priority

1. NO_COLOR environment variable (explicit user preference) — highest priority
2. TTY detection (implicit environment constraint)
3. Full color output if both checks pass

## Dynamic Evaluation

**Prompt Prefix**:

- Changed from compile-time variable to runtime function `promptPrefix()`
- Evaluates color codes at call time, respecting current color mode
- Located in `internal/input/inputmode.go`
- Used in `activate()` and `redrawLine()` methods

**Rationale**: Ensures prompt styling respects color settings in all contexts (initial prompt, continuation, redraw).

## Testing

**Unit tests** (`internal/ui/tokens_test.go`):

- `TestResolveColor_WithNOCOLOR()` — Verifies colors disabled when NO_COLOR set
- `TestResolveStyle_WithNOCOLOR()` — Verifies styles disabled when NO_COLOR set
- `TestResolveColor_NonTTY()` — Verifies colors disabled in non-TTY context
- `TestResolveStyle_NonTTY()` — Verifies styles disabled in non-TTY context
- `TestColorPriority()` — Confirms NO_COLOR takes priority over TTY detection

**E2E tests** (`cmd/root_test.go`):

- `TestE2E_NoColor_VersionOutputClean()` — Builds snap binary, runs with NO_COLOR=1, verifies no escape sequences in output

## Integration Points

- **Runner** — All status messages, headers, completion messages use color functions
- **Input mode** — Prompt prefix evaluated at call time via function
- **UI package** — All formatting functions use `ResolveColor()` and `ResolveStyle()`
- **README** — Documents NO_COLOR support and use cases in user-facing documentation

## User Documentation

See `README.md` sections:

- **Environment Variables** — NO_COLOR definition and usage
- **Color output** — Detailed explanation with examples:
  - When colors are automatically disabled
  - How to explicitly disable colors with NO_COLOR=1
  - Use cases for CI/CD pipelines and file output
