# Example Project for snap

This directory contains a sample PRD and task files for testing snap's workflow.

## Structure

- **PRD.md** - Main product requirements document with 3 tasks
- **TASK1.md** - Detailed specification for basic addition
- **TASK2.md** - Detailed specification for subtraction and multiplication
- **TASK3.md** - Detailed specification for division with error handling
- **progress.md** - Tracks completed tasks (initially empty)

## Project Concept

A simple calculator CLI tool with four basic operations: add, subtract, multiply, and divide.

## Usage with snap

From this directory, run:

```bash
snap
```

snap will:

1. Read the PRD and identify unimplemented tasks
2. Implement each task following TDD practices
3. Validate with tests and linters
4. Commit changes after each task
5. Update progress.md automatically

## Expected Outcome

After running snap, you should have:

- A working calculator CLI tool
- Complete test coverage
- Three implementation commits
- Updated progress.md showing all tasks completed
