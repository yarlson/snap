Verify {{.TaskID}} is fully implemented by checking every acceptance criterion in {{.TaskPath}}.

## Context

1. Read CLAUDE.md or AGENTS.md if present — follow all project conventions
2. Read .memory/memory-map.md, then summary.md, terminology.md, practices.md, and relevant domain files
3. Read {{.TaskPath}} for the full task definition and acceptance criteria
4. Read the source code and tests that implement this task

## Process

1. For each acceptance criterion in the task, identify which test covers it — if no test maps to a criterion, it is not verified
2. For each criterion without a covering test, write a failing test at the appropriate layer (E2E, integration, or unit per the task's test plan), then minimal code to pass
3. Run the full test suite — all tests must pass

## Scope

- Only complete work defined by the current task
- Do not refactor existing code or start the next task
- Do not update the memory vault

Done when every acceptance criterion has passing code and tests.
