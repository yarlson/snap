package state

import (
	"time"
)

// State represents the persistent state of a workflow execution.
// It enables resumption from the last completed step after interruption
// and tracks which tasks have been completed.
type State struct {
	// TasksDir is the directory containing task files.
	TasksDir string `json:"tasks_dir"`

	// CurrentTaskID is the task being implemented (e.g. "TASK1"), empty when idle.
	CurrentTaskID string `json:"current_task_id,omitempty"`

	// CurrentTaskFile is the filename of the active task (e.g. "TASK1.md"), empty when idle.
	CurrentTaskFile string `json:"current_task_file,omitempty"`

	// CurrentStep is the step number within the workflow (1-indexed).
	CurrentStep int `json:"current_step"`

	// TotalSteps is the total number of steps in the workflow.
	TotalSteps int `json:"total_steps"`

	// CompletedTaskIDs tracks which tasks have been completed.
	CompletedTaskIDs []string `json:"completed_task_ids"`

	// SessionID is the Claude CLI session ID for resuming with -c flag.
	// Empty string means session ID unavailable (graceful degradation to -c flag).
	SessionID string `json:"session_id"`

	// LastUpdated is the ISO8601 timestamp of last state update.
	LastUpdated time.Time `json:"last_updated"`

	// LastError contains error message from last failed step (empty if none).
	LastError string `json:"last_error,omitempty"`

	// PRDPath is the resolved path to PRD.md for validation.
	PRDPath string `json:"prd_path"`
}

// NewState creates a new idle state with default values.
func NewState(tasksDir, prdPath string, totalSteps int) *State {
	return &State{
		TasksDir:         tasksDir,
		CurrentTaskID:    "",
		CurrentTaskFile:  "",
		CurrentStep:      1,
		TotalSteps:       totalSteps,
		CompletedTaskIDs: []string{},
		SessionID:        "",
		LastUpdated:      time.Now(),
		LastError:        "",
		PRDPath:          prdPath,
	}
}

// IsValid checks if the state satisfies invariants.
func (s *State) IsValid() bool {
	if s == nil {
		return false
	}
	// Step bounds: 1 to TotalSteps for in-progress, TotalSteps+1 for completed (before reset).
	if s.CurrentStep < 1 || s.CurrentStep > s.TotalSteps+1 {
		return false
	}
	if s.PRDPath == "" {
		return false
	}
	if s.TasksDir == "" {
		return false
	}
	// CompletedTaskIDs must be unique.
	seen := make(map[string]bool, len(s.CompletedTaskIDs))
	for _, id := range s.CompletedTaskIDs {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	// Active task must not already be completed.
	if s.CurrentTaskID != "" && seen[s.CurrentTaskID] {
		return false
	}
	return true
}

// MarkStepComplete advances to the next step and clears any error.
func (s *State) MarkStepComplete() {
	s.CurrentStep++
	s.LastError = ""
	s.LastUpdated = time.Now()
}

// MarkStepFailed records an error for the current step.
func (s *State) MarkStepFailed(err error) {
	s.LastError = err.Error()
	s.LastUpdated = time.Now()
}

// IsTaskComplete returns true if all steps are complete.
func (s *State) IsTaskComplete() bool {
	return s.CurrentStep > s.TotalSteps
}
