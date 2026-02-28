# Infrastructure: Post-run Operations

Post-run operations execute after all tasks are completed. These steps automate git push and GitHub PR creation.

## Post-run Module

**Files**:

- `internal/postrun/postrun.go` — Post-run orchestration
- `internal/postrun/git.go` — Git remote detection, push, and branch tracking
- `internal/postrun/github.go` — GitHub API operations (PR creation, CI status checking)
- `internal/postrun/workflow.go` — CI workflow detection
- `internal/postrun/prompts/` — LLM prompt templates for PR generation
- `internal/postrun/parse.go` — LLM output parsing for PR title/body

## Workflow

After all tasks complete, the workflow runner calls `postrun.Run()` with:

```go
postrun.Run(ctx, postrun.Config{
    Output:       output,
    Executor:     executor,       // LLM executor for PR generation
    RemoteURL:    remoteURL,      // Pre-detected remote URL (empty = no remote)
    IsGitHub:     isGitHub,       // Pre-detected GitHub flag
    PRDPath:      prdPath,        // PRD.md path for PR body context
    TasksDir:     tasksDir,       // Tasks directory
    RepoRoot:     repoRoot,       // Repository root for workflow detection (defaults to ".")
    PollInterval: pollInterval,   // CI status poll interval (defaults to 15s)
})
```

### Step Sequence

1. **Check for remote** — If no `origin` remote configured, skip push and exit cleanly
2. **Push to origin** — Run `git push origin HEAD` (never uses `--force`)
   - Displays progress message: "Pushing to origin..."
   - On success, displays completion with branch name and timing
   - On failure, returns error (workflow stops with error message)
3. **Check for GitHub** — If non-GitHub remote, skip GitHub-specific features and exit
4. **PR creation flow** (GitHub remotes only):
   - Get default branch via `gh repo view`
   - Skip PR creation if on default branch
   - Check if PR already exists via `gh pr view` (skip if exists)
   - Generate PR title and body via LLM (using PRD context)
   - Create PR via `gh pr create`
   - Display PR URL and number
5. **CI status monitoring** (all remotes with GitHub or after PR creation):
   - Detect if CI workflows exist via `HasRelevantWorkflows()` (scans `.github/workflows/*.yml`)
   - If no relevant workflows: Display "No CI workflows found, done" and exit
   - If workflows exist: Poll CI status via `CheckStatus()` (uses `gh pr checks` or `gh run list`)
   - Display status updates when check status changes (silent between polls)
   - Format status as individual checks (≤5) or grouped summary (>5)
   - On all checks passed: Display "CI passed — PR ready for review" (or "CI passed" for default branch)
   - On any check failed: Return error with failed check names

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

Returns empty string for detached HEAD state.

Used in completion messages to show which branch was pushed, and to determine if PR creation should proceed.

## Diff Stat

**Function**: `DiffStat(ctx context.Context, baseBranch string)` in `internal/postrun/git.go`

Returns diff statistics between the base branch and HEAD using `git diff <baseBranch>...HEAD --stat`.

Used as input to the PR generation prompt to give the LLM context about what changed.

## GitHub Operations

**File**: `internal/postrun/github.go`

### DefaultBranch

Retrieves the repository's default branch using `gh repo view --json defaultBranchRef -q .defaultBranchRef.name`.

Used to determine if the current branch is the default branch (in which case PR creation is skipped).

### PRExists

Checks if a PR already exists for the current branch using `gh pr view --json state,url`.

- Returns `(true, url, nil)` if a PR exists
- Returns `(false, "", nil)` if no PR exists (exit code 1 is not treated as an error)
- Returns error only for actual gh failures

Used for PR deduplication — prevents creating duplicate PRs.

### CreatePR

Creates a new pull request using `gh pr create --title "..." --body "..."`.

Returns the PR URL. Returns error if creation fails.

### CheckStatus

Retrieves CI check status using `gh pr checks` or `gh run list`.

- **For PR branches**: Uses `gh pr checks --json name,state,conclusion` to get individual check results
- **For default branch**: Uses `gh run list --branch <branch> --json name,status,conclusion --limit 1` to get the latest workflow run
- Returns a slice of `CheckResult` structs with normalized statuses: "passed", "failed", "running", "pending"
- Normalizes GitHub's various status values (SUCCESS, PENDING, FAILURE, etc.) to our standard statuses

## CI Workflow Detection

**File**: `internal/postrun/workflow.go`

### HasRelevantWorkflows

Detects if the repository has GitHub Actions workflows triggered by `push` or `pull_request`:

- Scans `.github/workflows/*.yml` and `*.yaml` files
- Uses text-based scanning (not full YAML parsing) for conservative matching
- Matches trigger lines like `on: push`, `on: [push, pull_request]`, `on:\n  push:`, etc.
- Returns `true` if at least one workflow has a relevant trigger
- Returns `false` if `.github/workflows/` doesn't exist or is empty
- Skips unreadable files gracefully (returns false, not error)

Used to determine if CI monitoring should proceed after push/PR creation.

## PR Generation

**File**: `internal/postrun/prompts/pr.md` and `internal/postrun/prompts/prompts.go`

The PR prompt template (`pr.md`) instructs the LLM to:

- Generate a title under 72 characters describing the change
- Generate a body explaining *why* the changes were made using the PRD for context
- Output only the PR content (no preamble)

The `PR()` function renders this template with:

- `{{.PRDContent}}` — Full PRD.md text for context
- `{{.DiffStat}}` — Diff statistics from git

### Output Parsing

**File**: `internal/postrun/parse.go`

The `parsePROutput()` function extracts title and body from LLM output:

- Splits on first blank line (double newline)
- First line becomes the title
- Remaining text becomes the body
- Returns error if output is empty or malformed

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

## Implementation Status

Current implementation includes:

- ✅ Git push to remote
- ✅ GitHub PR creation with LLM-generated title and body (TASK2)
- ✅ CI status monitoring and polling (TASK3)

Future tasks will add:

- CI failure log reading and LLM-driven fix attempts (TASK4)
- Auto-merge handling (post-TASK4)

The PRD context is passed to the PR generation prompt to create meaningful PR descriptions that explain the *why* behind changes, not just raw diff summaries.
