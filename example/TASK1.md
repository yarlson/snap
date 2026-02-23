# TASK1: Basic Addition Command

## Objective

Implement an `add` command that takes two numbers as arguments and returns their sum.

## Technical Specification

**Command Interface:**

- Command name: `add`
- Arguments: `<num1> <num2>`
- Output: Sum printed to stdout
- Error output: stderr

**Input Validation:**

- Both arguments must be valid numbers (integer or decimal)
- Accept negative numbers
- Reject non-numeric input with clear error message

**Output Format:**

- Integer result if both inputs are integers
- Decimal result if any input is decimal
- No trailing zeros in decimal output

**Error Handling:**

- Missing arguments: "Error: add requires exactly 2 arguments"
- Invalid number: "Error: invalid number '<value>'"
- Exit code 1 on error, 0 on success

## Implementation Notes

- Use Cobra for command structure
- Use `strconv.ParseFloat` for number parsing
- Round result to reasonable precision (avoid floating point artifacts)

## Acceptance Criteria

- [ ] `calc add 5 3` returns `8`
- [ ] `calc add 2.5 3.7` returns `6.2`
- [ ] `calc add -5 3` returns `-2`
- [ ] `calc add abc 5` returns error to stderr with exit code 1
- [ ] `calc add 5` returns error about missing argument
- [ ] All tests pass
- [ ] golangci-lint passes
