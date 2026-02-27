package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManager_LoadAndSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Load should return nil when no state exists
	state, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != nil {
		t.Fatal("expected nil state when file doesn't exist")
	}

	// Create and save a state
	state = NewState("docs/tasks", "docs/tasks/PRD.md", 9)
	state.CurrentTaskID = "TASK2"
	state.CurrentStep = 5
	state.SessionID = "session_123"

	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load should return the saved state
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}

	// Verify state fields
	if loaded.TasksDir != state.TasksDir {
		t.Errorf("TasksDir = %s, want %s", loaded.TasksDir, state.TasksDir)
	}
	if loaded.CurrentTaskID != state.CurrentTaskID {
		t.Errorf("CurrentTaskID = %s, want %s", loaded.CurrentTaskID, state.CurrentTaskID)
	}
	if loaded.CurrentStep != state.CurrentStep {
		t.Errorf("CurrentStep = %d, want %d", loaded.CurrentStep, state.CurrentStep)
	}
	if loaded.SessionID != state.SessionID {
		t.Errorf("SessionID = %s, want %s", loaded.SessionID, state.SessionID)
	}
	if loaded.PRDPath != state.PRDPath {
		t.Errorf("PRDPath = %s, want %s", loaded.PRDPath, state.PRDPath)
	}
}

func TestManager_SaveCreatesDirAndGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	state := NewState("docs/tasks", "prd.md", 9)
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Check that .snap directory was created
	snapDir := filepath.Join(tmpDir, StateDir)
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		t.Error(".snap directory was not created")
	}

	// Check that .gitignore was created
	gitignorePath := filepath.Join(snapDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Error(".snap/.gitignore was not created")
	}

	// Verify .gitignore content
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	expectedContent := "*\n!.gitignore\n"
	if string(content) != expectedContent {
		t.Errorf(".gitignore content = %q, want %q", string(content), expectedContent)
	}
}

func TestManager_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Save initial state
	state := NewState("docs/tasks", "prd.md", 9)
	state.CurrentStep = 3
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Update and save again
	state.CurrentStep = 5
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify no temp files left behind
	snapDir := filepath.Join(tmpDir, StateDir)
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}

	// Verify state is correct
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.CurrentStep != 5 {
		t.Errorf("CurrentStep = %d, want 5", loaded.CurrentStep)
	}
}

