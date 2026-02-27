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

Alternatively, use sessions for named projects:

```bash
# Create a named session
snap new my-project

# Create task files in .snap/sessions/my-project/tasks/

# Run the session by name
snap run my-project
```

See `example/` for a complete working example.

## Usage

### Main workflow command

```bash
snap [flags]
snap run [session] [flags]
```

Runs the task-by-task implementation workflow. By default, reads tasks from `docs/tasks/` and implements each task until stopped. Optionally specify a `[session]` name to run a named session (created with `snap new`).

**Auto-detection**: If no session name is provided and exactly one session exists, snap automatically uses it. If multiple sessions exist, snap shows an error with a list of available sessions.

**Session example**: `snap run my-project` runs the session named `my-project` (files in `.snap/sessions/my-project/tasks/`).

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

### Session management commands

snap supports creating and managing named sessions. Each session has its own task directory and state, allowing you to work on multiple independent projects or features.

#### New session

```bash
snap new <name>
```

Creates a new named session. The session directory is `.snap/sessions/<name>/tasks/`. After creation, you can add task files to this directory and run the workflow for that session.

Example:

```bash
# Create a new session
snap new auth-system

# Add tasks to the session
# Create files: .snap/sessions/auth-system/tasks/TASK1.md, TASK2.md, ...

# Run the session workflow
snap run auth-system
```

#### List sessions

```bash
snap list
```

Lists all named sessions in the current project with a summary of tasks and progress:

```
  auth       2 tasks (1 done)  paused at step 5
  api        0 tasks           planning
  cleanup    1 task            complete
```

Sessions are sorted alphabetically by name.

#### Plan session

```bash
snap plan [session]
snap plan [session] --from <file>
```

Interactively plan tasks for a session using a two-phase pipeline:

- **Phase 1 (Interactive)**: Chat with Claude to gather requirements. Type `/done` to move to Phase 2.
- **Phase 2 (Autonomous)**: Claude generates planning documents (PRD, TECHNOLOGY, DESIGN, task files) based on your requirements.

The `[session]` argument is optional. If not provided and exactly one session exists, snap automatically uses it. If multiple sessions exist, snap shows an error with a list of available sessions.

**With `--from` flag**: Skip Phase 1 (interactive gathering) and provide requirements from a file instead:

```bash
snap plan auth --from requirements.md
```

Example:

```bash
# Create a new session
snap new auth-system

# Plan the session interactively
snap plan auth-system

# Or plan with input from a file
snap plan auth-system --from brief.md

# Or auto-detect single session (if only one exists)
snap plan
```

After planning completes, snap lists the generated files and shows the next step to run the session:

```
Files in .snap/sessions/auth-system/tasks:
  PRD.md
  TECHNOLOGY.md
  DESIGN.md
  TASK1.md
  TASK2.md

Run: snap run auth-system
```

#### Delete session

```bash
snap delete <name>
```

Deletes a session and all its files. By default, prompts for confirmation before deletion:

```
Delete session 'auth' and all its files? (y/N)
```

To skip the confirmation prompt and force deletion, use the `--force` flag:

```bash
snap delete <name> --force
```

### Flags

#### Global flags (available on all commands)

| Flag          | Short | Default      | Description                             |
| ------------- | ----- | ------------ | --------------------------------------- |
| `--version`   |       |              | Show version and exit                   |
| `--tasks-dir` | `-d`  | `docs/tasks` | Directory containing PRD and task files |

#### Workflow flags (for `snap` and `snap run`)

| Flag           | Short | Default              | Description                                            |
| -------------- | ----- | -------------------- | ------------------------------------------------------ |
| `--prd`        | `-p`  | `<tasks-dir>/PRD.md` | Path to PRD file                                       |
| `--fresh`      |       | `false`              | Ignore saved state and start fresh                     |
| `--show-state` |       | `false`              | Display human-readable workflow state summary and exit |
| `--json`       |       | `false`              | Output raw JSON (only with `--show-state`)             |

### Examples

