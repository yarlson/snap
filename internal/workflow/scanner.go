package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// taskFileRegex matches TASK<n>.md filenames (case-sensitive, uppercase only).
var taskFileRegex = regexp.MustCompile(`^TASK(\d+)\.md$`)

// TaskInfo describes a discovered task file.
type TaskInfo struct {
	ID       string // e.g. "TASK1"
	Number   int    // numeric index extracted from filename
	Filename string // e.g. "TASK1.md"
}

// ScanTasks reads the directory and returns all TASK<n>.md files sorted numerically.
// Only regular files matching the strict TASK<n>.md pattern are included.
func ScanTasks(dir string) ([]TaskInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	var tasks []TaskInfo
	seen := make(map[int]string) // number → first filename that claimed it
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := taskFileRegex.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		num, err := strconv.Atoi(matches[1])
		if err != nil {
			continue // Skip unparseable numbers (shouldn't happen with \d+).
		}
		if existing, ok := seen[num]; ok {
			return nil, fmt.Errorf("duplicate task number %d: %s and %s", num, existing, entry.Name())
		}
		seen[num] = entry.Name()
		tasks = append(tasks, TaskInfo{
			ID:       fmt.Sprintf("TASK%d", num),
			Number:   num,
			Filename: entry.Name(),
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Number < tasks[j].Number
	})

	return tasks, nil
}

// caseMismatchRegex matches task filenames case-insensitively (e.g., task1.md, Task2.md).
var caseMismatchRegex = regexp.MustCompile(`(?i)^task(\d+)\.md$`)

// prdTaskHeaderRegex matches PRD lines with embedded task headers (e.g., "## TASK1: Feature").
var prdTaskHeaderRegex = regexp.MustCompile(`^## TASK\d+:`)

// DiagnoseEmptyTaskDir checks for common reasons why ScanTasks returned no results.
// It returns hint strings for case-mismatched filenames and PRD-embedded task headers.
// This function only reads files, never modifies them.
func DiagnoseEmptyTaskDir(dir string) []string {
	var hints []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Check 1: Case-insensitive scan for task files that don't match strict pattern.
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if caseMismatchRegex.MatchString(name) && !taskFileRegex.MatchString(name) {
			matches := caseMismatchRegex.FindStringSubmatch(name)
			correctName := "TASK" + matches[1] + ".md"
			hints = append(hints, fmt.Sprintf("Found: %s (rename to %s)", name, correctName))
		}
	}

	// Check 2: Scan PRD.md for embedded task headers.
	prdPath := filepath.Join(dir, "PRD.md")
	if f, err := os.Open(prdPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if prdTaskHeaderRegex.MatchString(scanner.Text()) {
				hints = append(hints, "PRD.md contains TASK headers, but snap needs separate files: TASK1.md, TASK2.md, etc.")
				break
			}
		}
	}

	return hints
}

// FormatTaskDirError builds a user-facing error message for empty task directory.
// It follows the DESIGN.md error pattern: "Error: <what>", context, hints, fix.
func FormatTaskDirError(dir string, hints []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Error: no task files found in %s/", dir)
	b.WriteString("\n\nsnap looks for files named TASK1.md, TASK2.md, etc.")
	for _, hint := range hints {
		b.WriteString("\n")
		b.WriteString(hint)
	}
	b.WriteString("\n\nTo get started:\n  snap init")
	return b.String()
}

// SelectNextTask returns the first task not in completedIDs, or nil if all are completed.
func SelectNextTask(tasks []TaskInfo, completedIDs []string) *TaskInfo {
	if len(tasks) == 0 {
		return nil
	}

	completed := make(map[string]bool, len(completedIDs))
	for _, id := range completedIDs {
		completed[id] = true
	}

	for i := range tasks {
		if !completed[tasks[i].ID] {
			return &tasks[i]
		}
	}

	return nil
}
