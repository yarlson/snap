# snap

Autonomous feature implementation from PRD to production. Write your requirements as tasks, run snap, and watch it implement, test, review, and commit each task without manual intervention.

## Overview

snap automates the task-by-task implementation workflow:

- Reads next unimplemented task from PRD
- Implements the task with full TDD workflow
- Validates with linters and tests
- Reviews code changes
- Commits changes with conventional commit messages
- Updates memory vault for future context

Runs continuously until interrupted with Ctrl+C. Interrupted? No problem. snap saves state after every step and resumes exactly where it left off.

## Installation

```bash
go install github.com/yarlson/snap@latest
```

Or clone and build:

```bash
git clone https://github.com/yarlson/snap.git
cd snap
go build -o snap .
```

### Prerequisites

- Go 1.25.6 or later
- Default provider: [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) installed and in PATH
- Optional provider: Codex CLI (`codex`) installed and in PATH

## Quickstart

1. Create your tasks directory:

```bash
mkdir -p docs/tasks
```

2. Write your PRD (`docs/tasks/PRD.md`):

```markdown
# My Feature Set

## TASK1: User authentication

Implement JWT-based authentication with login and logout endpoints.

Requirements:

- POST /login accepts email/password, returns JWT
- POST /logout invalidates token
- Middleware validates JWT on protected routes

## TASK2: User profile CRUD

...
```

3. Create a progress tracker (`docs/tasks/progress.md`):

```markdown
# Implementation Progress

Track completed tasks here. Format: `TASK<n> done`
```

4. Run snap:

```bash
snap
```

snap implements TASK1, runs tests, reviews code, commits, updates progress, then moves to TASK2. Press Ctrl+C to stop. Run `snap` again to resume.

See `example/` for a complete working example.

## Usage

```bash
snap [flags]
```

### Flags

| Flag           | Short | Default              | Description                                 |
| -------------- | ----- | -------------------- | ------------------------------------------- |
| `--version`    |       |                      | Show version and exit                       |
| `--tasks-dir`  | `-d`  | `docs/tasks`         | Directory containing PRD and progress files |
| `--prd`        | `-p`  | `<tasks-dir>/PRD.md` | Path to PRD file                            |
| `--fresh`      |       | `false`              | Ignore saved state and start fresh          |
| `--show-state` |       | `false`              | Display current workflow state and exit     |

### Examples

```bash
# Default: reads docs/tasks/PRD.md
snap

# Custom tasks directory
snap --tasks-dir ./features

# Custom PRD path
snap -d docs -p docs/requirements.md

# Ignore saved state, start from scratch
snap --fresh

# Show current workflow checkpoint
snap --show-state
```

## Configuration

| Variable        | Description                                           | Default  |
| --------------- | ----------------------------------------------------- | -------- |
| `SNAP_PROVIDER` | Provider selection (`claude`, `claude-code`, `codex`) | `claude` |

### Provider selection

snap uses Claude by default. To run with Codex:

```bash
SNAP_PROVIDER=codex snap
```

Supported values:

- `claude` (default)
- `claude-code` (alias for `claude`)
- `codex`

## How it works

snap runs a 10-step workflow for each feature task:

1. **Implement** - Reads PRD and progress, implements only the next task
2. **Check** - Verifies the task is fully implemented
3. **Validate** - Runs linters and tests, fixes any issues
4. **Review** - Reviews all changes with code-review
5. **Fix** - Addresses review feedback
6. **Validate fix** - Re-runs linters and tests after fixes
7. **Commit** - Stages and commits with conventional commit message
8. **Update vault** - Updates project memory for future context
9. **Update progress** - Marks task as done in progress.md
10. **Commit** - Commits progress tracking

After step 10, snap starts the next task.

## Interactive prompt queue

While snap runs, type a directive and press Enter. Your prompt queues up and executes between steps.

```
▶ Step 3/10: Validate implementation
> use table-driven tests instead of individual test functions
┌──────────────────────────────────────────────────────┐
│  Queued: "use table-driven tests instead of..."      │
│  Will run after Step 3/10: Validate implementation   │
└──────────────────────────────────────────────────────┘
```

Press Enter with no text to see what's queued.

## Project structure

snap expects this layout:

```
your-project/
├── docs/tasks/
│   ├── PRD.md        # Feature requirements (TASK1, TASK2, ...)
│   └── progress.md   # Tracks completed tasks
├── .memory/          # Optional: project context for Claude
└── .snap/
    └── state.json    # Auto-managed workflow checkpoint
```

**PRD format**: Use `## TASK<n>: <title>` headers. snap reads these sequentially and implements one at a time.

**Progress format**: snap appends `TASK<n> done` after each task. Start empty or pre-populate with completed tasks.

## Resume from interruption

snap saves state after every completed step in `.snap/state.json`. If interrupted (Ctrl+C, crash, system restart), run `snap` again - it resumes from the exact step where it stopped.

State is automatically cleaned up after each task completes.

## Troubleshooting

| Symptom                                    | Solution                                                                                                                                  |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `claude: command not found`                | Install the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) and ensure it's in your PATH, or set `SNAP_PROVIDER=codex`        |
| `codex: command not found`                 | Install Codex CLI and ensure `codex` is in your PATH                                                                                      |
| `failed to load state: corrupt state file` | Run `snap --fresh` to reset state                                                                                                         |
| snap implements the wrong task             | Check `docs/tasks/progress.md`. snap picks the first task not marked as done. Edit `progress.md` manually if a task is incorrectly marked |

## Development

```bash
go test ./...          # Run tests
golangci-lint run      # Lint
go build .             # Build
```

## License

[MIT](LICENSE)