func TestManager_SaveNilState(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	err := mgr.Save(nil)
	if err == nil {
		t.Error("expected error when saving nil state")
	}
	if err.Error() != "cannot save nil state" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManager_SaveInvalidState(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// State with invalid step
	state := NewState("docs/tasks", "prd.md", 9)
	state.CurrentStep = 0 // Invalid

	err := mgr.Save(state)
	if err == nil {
		t.Error("expected error when saving invalid state")
	}
	if err.Error() != "cannot save invalid state" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManager_LoadCorruptState(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Create .snap directory
	snapDir := filepath.Join(tmpDir, StateDir)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Write invalid JSON
	statePath := filepath.Join(snapDir, StateFile)
	if err := os.WriteFile(statePath, []byte("invalid json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should return error
	state, err := mgr.Load()
	if err == nil {
		t.Error("expected error when loading corrupt state")
	}
	if state != nil {
		t.Error("expected nil state when load fails")
	}
}

func TestManager_LoadInvalidState(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Create .snap directory
	snapDir := filepath.Join(tmpDir, StateDir)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Write valid JSON but invalid state (step out of range)
	invalidState := &State{
		TasksDir:         "docs/tasks",
		CurrentStep:      0, // Invalid
		PRDPath:          "prd.md",
		CompletedTaskIDs: []string{},
		LastUpdated:      time.Now(),
	}
	data, err := json.Marshal(invalidState)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	statePath := filepath.Join(snapDir, StateFile)
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should return error
	state, err := mgr.Load()
	if err == nil {
		t.Error("expected error when loading invalid state")
	}
	if state != nil {
		t.Error("expected nil state when validation fails")
	}
}

func TestManager_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Save a state
	state := NewState("docs/tasks", "prd.md", 9)
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify state exists
	if !mgr.Exists() {
		t.Fatal("state should exist after save")
	}

	// Reset should delete the state
	if err := mgr.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	// Verify state no longer exists
	if mgr.Exists() {
		t.Error("state should not exist after reset")
	}

	// Reset on non-existent state should not error
	if err := mgr.Reset(); err != nil {
		t.Errorf("Reset() on non-existent state error = %v", err)
	}
}

func TestManager_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Should not exist initially
	if mgr.Exists() {
		t.Error("state should not exist initially")
	}

	// Create state
	state := NewState("docs/tasks", "prd.md", 9)
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Should exist after save
	if !mgr.Exists() {
		t.Error("state should exist after save")
	}

	// Delete state
	if err := mgr.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	// Should not exist after reset
	if mgr.Exists() {
		t.Error("state should not exist after reset")
	}
}

func TestManager_HumanReadable(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	// Save a state
	state := NewState("docs/tasks", "docs/tasks/PRD.md", 9)
	state.CurrentTaskID = "TASK3"
	state.CurrentStep = 7
	state.SessionID = "session_abc123"
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw file content
	statePath := filepath.Join(tmpDir, StateDir, StateFile)
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Verify it's indented JSON (human-readable)
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("JSON parsing error = %v", err)
	}

	// Check that content contains newlines (indented)
	contentStr := string(content)
	if !strings.Contains(contentStr, "\n") {
		t.Error("expected indented JSON with newlines")
	}

	// Verify key fields are present and readable
	if !strings.Contains(contentStr, "\"current_task_id\": \"TASK3\"") {
		t.Error("expected current_task_id to be readable in JSON")
	}
	if !strings.Contains(contentStr, "\"current_step\": 7") {
		t.Error("expected current_step to be readable in JSON")
	}
	if !strings.Contains(contentStr, "\"completed_task_ids\"") {
		t.Error("expected completed_task_ids field in JSON")
	}
	if !strings.Contains(contentStr, "\"tasks_dir\": \"docs/tasks\"") {
		t.Error("expected tasks_dir field in JSON")
	}
}

func TestNewManagerInDir_SavesDirectlyInDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManagerInDir(dir)

	s := NewState("tasks", "tasks/PRD.md", 10)
	if err := mgr.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// State file should be at dir/state.json (not dir/.snap/state.json).
	statePath := filepath.Join(dir, StateFile)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Errorf("state.json not found at %s", statePath)
	}

	// .snap subdirectory should NOT be created.
	snapSubDir := filepath.Join(dir, StateDir)
	if _, err := os.Stat(filepath.Join(snapSubDir, StateFile)); err == nil {
		t.Error("state.json should not exist inside .snap subdirectory")
	}

	// Load should work.
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.TasksDir != "tasks" {
		t.Errorf("TasksDir = %s, want tasks", loaded.TasksDir)
	}
}

func TestNewManagerInDir_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManagerInDir(dir)

	s := NewState("tasks", "tasks/PRD.md", 10)
	s.CurrentTaskID = "TASK2"
	s.CurrentStep = 5
	s.CompletedTaskIDs = []string{"TASK1"}

	if err := mgr.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.CurrentTaskID != "TASK2" {
		t.Errorf("CurrentTaskID = %s, want TASK2", loaded.CurrentTaskID)
	}
	if loaded.CurrentStep != 5 {
		t.Errorf("CurrentStep = %d, want 5", loaded.CurrentStep)
	}
	if len(loaded.CompletedTaskIDs) != 1 || loaded.CompletedTaskIDs[0] != "TASK1" {
		t.Errorf("CompletedTaskIDs = %v, want [TASK1]", loaded.CompletedTaskIDs)
	}
}

func TestNewManagerInDir_ResetAndExists(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManagerInDir(dir)

	if mgr.Exists() {
		t.Error("should not exist initially")
	}

	s := NewState("tasks", "tasks/PRD.md", 10)
	if err := mgr.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !mgr.Exists() {
		t.Error("should exist after save")
	}

	if err := mgr.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if mgr.Exists() {
		t.Error("should not exist after reset")
	}
}

func TestManager_SerializedFieldPresence(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManagerWithDir(tmpDir)

	state := NewState("docs/tasks", "docs/tasks/PRD.md", 9)
	state.CurrentTaskID = "TASK1"
	state.CompletedTaskIDs = []string{"TASK0"}
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	statePath := filepath.Join(tmpDir, StateDir, StateFile)
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("JSON parse error = %v", err)
	}

	requiredFields := []string{"tasks_dir", "current_step", "total_steps", "completed_task_ids", "session_id", "last_updated", "prd_path"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q in serialized JSON", field)
		}
	}
}
