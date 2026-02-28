# Workflow: Task Discovery & Diagnostics

## Task File Format

**Valid task filenames**: `TASK1.md`, `TASK2.md`, `TASK3.md`, etc.

- Filenames are **case-sensitive** — must be uppercase `TASK` prefix
- Followed by decimal number and `.md` extension
- Ordered numerically (TASK1 before TASK2)
- Located in configurable `TasksDir` (typically `docs/tasks/`)

**Task file structure**:

```markdown
# TASK1: Feature Name

Feature description and requirements...
```

## Task Scanning

**ScanTasks()** (`internal/workflow/scanner.go`):

- Reads directory specified in runner config
- Uses case-sensitive regex: `^TASK\d+\.md$`
- Returns sorted slice of `TaskInfo` structs
- Returns empty slice if no valid files found

**Task Selection** (`selectIdleTask()` in runner.go):

- Calls `ScanTasks()` to discover available tasks
- Selects first task not in `CompletedTaskIDs` from state
- Returns error if no tasks found

## Task Discovery Diagnostics

When `ScanTasks()` returns empty results, **DiagnoseEmptyTaskDir()** (`scanner.go`) identifies common issues:

### Check 1: Case-Mismatched Filenames

Detects task files with incorrect case (e.g., `task1.md`, `Task2.md`):

- Uses case-insensitive regex: `(?i)^task\d+\.md$`
- Compares against strict uppercase requirement
- Returns hint: `"Found: task1.md (rename to TASK1.md)"`

### Check 2: PRD-Embedded Task Headers

Scans `PRD.md` for task headers (common migration pattern from monolithic docs):

- Looks for lines matching: `^## TASK\d+:`
- Returns hint if found: `"PRD.md contains TASK headers, but snap needs separate files: TASK1.md, TASK2.md, etc."`

## Error Formatting

**FormatTaskDirError()** (`scanner.go`) builds user-facing error message following [DESIGN.md](../../docs/DESIGN.md) pattern:

1. **Error statement**: "Error: no task files found in docs/tasks/"
2. **Context**: Explains expected format ("snap looks for files named TASK1.md, TASK2.md, etc.")
3. **Hints**: Lists diagnostic findings from `DiagnoseEmptyTaskDir()`
4. **Fix**: "To get started: snap plan my-project" or create task files manually

**Example output**:

```
Error: no task files found in docs/tasks/

snap looks for files named TASK1.md, TASK2.md, etc.
Found: task1.md (rename to TASK1.md)

To get started:
  snap plan my-project
```

## Integration

**Runner** (`selectIdleTask()` in runner.go):

1. Calls `ScanTasks()`
2. If empty, calls `DiagnoseEmptyTaskDir()`
3. Passes hints to `FormatTaskDirError()`
4. Returns formatted error to user

**Testing**:

- Unit tests in `scanner_test.go` cover each diagnostic case
- E2E test `TestE2E_TaskFileErrorRecovery_CaseMismatch` validates error recovery flow
