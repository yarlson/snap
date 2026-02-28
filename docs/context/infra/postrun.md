# Infrastructure: Post-run Operations

Post-run operations execute after all tasks are completed. These steps automate git push and prepare for future GitHub PR and CI features.

## Post-run Module

**Files**:

- `internal/postrun/postrun.go` — Post-run orchestration
- `internal/postrun/git.go` — Git remote detection, push, and branch tracking

## Workflow

After all tasks complete, the workflow runner calls `postrun.Run()` with:

```go
postrun.Run(ctx, postrun.Config{
    Output:    output,
    RemoteURL: remoteURL,      // Pre-detected remote URL (empty = no remote)
    IsGitHub:  isGitHub,       // Pre-detected GitHub flag
    PRDPath:   prdPath,        // For future PR body context (TASK2)
    TasksDir:  tasksDir,       // For future PR body context (TASK2)
})
```

### Step Sequence

1. **Check for remote** — If no `origin` remote configured, skip push and exit cleanly
2. **Push to origin** — Run `git push origin HEAD` (never uses `--force`)
   - Displays progress message: "Pushing to origin..."
   - On success, displays completion with branch name and timing
   - On failure, returns error (workflow stops with error message)
3. **Check for GitHub** — If non-GitHub remote, skip GitHub-specific features and exit
4. **Future GitHub features** — PR creation and CI monitoring (planned in TASK2)

## Git Remote Detection

**Function**: `DetectRemote()` in `internal/postrun/git.go`

Returns the URL for the `origin` remote:

- **Success**: Returns remote URL string
- **No remote**: Returns empty string and nil error (not treated as an error)
- **Error**: Returns error only for actual git failures (e.g., not in a git repository, permission errors)

Uses `git remote get-url origin` under the hood.

## GitHub Remote Detection

**Function**: `IsGitHubRemote(remoteURL string)` in `internal/postrun/git.go`

Returns true if the remote URL points to github.com. Handles:

- **HTTPS format**: `https://github.com/user/repo.git`
- **SSH shorthand**: `git@github.com:user/repo.git`
- **SSH protocol**: `ssh://git@github.com/user/repo.git`

Returns false for empty URL or non-GitHub remotes.

## Git Push

**Function**: `Push(ctx context.Context)` in `internal/postrun/git.go`

Pushes the current branch to `origin` using `git push origin HEAD`:

- Never uses `--force` flag (safe by design)
- Captures stderr output for error reporting
- Returns `PushError` type wrapping git error with stderr output
- `PushError.Error()` displays stderr if available, else underlying error

## Current Branch

**Function**: `CurrentBranch(ctx context.Context)` in `internal/postrun/git.go`

Returns the name of the current branch using `git branch --show-current`.

Used in completion messages to show which branch was pushed.

## Integration Points

**Pre-flight checks** (in `cmd/run.go`):

1. Detect remote via `postrun.DetectRemote()`
2. Check if GitHub via `postrun.IsGitHubRemote(remoteURL)`
3. If GitHub, validate `gh` CLI via `provider.ValidateGH()` (see [`provider.md`](../cli/provider.md))
4. Pass `remoteURL` and `isGitHub` flags to workflow runner

**Completion** (in `internal/workflow/runner.go`):

- After all tasks complete, call `postrun.Run()` with detected remote info
- Runner's `selectIdleTask()` invokes post-run when no more tasks found

## Configuration

Post-run behavior is automatic and requires no user configuration:

- If no remote: Push is skipped
- If non-GitHub remote: Push succeeds, GitHub features skipped
- If GitHub remote: Push succeeds, gh CLI pre-validated during startup

## Future Extensions (TASK2+)

Current implementation handles git push only. Future tasks will add:

- PR creation via `gh pr create`
- CI status monitoring
- Auto-merge handling (if configured)

PRDPath and TasksDir are passed to `postrun.Run()` for future PR body context generation.
