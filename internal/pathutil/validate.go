package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath checks if a path is safe to use (no injection characters, no traversal outside project).
func ValidatePath(path string) error {
	// Check for injection characters
	if strings.Contains(path, "\n") || strings.Contains(path, "\r") {
		return fmt.Errorf("path contains invalid characters (newline)")
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Ensure path is within current working directory (prevent path traversal)
	if !strings.HasPrefix(absPath, cwd) {
		return fmt.Errorf("path must be within project directory (cwd: %s, path: %s)", cwd, absPath)
	}

	return nil
}

// ResolvePRDPath resolves the PRD path with a default from tasksDir.
func ResolvePRDPath(tasksDir, prdPath string) string {
	if prdPath == "" {
		return filepath.Join(tasksDir, "PRD.md")
	}
	return prdPath
}

// CheckPathExists checks if a path exists and returns a warning message if not.
func CheckPathExists(path string) (exists bool, warning string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, fmt.Sprintf("Warning: file does not exist: %s", path)
	} else if err != nil {
		return false, fmt.Sprintf("Warning: cannot access file: %s (%v)", path, err)
	}
	return true, ""
}
