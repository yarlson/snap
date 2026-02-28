# CLI: Signal Handling & Exit Codes

## Entry Point Behavior

**Root command handler** (`cmd/root.go` → `Execute()`):

- Executes the Cobra root command and its subcommands
- Catches errors from command execution
- Maps specific error types to Unix-standard exit codes before process termination

## Exit Code Mapping

**Context Cancellation** (from workflow interruption):

- Condition: Error is `context.Canceled` (triggered by signal handler in runner)
- Exit code: **130** (standard SIGINT convention: 128 + 2)
- Implementation: `errors.Is(err, context.Canceled)` check in root Execute handler
- Effect: Signals to parent processes (shell, CI/CD) that workflow was interrupted by signal

**General Command Errors**:

- Exit code: **1** (generic error)
- Any other error type falls through to default handling

## Signal Flow Architecture

1. **OS sends signal** (SIGINT from Ctrl+C or SIGTERM from system)
2. **Runner's signal handler** (goroutine in `internal/workflow/runner.go`):
   - Receives signal via `sigChan`
   - Writes interrupt message via `SwitchWriter.Direct()` to bypass paused buffers
   - Calls `cancel()` on context (via deferred cleanup in `Run()`)
3. **Main goroutine** (in Runner.Run):
   - Detects `context.Canceled` when reading from `ctx.Done()`
   - Returns `context.Err()` to caller
4. **Root Execute handler**:
   - Receives `context.Canceled` error
   - Maps to exit code 130
   - Process exits with standard code

## Guarantees

- **Graceful shutdown**: All deferred cleanup (terminal restore, signal.Stop) runs before exit
- **State persistence**: Workflow state saved after each step, enabling resumability
- **Message visibility**: Interrupt message always displayed, even if output buffer was paused
- **Signal safety**: `signal.Stop()` called before exit, so second SIGINT gets Go's default termination behavior
