# TASK2: Subtraction and Multiplication

## Objective

Add `subtract` and `multiply` commands with the same interface as addition.

## Technical Specification

**Subtract Command:**

- Command name: `subtract`
- Arguments: `<num1> <num2>`
- Returns: `num1 - num2`
- Handles negative results

**Multiply Command:**

- Command name: `multiply`
- Arguments: `<num1> <num2>`
- Returns: `num1 * num2`
- Maintains decimal precision

**Input Validation:**

- Same validation rules as TASK1 (add command)
- Same error messages and exit codes

**Output Format:**

- Same formatting rules as TASK1
- Integer results when possible
- Clean decimal output otherwise

## Implementation Notes

- Reuse validation logic from add command
- Consider extracting common parsing/formatting to shared function
- Follow same Cobra command pattern

## Acceptance Criteria

- [ ] `calc subtract 10 3` returns `7`
- [ ] `calc subtract 3 10` returns `-7`
- [ ] `calc subtract 5.5 2.3` returns `3.2`
- [ ] `calc multiply 4 5` returns `20`
- [ ] `calc multiply 2.5 4` returns `10`
- [ ] `calc multiply -3 4` returns `-12`
- [ ] Invalid inputs return appropriate errors
- [ ] All tests pass
- [ ] golangci-lint passes
