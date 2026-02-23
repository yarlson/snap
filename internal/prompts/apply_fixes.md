Fix all issues identified in the code review.

## Process

1. Re-read the review findings from the previous step
2. For each finding (CRITICAL and HIGH first, then MEDIUM and LOW):
   - Apply the fix
   - Run the relevant test or linter to confirm resolution
   - Move to the next finding
3. If a fix introduces a new failure, resolve it before moving on

## Scope

- Only fix issues raised in the review — do not refactor or improve unrelated code
- Keep fixes minimal and focused
- Do not update the memory vault

Done when all actionable findings from the review are resolved.
