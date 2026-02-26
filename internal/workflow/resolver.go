package workflow

import (
	"fmt"

	"github.com/yarlson/snap/internal/state"
)

type startupAction int

const (
	actionResume startupAction = iota
	actionSelect
)

type startupTarget struct {
	action   startupAction
	taskID   string
	taskFile string
	step     int
	tasks    []TaskInfo // Scanned tasks (populated on resume, available for caller use)
}

// resolveStartup determines whether to resume an active task or select a new one.
// When the state has an active task, it validates that the task file exists in the
// tasks directory and that the step is within bounds. When idle, it returns a select
// target without scanning the filesystem.
// Returns an error with recovery guidance for inconsistent state.
// The returned target includes scanned tasks when resuming, which can be reused to
// avoid redundant directory scans by the caller.
func resolveStartup(workflowState *state.State, tasksDir string, totalSteps int) (*startupTarget, error) {
	if workflowState == nil || workflowState.CurrentTaskID == "" {
		return &startupTarget{action: actionSelect}, nil
	}

	// Active task exists — validate for resume.

	// Scan tasks directory to verify active task file still exists.
	tasks, err := ScanTasks(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan tasks for resume validation: %w", err)
	}

	var found bool
	for _, s := range tasks {
		if s.ID == workflowState.CurrentTaskID {
			found = true
			// Backfill file field when missing (e.g. after v1 migration).
			if workflowState.CurrentTaskFile == "" {
				workflowState.CurrentTaskFile = s.Filename
			}
			break
		}
	}
	if !found {
		return nil, fmt.Errorf(
			"active task %s not found in %s (file may have been deleted or renamed); use --fresh to reset or --show-state to inspect",
			workflowState.CurrentTaskID, tasksDir,
		)
	}

	// Defense-in-depth: active task must not be in completed list.
	for _, id := range workflowState.CompletedTaskIDs {
		if id == workflowState.CurrentTaskID {
			return nil, fmt.Errorf(
				"active task %s is already marked as completed; use --fresh to reset",
				workflowState.CurrentTaskID,
			)
		}
	}

	// Validate step against the current workflow step count (not state's TotalSteps,
	// which may be from an older version of the workflow).
	if workflowState.CurrentStep < 1 || workflowState.CurrentStep > totalSteps+1 {
		return nil, fmt.Errorf(
			"invalid step %d for %s (expected 1-%d); use --fresh to reset or --show-state to inspect",
			workflowState.CurrentStep, workflowState.CurrentTaskID, totalSteps,
		)
	}

	return &startupTarget{
		action:   actionResume,
		taskID:   workflowState.CurrentTaskID,
		taskFile: workflowState.CurrentTaskFile,
		step:     workflowState.CurrentStep,
		tasks:    tasks,
	}, nil
}
