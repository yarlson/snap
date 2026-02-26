Review the changes made in this task and update user-facing documentation if behavior changed.

## Context

1. Read CLAUDE.md or AGENTS.md if present — follow project conventions
2. Read .memory/ for project context

## Process

1. Run `git diff $(git merge-base HEAD origin/main)` to see what changed
2. Read README.md and any other user-facing docs (CLI help text, usage examples, API docs)
3. Determine if changes affect user-facing behavior:
   - New or changed CLI flags, subcommands, or arguments
   - New or changed terminal output (messages, formatting, spinners)
   - New features users interact with or observe
   - Changed defaults, prerequisites, or project structure
4. If user-facing behavior changed — update the relevant documentation sections to reflect current behavior
5. If nothing user-facing changed — do nothing (this is explicitly valid)

## Scope

- Only modify documentation files (README.md, docs/, CLI help text, usage examples)
- Do not modify source code
- Do not update the memory vault
- Do not create new documentation files unless the change clearly warrants it

## What Does Not Count as User-Facing

- Internal refactors with no external effect
- Test-only changes
- Memory vault updates
- Prompt template changes
- Code style or linting fixes
