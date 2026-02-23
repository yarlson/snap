Read AGENTS.md (or CLAUDE.md) and all linked docs to discover the project's required linters and test commands. Run them all and fix any failures.

## Process

1. Read AGENTS.md or CLAUDE.md for project-specific linter and test commands
2. Run all required linters
3. Run all tests
4. For each failure: fix the issue, re-run the failing check to confirm
5. Repeat until all linters report zero issues and all tests pass

## Scope

- Only fix lint errors and test failures — do not refactor or improve unrelated code
- If a fix requires changing logic, keep it minimal and focused on the failing check
- Do not update the memory vault

Done when all linters pass with zero issues and all tests pass.
