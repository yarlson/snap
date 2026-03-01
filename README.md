# snap

Without snap, this is your afternoon:

```
You: "implement auth middleware"  →  Claude writes it
You: "run the tests"              →  they fail
You: "fix the failing test"       →  Claude fixes it
You: "review the code"            →  2 issues found
You: "fix those"                  →  Claude fixes them
You: "run tests again"            →  pass
You: "commit this"                →  done
You: "now do task 2..."           →  repeat from the top
```

Same loop, every task, one prompt at a time.

snap runs the loop for you. `snap plan`, `snap run`, walk away. Come back to tested, reviewed, committed code.

## Quickstart

```bash
# Install
go install github.com/yarlson/snap@latest

# Plan your tasks interactively (creates a session automatically)
snap plan my-feature

# Let snap implement everything
snap run my-feature
```

That's it. `snap plan` walks you through requirements and generates task files. `snap run` picks them up one by one and implements each to completion. On a fresh project with no sessions, `snap plan` automatically creates a default session.

### Prerequisites

- Go 1.25.6+
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) in your PATH (default provider)
- Or: [Codex CLI](https://openai.com/index/introducing-codex/) with `SNAP_PROVIDER=codex`
- For GitHub remotes: [gh CLI](https://cli.github.com/) (optional, only needed if pushing to GitHub)

## Planning

`snap plan` is an interactive session where you describe what you want to build. Chat about your requirements, type `/done`, and snap generates the full planning scaffold: PRD, technology decisions, design doc, and numbered task files.

```bash
snap plan my-feature           # Chat about requirements, type /done when ready
snap run my-feature            # Implements everything
```

On a fresh project with no sessions, `snap plan` automatically creates a session. You can also pre-create named sessions with `snap new <name>`.

If you run `snap plan` again on a session with existing planning artifacts, snap will prompt you to either clean up and re-plan, or create a new session (in interactive mode). Non-interactive mode shows clear instructions to prevent accidental overwrites.

Or skip the chat and feed a requirements file:

```bash
snap plan --from requirements.md
```

### Manual task files

If you prefer full control, write task files directly in `docs/tasks/` and run `snap run`. Name them `TASK1.md`, `TASK2.md`, etc. (uppercase, numbered). Each should describe what to build, requirements, and acceptance criteria. See `example/` for a working sample.

## The workflow

Each task goes through 10 steps. snap saves state after every step — interrupt anytime, resume exactly where you left off.

```
TASK1.md ──┐
           │   ┌─────────────────────────────────────┐
           ├──▶│  1. Implement feature               │ ◀─ Thinking model (deep)
           │   │  2. Verify completeness             │ ◀─ Thinking model (deep)
           │   │  3. Lint & test                     │
           │   │  4. Code review                     │ ◀─ Thinking model (deep)
           │   │  5. Apply fixes from review         │
           │   │  6. Re-validate after fixes         │
           │   │  7. Update docs                     │
           │   │  8. Commit code                     │
           │   │  9. Update project context          │
           │   │ 10. Commit context                  │
           │   └──────────────────┬──────────────────┘
           │                      │
TASK2.md ──┘                      ▼ next task
```

Steps 1, 2, and 4 use a thinking model (Opus) for deep analysis. The rest use a fast model (Haiku) for speed. Context carries across steps within a task.

After each task, snap updates `docs/context/` — a project knowledge base it maintains itself. Architecture decisions, conventions, terminology, and domain knowledge accumulate as tasks complete. Task 10 understands the codebase as well as task 1 built it.

## Auto-push and PR creation

When all tasks are complete, snap automatically pushes commits to your configured git remote (`origin`):

- **No remote configured**: Pushes are skipped, workflow completes cleanly
- **Non-GitHub remote**: Commits are pushed; PR and CI features are skipped
- **GitHub remote**: Commits are pushed, then snap creates a PR with an LLM-generated title and description, then monitors CI status until completion

If push fails (e.g., rejected by remote), the error is displayed and the workflow stops.

### GitHub PR Creation

On GitHub remotes, after pushing:

1. snap skips PR creation if you're on the default branch (e.g., `main`)
2. snap skips PR creation if a PR already exists for this branch
3. snap uses Claude to generate a concise PR title (< 72 chars) and body that explains _why_ the changes were made, using your PRD as context
4. snap creates the PR via `gh` CLI and displays the URL

Requires `gh` CLI in PATH — pre-validated during startup if you're on a GitHub remote.

### CI Status Monitoring & Auto-Fix

After pushing and creating a PR (or pushing to the default branch), snap monitors GitHub Actions workflows:

- If no CI workflows are configured (no `.github/workflows/*.yml`), snap completes cleanly
- If CI workflows exist, snap polls their status with live terminal updates showing check progress
- Individual check status is displayed when ≤5 checks (e.g., `lint: passed, test: running`)
- When >5 checks exist, status is summarized (e.g., `3 passed, 1 running, 2 pending`)
- When all checks pass, snap prints `CI passed — PR ready for review` and completes
- **If any check fails, snap automatically attempts to fix it**:
  - Fetches the CI failure logs (in-memory only, never written to disk)
  - Calls Claude to diagnose the issue and apply a minimal fix
  - Creates a new commit with message `fix: resolve <check-name> CI failure`
  - Pushes the fix and re-polls CI
  - Repeats up to 10 times; if CI still fails after 10 attempts, stops with an error

Status updates only print when check status changes — polls with no changes are silent.

## Steering while it runs

While snap works, type a directive and press Enter. It queues up and runs between steps.

```
▶ Step 3/10: Validate implementation
> use table-driven tests instead of individual test functions
┌──────────────────────────────────────────────────────┐
│  Queued: "use table-driven tests instead of..."      │
│  Will run after Step 3/10: Validate implementation   │
└──────────────────────────────────────────────────────┘
```

You stay in control without breaking the flow.

## Commands

| Command                 | Description                                       |
| ----------------------- | ------------------------------------------------- |
| `snap run [session]`    | Run the implementation workflow                   |
| `snap plan [session]`   | Interactively plan and generate task files        |
| `snap new <name>`       | Create a named session                            |
| `snap list`             | List all sessions with progress                   |
| `snap status [session]` | Show task completion and current step             |
| `snap delete <name>`    | Delete a session (`--force` to skip confirmation) |

Session argument is optional: `snap plan` auto-creates a default session if none exist, and auto-detects when exactly one session exists.

### Flags

| Flag                | Description                                              |
| ------------------- | -------------------------------------------------------- |
| `--fresh`           | Discard saved state, start over                          |
| `--show-state`      | Print current progress and exit (`--json` for raw state) |
| `--tasks-dir`, `-d` | Custom tasks directory (default: `docs/tasks`)           |
| `--prd`, `-p`       | Custom PRD file path                                     |
| `--from`            | Feed requirements from file (plan command only)          |
| `--version`         | Print version                                            |

## Configuration

| Variable        | Description                                   | Default  |
| --------------- | --------------------------------------------- | -------- |
| `SNAP_PROVIDER` | AI provider: `claude`, `claude-code`, `codex` | `claude` |
| `NO_COLOR`      | Disable colored output (any non-empty value)  | unset    |

## Resume from anywhere

snap checkpoints after every step. Ctrl+C, crash, reboot — doesn't matter.

```bash
snap run my-feature
# ... interrupted at step 5 of TASK2 ...

snap run my-feature
# snap: my-feature | claude | 3 tasks (1 done) | resuming TASK2 from step 5
```

Picks up exactly where it stopped. State lives in `.snap/state.json` (or `.snap/sessions/<name>/state.json` for sessions).

## Troubleshooting

| Problem                     | Fix                                                                                              |
| --------------------------- | ------------------------------------------------------------------------------------------------ |
| `claude: command not found` | Install [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) or use `SNAP_PROVIDER=codex` |
| `codex: command not found`  | Install Codex CLI and add to PATH                                                                |
| Corrupt state file          | `snap run --fresh`                                                                               |
| Wrong task running          | `snap run --show-state` to check, `snap run --fresh` to reset                                    |

## Development

```bash
go test ./...          # Tests
golangci-lint run      # Lint
go build .             # Build
```

## License

[MIT](LICENSE)
