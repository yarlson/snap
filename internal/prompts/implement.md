## Context

1. Read CLAUDE.md or AGENTS.md if present — follow all project conventions
2. Read .memory/memory-map.md, then summary.md, terminology.md, practices.md, and relevant domain files
3. Read {{.PRDPath}} for product context
4. If TECHNOLOGY.md exists, read it for architecture and tech stack
5. If TASKS.md exists, read it for task structure
   {{- if .TaskPath}}
6. Read {{.TaskPath}} — this is the task to implement
   {{- end}}
7. Study existing source code for established patterns, naming, and structure

{{if .TaskID}}Implement {{.TaskID}} in this run.{{else}}Pick the next unimplemented task and implement only that one.{{end}}

## Scope

- Implement ONLY what the task defines — nothing more
- Do not refactor or modify code outside the task scope
- Follow patterns already established in the codebase
- Do not update the memory vault

## Process

1. Read the task's acceptance criteria — these define "done"
2. For each piece of functionality:
   - Write a failing test first
   - Implement minimal code to pass
   - Refactor if needed
3. Every public function and every error path must have a test
4. Verify all acceptance criteria are met before finishing

## Quality Guardrails

**Security:**

- No hardcoded secrets — use environment variables
- Validate and sanitize all external input
- Use safe APIs (parameterized queries, escaped output, etc.)
- Enforce authorization on protected operations

**Reliability:**

- Close resources deterministically (defer, finally, context managers, etc.)
- Handle errors explicitly — never swallow them
- Check edge cases: nil/null, empty, boundary values

**Performance:**

- No N+1 patterns — batch or join
- Avoid O(n²) in hot paths

**Architecture:**

- Keep business logic separate from I/O
- No god files (>500 lines) — split by responsibility
