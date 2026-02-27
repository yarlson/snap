package session

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
