# TASK3: Division with Error Handling

## Objective

Add `divide` command with proper division-by-zero error handling.

## Technical Specification

**Divide Command:**

- Command name: `divide`
- Arguments: `<num1> <num2>`
- Returns: `num1 / num2`
- Result precision: up to 2 decimal places

**Division by Zero:**

- Check for zero divisor before operation
- Error message: "Error: division by zero"
- Exit code 1

**Input Validation:**

- Same validation rules as previous commands
- Additional check for zero divisor

**Output Format:**

- Always show up to 2 decimal places for division results
- Example: `10 / 3 = 3.33`
- Example: `10 / 2 = 5.00` or `5` (implementation choice)

## Implementation Notes

- Explicit zero check before division
- Use `fmt.Sprintf("%.2f", result)` for formatting
- Consider if you want to show `.00` for whole numbers

## Acceptance Criteria

- [ ] `calc divide 10 2` returns `5` (or `5.00`)
- [ ] `calc divide 10 3` returns `3.33`
- [ ] `calc divide 7 2` returns `3.50` (or `3.5`)
- [ ] `calc divide 5 0` returns "Error: division by zero" to stderr
- [ ] `calc divide 0 5` returns `0` (or `0.00`)
- [ ] Invalid inputs return appropriate errors
- [ ] All tests pass
- [ ] golangci-lint passes
