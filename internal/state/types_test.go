package state

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	state := NewState("docs/tasks", "docs/tasks/PRD.md", 9)

	if state.TasksDir != "docs/tasks" {
		t.Errorf("expected tasks dir docs/tasks, got %s", state.TasksDir)
	}
	if state.CurrentTaskID != "" {
		t.Errorf("expected empty current task ID, got %s", state.CurrentTaskID)
	}
	if state.CurrentTaskFile != "" {
		t.Errorf("expected empty current task file, got %s", state.CurrentTaskFile)
	}
	if state.CurrentStep != 1 {
		t.Errorf("expected current step 1, got %d", state.CurrentStep)
	}
	if state.SessionID != "" {
		t.Errorf("expected empty session ID, got %s", state.SessionID)
	}
	if state.PRDPath != "docs/tasks/PRD.md" {
		t.Errorf("expected prd path docs/tasks/PRD.md, got %s", state.PRDPath)
	}
	if state.TotalSteps != 9 {
		t.Errorf("expected total steps 9, got %d", state.TotalSteps)
	}
	if state.LastError != "" {
		t.Errorf("expected empty last error, got %s", state.LastError)
	}
	if state.CompletedTaskIDs == nil {
		t.Error("expected non-nil CompletedTaskIDs")
	}
	if len(state.CompletedTaskIDs) != 0 {
		t.Errorf("expected empty CompletedTaskIDs, got %v", state.CompletedTaskIDs)
	}
	if time.Since(state.LastUpdated) > time.Second {
		t.Error("expected last updated to be recent")
	}
}

func TestState_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		want  bool
	}{
		{
			name:  "nil state",
			state: nil,
			want:  false,
		},
		{
			name: "valid idle state",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      1,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: true,
		},
		{
			name: "valid active state",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentTaskID:    "TASK1",
				CurrentStep:      5,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: true,
		},
		{
			name: "valid active state with completed tasks",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentTaskID:    "TASK3",
				CurrentStep:      3,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{"TASK1", "TASK2"},
			},
			want: true,
		},
		{
			name: "step too low",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      0,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: false,
		},
		{
			name: "step at TotalSteps (valid in-progress)",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      10,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: true,
		},
		{
			name: "step at TotalSteps+1 (completed, before reset)",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      11,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: true,
		},
		{
			name: "step too high",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      12,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: false,
		},
		{
			name: "missing prd path",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      5,
				TotalSteps:       10,
				PRDPath:          "",
				CompletedTaskIDs: []string{},
			},
			want: false,
		},
		{
			name: "missing tasks dir",
			state: &State{
				TasksDir:         "",
				CurrentStep:      5,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{},
			},
			want: false,
		},
		{
			name: "duplicate completed IDs",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      1,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{"TASK1", "TASK1"},
			},
			want: false,
		},
		{
			name: "active task already completed",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentTaskID:    "TASK1",
				CurrentStep:      3,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: []string{"TASK1", "TASK2"},
			},
			want: false,
		},
		{
			name: "nil CompletedTaskIDs treated as empty",
			state: &State{
				TasksDir:         "docs/tasks",
				CurrentStep:      1,
				TotalSteps:       10,
				PRDPath:          "prd.md",
				CompletedTaskIDs: nil,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_MarkStepComplete(t *testing.T) {
	state := NewState("docs/tasks", "prd.md", 9)
	state.CurrentStep = 5
	state.LastError = "some error"
	beforeUpdate := state.LastUpdated

	// Wait a bit to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	state.MarkStepComplete()

	if state.CurrentStep != 6 {
		t.Errorf("expected step 6, got %d", state.CurrentStep)
	}
	if state.LastError != "" {
		t.Errorf("expected error cleared, got %s", state.LastError)
	}
	if !state.LastUpdated.After(beforeUpdate) {
		t.Error("expected last updated to be updated")
	}
}

func TestState_MarkStepFailed(t *testing.T) {
	state := NewState("docs/tasks", "prd.md", 9)
	state.CurrentStep = 5
	beforeUpdate := state.LastUpdated

	// Wait a bit to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	err := errors.New("test error")
	state.MarkStepFailed(err)

	if state.CurrentStep != 5 {
		t.Errorf("expected step unchanged at 5, got %d", state.CurrentStep)
	}
	if state.LastError != "test error" {
		t.Errorf("expected error 'test error', got %s", state.LastError)
	}
	if !state.LastUpdated.After(beforeUpdate) {
		t.Error("expected last updated to be updated")
	}
}

func TestState_IsTaskComplete(t *testing.T) {
	tests := []struct {
		name        string
		currentStep int
		want        bool
	}{
		{"step 1", 1, false},
		{"step 5", 5, false},
		{"step 9", 9, false},
		{"step 10", 10, true},
		{"step 11", 11, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState("docs/tasks", "prd.md", 9)
			state.CurrentStep = tt.currentStep

			if got := state.IsTaskComplete(); got != tt.want {
				t.Errorf("IsTaskComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_Summary(t *testing.T) {
	stubStepName := func(n int) string {
		names := map[int]string{
			1: "Implement", 2: "Ensure completeness", 3: "Lint & test",
			4: "Code review", 5: "Apply fixes", 6: "Verify fixes",
			7: "Update docs", 8: "Commit code", 9: "Update memory", 10: "Commit memory",
		}
		if name, ok := names[n]; ok {
			return name
		}
		return "unknown"
	}

	tests := []struct {
		name     string
		state    *State
		contains []string
	}{
		{
			name: "active task at step 5",
			state: &State{
				CurrentTaskID:    "TASK2",
				CurrentStep:      5,
				TotalSteps:       10,
				CompletedTaskIDs: []string{"TASK1"},
			},
			contains: []string{"TASK2 in progress", "step 5/10", "Apply fixes", "1 task completed"},
		},
		{
			name: "no active task with 3 completed",
			state: &State{
				CurrentTaskID:    "",
				CurrentStep:      1,
				TotalSteps:       10,
				CompletedTaskIDs: []string{"TASK1", "TASK2", "TASK3"},
			},
			contains: []string{"No active task", "3 tasks completed"},
		},
		{
			name: "active task at step 1",
			state: &State{
				CurrentTaskID:    "TASK1",
				CurrentStep:      1,
				TotalSteps:       10,
				CompletedTaskIDs: []string{},
			},
			contains: []string{"step 1/10", "Implement"},
		},
		{
			name: "active task at step 10",
			state: &State{
				CurrentTaskID:    "TASK5",
				CurrentStep:      10,
				TotalSteps:       10,
				CompletedTaskIDs: []string{"TASK1", "TASK2", "TASK3", "TASK4"},
			},
			contains: []string{"step 10/10", "Commit memory"},
		},
		{
			name: "no active task with 0 completed",
			state: &State{
				CurrentTaskID:    "",
				CurrentStep:      1,
				TotalSteps:       10,
				CompletedTaskIDs: []string{},
			},
			contains: []string{"No active task", "0 tasks completed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.state.Summary(stubStepName)
			for _, s := range tt.contains {
				if !strings.Contains(summary, s) {
					t.Errorf("Summary() = %q, want it to contain %q", summary, s)
				}
			}
		})
	}
}

func TestState_MarkStepComplete_AfterFinalStep(t *testing.T) {
	state := NewState("docs/tasks", "prd.md", 10)
	state.CurrentStep = 10

	state.MarkStepComplete()

	if !state.IsValid() {
		t.Error("state should be valid after final step completion, before reset")
	}

	if state.CurrentStep != 11 {
		t.Errorf("expected step 11, got %d", state.CurrentStep)
	}
}
