package workflow

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
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