```bash
# Initialize a new project with templates
snap init

# Initialize in custom directory
snap init -d ./features

# Run implementation workflow (default: docs/tasks)
snap

# Run with explicit subcommand (equivalent to `snap`)
snap run

# Run with custom tasks directory
snap --tasks-dir ./features

# Custom PRD path
snap -d docs -p docs/requirements.md

# Ignore saved state, start from scratch
snap --fresh

# Show current workflow checkpoint (human-readable format)
snap --show-state

# Show current workflow checkpoint as JSON
snap --show-state --json

# Create a new session
snap new auth-feature

# Plan the session (interactive or with file)
snap plan auth-feature
# Or with input from a file (skips interactive phase)
snap plan auth-feature --from requirements.md

# Add tasks to .snap/sessions/auth-feature/tasks/TASK1.md, TASK2.md, ...

# List all sessions
snap list

# Run a specific session
snap run auth-feature

# Show state for a specific session
snap run auth-feature --show-state

# Delete a session (with confirmation)
snap delete auth-feature

# Force delete without confirmation
snap delete auth-feature --force
```

### Show state output

The `--show-state` flag displays workflow progress. By default, it shows a human-readable summary:

```
TASK2 in progress — step 5/10: Apply fixes — 1 task completed
```

Or when no task is active:

```
No active task — 2 tasks completed
```

To get the complete workflow state as JSON, use `--show-state --json`:

```json
{
  "tasks_dir": "docs/tasks",
  "current_task_id": "TASK2",
  "current_task_file": "TASK2.md",
  "current_step": 5,
  "total_steps": 10,
  "completed_task_ids": ["TASK1"],
  "session_id": "",
  "last_updated": "2025-01-15T14:32:22Z",
  "prd_path": "docs/tasks/PRD.md"
}
```

## Configuration

| Variable        | Description                                           | Default  |
| --------------- | ----------------------------------------------------- | -------- |
| `SNAP_PROVIDER` | Provider selection (`claude`, `claude-code`, `codex`) | `claude` |
| `NO_COLOR`      | Disable color output (set to any non-empty value)     | unset    |

### Provider selection

snap uses Claude by default. To run with Codex:

```bash
SNAP_PROVIDER=codex snap
```

Supported values:

- `claude` (default)
- `claude-code` (alias for `claude`)
- `codex`

### Color output

snap uses color-coded output by default for better readability. Colors are automatically disabled in these cases:

- When the `NO_COLOR` environment variable is set to any non-empty value
- When output is piped (non-TTY environments like CI/CD)

To disable colors explicitly:

```bash
NO_COLOR=1 snap
```

This is useful in CI/CD pipelines or when redirecting output to files.

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

When running a named session, the session name appears in the startup summary:

```
snap: auth-feature | claude | 3 tasks (1 done) | starting TASK2
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

snap expects this layout for the default workflow:

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

For session-based workflows, snap creates:

```
your-project/
├── .snap/
│   ├── sessions/
│   │   ├── <session-name>/
│   │   │   ├── tasks/
│   │   │   │   ├── PRD.md
│   │   │   │   ├── TASK1.md
│   │   │   │   └── ...
│   │   │   └── state.json   # Session-specific state
│   │   ├── <another-session>/
│   │   │   └── ...
│   │   └── .gitignore       # Sessions dir is git-ignored
│   └── state.json           # Global workflow checkpoint
```

**Task files**: Name them `TASK1.md`, `TASK2.md`, etc. snap discovers and implements them in order.

## Resume from interruption

snap saves state after every completed step. For the default layout, state is saved in `.snap/state.json`. For sessions, state is saved in `.snap/sessions/<name>/state.json`. If interrupted (Ctrl+C, crash, system restart), run `snap` or `snap run <session>` again - it resumes from the exact step where it stopped.

On resume, you'll see a startup summary showing your position:

```
snap: docs/tasks/ | claude | 3 tasks (1 done) | resuming TASK2 from step 5
```

When resuming a session:

```
snap: auth-feature | claude | 3 tasks (1 done) | resuming TASK2 from step 5
```

This confirms which task and step you're resuming from before the workflow continues.

State is automatically cleaned up after each task completes.

## Troubleshooting

| Symptom                                    | Solution                                                                                                                           |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| `claude: command not found`                | Install the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) and ensure it's in your PATH, or set `SNAP_PROVIDER=codex` |
| `codex: command not found`                 | Install Codex CLI and ensure `codex` is in your PATH                                                                               |
| `failed to load state: corrupt state file` | Run `snap --fresh` to reset state                                                                                                  |
| snap implements the wrong task             | Run `snap --show-state` to see current progress. Use `snap --fresh` to restart from the first incomplete task                      |

## Development

```bash
go test ./...          # Run tests
golangci-lint run      # Lint
go build .             # Build
```

## License

[MIT](LICENSE)
