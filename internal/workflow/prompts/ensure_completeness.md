Verify {{.TaskID}} is fully implemented by checking every acceptance criterion in {{.TaskPath}}.

## Context

1. Read CLAUDE.md or AGENTS.md if present — follow all project conventions
2. Read docs/context/context-map.md, then summary.md, terminology.md, practices.md, and relevant domain files
3. Read {{.TaskPath}} for the full task definition and acceptance criteria
4. Read the source code and tests that implement this task

## Process

### Criterion-to-Evidence Mapping

For every acceptance criterion in the task, identify covering evidence (a passing test or validated artifact). Produce a mapping table:

| # | Criterion | Evidence | Status |
|---|-----------|----------|--------|
| 1 | <criterion text> | <test name or artifact> | COVERED / MISSING |

For each criterion with Status = MISSING:

1. Write a failing test at the appropriate layer (E2E, integration, or unit per the task's test plan)
2. Write minimal code to make the test pass
3. Update the mapping table entry to COVERED

After all criteria are mapped, run the full test suite — all tests must pass, including E2E tests from prior tasks.

### UI Verification

This section is conditional on the task's user-facing flag. Read section 0 of the task file — if it says "user-facing: no" or the task has no user-facing impact, skip this subsection entirely.

For user-facing tasks:

1. Verify that UI states from section 4 (UI Deliverables) are implemented and tested
2. Verify that DESIGN.md contract rules applicable to this task are followed
3. Verify that accessibility requirements from DESIGN.md are met
4. Capture actual output and verify it matches expected behavior — take terminal screenshots for CLI/TUI, use browser automation (Playwright, Cypress, etc.) for web, use platform UI automation for native apps, or call endpoints for APIs

Any unmapped or failing UI criterion must be addressed: write a failing test, then minimal code to pass.

## Scope

- Only complete work defined by the current task
- Do not refactor existing code or start the next task
- Do not update the project context

Done when every acceptance criterion has passing code and tests.
