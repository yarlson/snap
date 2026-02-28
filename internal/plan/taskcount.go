package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TaskSpec holds the extracted specification for a single task from TASKS.md section G.
type TaskSpec struct {
	Number   int
	FileName string
	Name     string
	Spec     string // full row content for the sub-agent prompt
}

// CountTasksInSummary reads TASKS.md from tasksDir and counts the task entries in section G.
func CountTasksInSummary(tasksDir string) (int, error) {
	specs, err := ExtractTaskSpecs(tasksDir)
	if err != nil {
		return 0, err
	}
	return len(specs), nil
}

// ExtractTaskSpecs reads TASKS.md from tasksDir and extracts task specifications from section G.
func ExtractTaskSpecs(tasksDir string) ([]TaskSpec, error) {
	content, err := os.ReadFile(filepath.Join(tasksDir, "TASKS.md"))
	if err != nil {
		return nil, fmt.Errorf("read TASKS.md: %w", err)
	}

	return parseTaskSpecs(string(content))
}

// taskRowRe matches a markdown table row with a task number in the first column.
var taskRowRe = regexp.MustCompile(`^\|\s*(\d+)\s*\|`)

// parseTaskSpecs extracts TaskSpec entries from the content of a TASKS.md file.
// It looks for section G and parses markdown table rows with numeric first columns.
func parseTaskSpecs(content string) ([]TaskSpec, error) {
	lines := strings.Split(content, "\n")

	// Find section G.
	inSectionG := false
	var specs []TaskSpec

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section G header.
		if strings.HasPrefix(trimmed, "## G.") || strings.HasPrefix(trimmed, "## G ") {
			inSectionG = true
			continue
		}

		// Detect next section (exit section G).
		if inSectionG && strings.HasPrefix(trimmed, "## ") {
			break
		}

		if !inSectionG {
			continue
		}

		// Skip table header and separator rows.
		if strings.HasPrefix(trimmed, "| #") || strings.HasPrefix(trimmed, "|--") || strings.HasPrefix(trimmed, "| -") {
			continue
		}

		// Match task rows.
		matches := taskRowRe.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		var num int
		if _, err := fmt.Sscanf(matches[1], "%d", &num); err != nil {
			continue
		}

		// Parse columns from the table row.
		cols := strings.Split(trimmed, "|")
		// Trim empty first and last elements from split on leading/trailing |.
		var cleaned []string
		for _, col := range cols {
			c := strings.TrimSpace(col)
			if c != "" {
				cleaned = append(cleaned, c)
			}
		}

		spec := TaskSpec{Number: num, Spec: trimmed}
		if len(cleaned) >= 2 {
			spec.FileName = cleaned[1]
		}
		if len(cleaned) >= 3 {
			spec.Name = cleaned[2]
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// splitBatches divides items into batches of the given size.
// The last batch may be smaller than batchSize.
func splitBatches[T any](items []T, batchSize int) [][]T {
	if batchSize <= 0 || len(items) == 0 {
		return nil
	}

	var batches [][]T
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}
	return batches
}
