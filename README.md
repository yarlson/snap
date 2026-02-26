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

1. Initialize your snap project:

```bash
snap init
```

This scaffolds your project with template PRD and task files in `docs/tasks/`.

2. Edit your PRD (`docs/tasks/PRD.md`):

```markdown
# My Feature Set

Overview of what you're building.
```

3. Edit your first task file (`docs/tasks/TASK1.md`):

```markdown
# TASK1: User authentication

Implement JWT-based authentication with login and logout endpoints.

## Requirements

- POST /login accepts email/password, returns JWT
- POST /logout invalidates token
- Middleware validates JWT on protected routes

## Acceptance Criteria

- [ ] Login endpoint returns JWT on valid credentials
- [ ] Logout endpoint invalidates token
- [ ] Middleware rejects requests without valid JWT
```

Create additional task files (`TASK2.md`, `TASK3.md`, ...) for each feature.

4. Run snap:

```bash
snap
```

snap implements TASK1, runs tests, reviews code, commits, then moves to TASK2. Press Ctrl+C to stop. Run `snap` again to resume.

See `example/` for a complete working example.

## Usage

### Main command

```bash
snap [flags]
```

Runs the task-by-task implementation workflow. By default, reads tasks from `docs/tasks/` and implements each task until stopped.

### Init subcommand

```bash
snap init [flags]
```

Scaffolds a new snap project with template PRD and task files. Safe to run multiple times—existing files are not overwritten. Creates:
- `docs/tasks/PRD.md` (product requirements template)
- `docs/tasks/TASK1.md` (first task template)

Common usage:
```bash
# Initialize project in default location (docs/tasks)
snap init

# Initialize in custom directory
snap init --tasks-dir ./features
```

### Flags

| Flag           | Short | Default              | Description                                            |
| -------------- | ----- | -------------------- | ------------------------------------------------------ |
| `--version`    |       |                      | Show version and exit                                  |
| `--tasks-dir`  | `-d`  | `docs/tasks`         | Directory containing PRD and task files (persistent)   |
| `--prd`        | `-p`  | `<tasks-dir>/PRD.md` | Path to PRD file                                       |
| `--fresh`      |       | `false`              | Ignore saved state and start fresh                     |
| `--show-state` |       | `false`              | Display current workflow state and exit                |

### Examples

```bash
# Initialize a new project with templates
snap init

# Initialize in custom directory
snap init -d ./features

# Run implementation workflow (default: docs/tasks)
snap

# Run with custom tasks directory
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

1. **Implement** - Reads task file and implements the feature
2. **Ensure completeness** - Verifies the task is fully implemented
3. **Lint & test** - Runs linters and tests, fixes any issues
4. **Code review** - Reviews all changes
5. **Apply fixes** - Addresses review feedback
6. **Verify fixes** - Re-runs linters and tests after fixes
7. **Update docs** - Updates user-facing documentation
8. **Commit code** - Stages and commits with conventional commit message
9. **Update memory** - Updates project memory vault for future context
10. **Commit memory** - Commits memory vault changes

After step 10, snap starts the next task.

## Interactive prompt queue

While snap runs, type a directive and press Enter. Your prompt queues up and executes between steps.

When starting in an interactive terminal, you'll see a hint:

```
snap: docs/tasks/ | claude | 3 tasks (1 done) | starting TASK2
Type a directive and press Enter to queue it between steps
```

The startup summary shows your progress (tasks completed, current action), then the workflow begins:

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
│   ├── PRD.md        # Product context and overview
│   ├── TASK1.md      # First task specification
│   ├── TASK2.md      # Second task specification
│   └── ...
├── .memory/          # Optional: project context for Claude
└── .snap/
    └── state.json    # Auto-managed workflow checkpoint
```

**Task files**: Name them `TASK1.md`, `TASK2.md`, etc. snap discovers and implements them in order.

## Resume from interruption

snap saves state after every completed step in `.snap/state.json`. If interrupted (Ctrl+C, crash, system restart), run `snap` again - it resumes from the exact step where it stopped.

On resume, you'll see a startup summary showing your position:

```
snap: docs/tasks/ | claude | 3 tasks (1 done) | resuming TASK2 from step 5
```

This confirms which task and step you're resuming from before the workflow continues.

State is automatically cleaned up after each task completes.

## Troubleshooting

| Symptom                                    | Solution                                                                                                                                  |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `claude: command not found`                | Install the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) and ensure it's in your PATH, or set `SNAP_PROVIDER=codex`        |
| `codex: command not found`                 | Install Codex CLI and ensure `codex` is in your PATH                                                                                      |
| `failed to load state: corrupt state file` | Run `snap --fresh` to reset state                                                                                                         |
| snap implements the wrong task             | Run `snap --show-state` to see current progress. Use `snap --fresh` to restart from the first incomplete task |

## Development

```bash
go test ./...          # Run tests
golangci-lint run      # Lint
go build .             # Build
```

## License

[MIT](LICENSE)
