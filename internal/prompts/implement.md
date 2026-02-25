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
2. Read the task's **Test plan** section — it specifies which test types to write (E2E, integration, unit)
3. If TASKS.md has a **Critical User Journeys** section, E2E tests must map to those CUJs by name
4. If TECHNOLOGY.md defines a testing strategy, follow it
5. Run the existing test suite before writing new code — if anything fails, stop and fix it first
6. Work outside-in:
   - Start from the user surface — write a failing E2E or integration test for the happy path first
   - Implement minimal code to pass
   - Add unit tests only for logic with combinatorial edge cases (parsers, validators, state machines)
   - Refactor if needed
7. Prefer real dependencies in tests — mock only at boundaries you don't control (external APIs, third-party services)
8. Run the full test suite after implementation — all existing and new tests must pass, including E2E tests from prior tasks
9. If the task changes user-facing behavior, update the relevant docs (README, CLI help text, usage examples, API docs)
10. Verify all acceptance criteria are met before finishing

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

**Simplicity:**

- Don't create abstractions, interfaces, or wrapper types with only one implementation
- Don't extract helpers or utilities for code used in one place — inline it
- Don't add extension points, hooks, or configuration for hypothetical future needs
- Don't wrap standard library or framework APIs — use them directly
- Three similar lines are better than a premature abstraction

**Dependencies:**

- Prefer the standard library over external packages — add a dependency only when it saves significant complexity
- Before adding a package, check: actively maintained, permissive license (MIT/Apache/BSD), no known vulnerabilities
- One dependency per problem — don't add two packages that solve the same thing

**Architecture:**

- Keep business logic separate from I/O
- No god files (>500 lines) — split by responsibility
