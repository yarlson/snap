package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// ValidateName checks that a session name is non-empty and filesystem-safe.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("session name required")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid session name %q (use alphanumeric, hyphens, underscores)", name)
	}
	return nil
}

// Dir returns the path to a session's root directory.
func Dir(projectRoot, name string) string {
	return filepath.Join(projectRoot, ".snap", "sessions", name)
}

// TasksDir returns the path to a session's tasks directory.
func TasksDir(projectRoot, name string) string {
	return filepath.Join(Dir(projectRoot, name), "tasks")
}

// Exists checks whether a session directory exists.
func Exists(projectRoot, name string) bool {
	info, err := os.Stat(Dir(projectRoot, name))
	return err == nil && info.IsDir()
}

// Create validates the session name and creates the session directory structure.
func Create(projectRoot, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if Exists(projectRoot, name) {
		return fmt.Errorf("session '%s' already exists", name)
	}
	return os.MkdirAll(TasksDir(projectRoot, name), 0o755)
}

// Info describes a session's name, task counts, and derived status.
type Info struct {
	Name           string
	TaskCount      int
	CompletedCount int
	Status         string
}

// sessionsDir returns the path to the sessions root directory.
func sessionsDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".snap", "sessions")
}

// taskFileRegex matches TASK<n>.md filenames (uppercase only).
var taskFileRegex = regexp.MustCompile(`^TASK\d+\.md$`)

// List scans .snap/sessions/ and returns info for each session, sorted by name.
func List(projectRoot string) ([]Info, error) {
	dir := sessionsDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info := Info{Name: entry.Name()}
		sessionPath := filepath.Join(dir, entry.Name())

		// Count task files.
		tasksPath := filepath.Join(sessionPath, "tasks")
		if taskEntries, err := os.ReadDir(tasksPath); err == nil {
			for _, te := range taskEntries {
				if !te.IsDir() && taskFileRegex.MatchString(te.Name()) {
					info.TaskCount++
				}
			}
		}

		// Read state.json for completed count and step info.
		statePath := filepath.Join(sessionPath, "state.json")
		stateData, stateErr := os.ReadFile(statePath)

		var st *sessionState
		stateCorrupt := false
		if stateErr == nil {
			var parsed sessionState
			if json.Unmarshal(stateData, &parsed) == nil {
				st = &parsed
				info.CompletedCount = len(parsed.CompletedTaskIDs)
			} else {
				stateCorrupt = true
			}
		}

		// Check for .plan-started marker.
		planMarkerPath := filepath.Join(sessionPath, ".plan-started")
		_, planErr := os.Stat(planMarkerPath)
		hasPlanning := planErr == nil

		// Derive status.
		info.Status = deriveStatus(info.TaskCount, st, stateCorrupt, hasPlanning)

		sessions = append(sessions, info)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})

	return sessions, nil
}

// sessionState is a minimal struct to read state.json fields needed for status derivation.
type sessionState struct {
	CurrentTaskID    string   `json:"current_task_id"`
	CurrentStep      int      `json:"current_step"`
	CompletedTaskIDs []string `json:"completed_task_ids"`
}

func deriveStatus(taskCount int, st *sessionState, stateCorrupt, hasPlanning bool) string {
	if stateCorrupt {
		return "unknown"
	}

	completedCount := 0
	if st != nil {
		completedCount = len(st.CompletedTaskIDs)
	}

	if hasPlanning && completedCount == 0 {
		return "planning"
	}

	if taskCount == 0 && !hasPlanning {
		return "no tasks"
	}

	if st != nil {
		if taskCount > 0 && completedCount >= taskCount {
			return "complete"
		}
		if st.CurrentTaskID != "" && st.CurrentStep > 0 {
			return fmt.Sprintf("paused at step %d", st.CurrentStep)
		}
		return "idle"
	}

	return "idle"
}

// Delete removes a session directory and all its contents.
func Delete(projectRoot, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if !Exists(projectRoot, name) {
		return fmt.Errorf("session '%s' not found", name)
	}
	return os.RemoveAll(Dir(projectRoot, name))
}
