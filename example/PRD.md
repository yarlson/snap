# Calculator CLI - Product Requirements

A simple command-line calculator that performs basic arithmetic operations.

## TASK1: Basic Addition Command

Implement an `add` command that takes two numbers as arguments and returns their sum.

**Requirements:**

- Command: `calc add <num1> <num2>`
- Accepts integer and decimal numbers
- Returns the sum with appropriate precision
- Error handling for invalid inputs
- Exit code 0 on success, 1 on error

**Example:**

```bash
$ calc add 5 3
8
$ calc add 2.5 3.7
6.2
$ calc add abc 5
Error: invalid number 'abc'
```

## TASK2: Subtraction and Multiplication

Add `subtract` and `multiply` commands with the same interface as addition.

**Requirements:**

- Command: `calc subtract <num1> <num2>`
- Command: `calc multiply <num1> <num2>`
- Same input validation as addition
- Proper handling of negative results
- Decimal precision maintained

**Example:**

```bash
$ calc subtract 10 3
7
$ calc multiply 4 2.5
10
```

## TASK3: Division with Error Handling

Add `divide` command with proper division-by-zero error handling.

**Requirements:**

- Command: `calc divide <num1> <num2>`
- Error when dividing by zero
- Result precision up to 2 decimal places
- Same input validation as other commands

**Example:**

```bash
$ calc divide 10 2
5
$ calc divide 10 3
3.33
$ calc divide 5 0
Error: division by zero
```
