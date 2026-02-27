# snap

Write tasks. Run snap. Get committed code.

snap is a CLI that turns task specs into tested, reviewed, committed code — autonomously. It orchestrates an AI coding agent through a structured 10-step workflow per task: implement, test, review, fix, commit. No babysitting. No copy-pasting prompts. Just `snap run` and come back to clean commits.

## Quickstart

```bash
# Install
go install github.com/yarlson/snap@latest

# Create a session and plan your tasks interactively
snap new my-feature
snap plan my-feature

# Let snap implement everything
snap run my-feature
```

That's it. `snap plan` walks you through requirements and generates task files. `snap run` picks them up one by one and implements each to completion.

### Prerequisites

- Go 1.25.6+
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) in your PATH (default provider)
- Or: [Codex CLI](https://openai.com/index/introducing-codex/) with `SNAP_PROVIDER=codex`

## The workflow

Each task goes through 10 steps. snap saves state after every step — interrupt anytime, resume exactly where you left off.

```
TASK1.md ─┐
           │   ┌─────────────────────────────────────┐
           ├──▶│  1. Implement feature                │ ◀─ Thinking model (deep)
           │   │  2. Verify completeness              │ ◀─ Thinking model (deep)
           │   │  3. Lint & test                      │
           │   │  4. Code review                      │ ◀─ Thinking model (deep)
           │   │  5. Apply fixes from review          │
           │   │  6. Re-validate after fixes          │
           │   │  7. Update docs                      │
           │   │  8. Commit code                      │
           │   │  9. Update memory vault              │
           │   │ 10. Commit memory                    │
           │   └──────────────────┬──────────────────┘
           │                      │
TASK2.md ──┘                      ▼ next task
```

Steps 1, 2, and 4 use a thinking model (Opus) for deep analysis. The rest use a fast model (Haiku) for speed. Context carries across steps within a task.

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

## Two ways to set up tasks

### Let snap plan (recommended)

```bash
snap new auth-system
snap plan auth-system          # Interactive: chat about requirements, type /done
snap run auth-system
```

snap generates PRD, technology decisions, design doc, and numbered task files — then implements them all.

You can also feed requirements from a file:

```bash
snap plan auth-system --from requirements.md
```

### Write tasks yourself

Drop markdown files in `docs/tasks/`:

```
docs/tasks/
├── PRD.md           # Context and overview (optional)
├── TASK1.md         # First task spec
├── TASK2.md         # Second task spec
└── TASK3.md         # ...
```

Then run:

```bash
snap run
```

Task files are `TASK1.md`, `TASK2.md`, etc. (uppercase, numbered). Each should describe what to build, requirements, and acceptance criteria. See `example/` for a working sample.

## Commands

| Command                 | Description                                       |
| ----------------------- | ------------------------------------------------- |
| `snap run [session]`    | Run the implementation workflow                   |
| `snap plan [session]`   | Interactively plan and generate task files        |
| `snap new <name>`       | Create a named session                            |
| `snap list`             | List all sessions with progress                   |
| `snap status [session]` | Show task completion and current step             |
| `snap delete <name>`    | Delete a session (`--force` to skip confirmation) |

Session argument is optional when only one session exists — snap auto-detects it.

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
