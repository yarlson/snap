package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// StateDir is the directory where state files are stored.
	StateDir = ".snap"

	// StateFile is the name of the state file.
	StateFile = "state.json"
)

// Manager provides persistent state management for workflow resumption.
type Manager struct {
	stateDir  string
	statePath string
}

// NewManager creates a new state manager.
func NewManager() *Manager {
	stateDir := StateDir
	statePath := filepath.Join(stateDir, StateFile)

	return &Manager{
		stateDir:  stateDir,
		statePath: statePath,
	}
}

// NewManagerWithDir creates a new state manager with a custom directory.
// Used for testing with temporary directories. State lives at dir/.snap/state.json.
func NewManagerWithDir(dir string) *Manager {
	stateDir := filepath.Join(dir, StateDir)
	statePath := filepath.Join(stateDir, StateFile)

	return &Manager{
		stateDir:  stateDir,
		statePath: statePath,
	}
}

// NewManagerInDir creates a state manager that stores state.json directly in dir.
// Used for session-scoped state where state.json lives at the session root
// (e.g., .snap/sessions/<name>/state.json).
func NewManagerInDir(dir string) *Manager {
	return &Manager{
		stateDir:  dir,
		statePath: filepath.Join(dir, StateFile),
	}
}

// Load reads state from disk.
// Returns nil state with nil error if file doesn't exist (no error occurred, just no state).
func (m *Manager) Load() (*State, error) {
	// Check if state file exists
	if !m.Exists() {
		//nolint:nilnil // Returning nil state with nil error is intentional - no state exists, no error occurred
		return nil, nil
	}

	// Read state file
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	// Parse state
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}

	// Validate state
	if !s.IsValid() {
		return nil, errors.New("invalid state: failed validation")
	}

	return &s, nil
}

// Save atomically writes state to disk using atomic rename pattern.
func (m *Manager) Save(state *State) error {
	if state == nil {
		return errors.New("cannot save nil state")
	}

	// Validate state before saving
	if !state.IsValid() {
		return errors.New("cannot save invalid state")
	}

	// Ensure state directory exists
	if err := os.MkdirAll(m.stateDir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Write to temp file first (atomic write pattern)
	// Use PID + timestamp to avoid race conditions with PID reuse
	tmpPath := fmt.Sprintf("%s.tmp.%d.%d", m.statePath, os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}

	// Atomic rename (POSIX guarantees atomicity)
	if err := os.Rename(tmpPath, m.statePath); err != nil {
		// Cleanup temp file on failure
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename: %w", err)
	}

	// Ensure .snap/.gitignore exists to prevent accidental commits
	if err := m.ensureGitignore(); err != nil {
		// Non-fatal - state is already saved
		return fmt.Errorf("warning: failed to create .gitignore: %w", err)
	}

	return nil
}

// Reset deletes the state file.
func (m *Manager) Reset() error {
	if !m.Exists() {
		return nil // Nothing to delete
	}

	if err := os.Remove(m.statePath); err != nil {
		return fmt.Errorf("remove state file: %w", err)
	}

	return nil
}

// Exists checks if state file exists.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.statePath)
	return err == nil
}

// ensureGitignore creates .snap/.gitignore if it doesn't exist.
func (m *Manager) ensureGitignore() error {
	gitignorePath := filepath.Join(m.stateDir, ".gitignore")

	// Check if already exists
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	}

	// Create .gitignore with pattern to ignore all files except itself
	content := "*\n!.gitignore\n"
	if err := os.WriteFile(gitignorePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	return nil
}
