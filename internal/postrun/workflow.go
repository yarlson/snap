package postrun

import (
	"os"
	"path/filepath"
	"strings"
)

// HasRelevantWorkflows scans .github/workflows/*.yml and *.yaml files for
// push or pull_request trigger strings. Returns true if at least one workflow
// has a relevant trigger. Returns false (not error) if the workflows directory
// doesn't exist or is empty.
func HasRelevantWorkflows(repoRoot string) (bool, error) {
	wfDir := filepath.Join(repoRoot, ".github", "workflows")

	entries, err := os.ReadDir(wfDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(wfDir, name))
		if err != nil {
			continue // skip unreadable files
		}

		if hasRelevantTrigger(string(data)) {
			return true, nil
		}
	}

	return false, nil
}

// hasRelevantTrigger does a conservative line-by-line scan for push or
// pull_request triggers in a workflow file's content.
func hasRelevantTrigger(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Match trigger declarations: "on: push", "on: [push, pull_request]",
		// "push:" (as a map key under on:), "pull_request:" (same)
		if matchesTriggerLine(trimmed) {
			return true
		}
	}
	return false
}

// matchesTriggerLine checks if a single line indicates a push or pull_request trigger.
func matchesTriggerLine(line string) bool {
	// "on: push", "on: pull_request", "on: [push, ...]"
	if strings.HasPrefix(line, "on:") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "on:"))
		if containsTriggerWord(rest) {
			return true
		}
	}

	// Map-style trigger: "push:" or "pull_request:" as a key (indented under on:)
	if line == "push:" || strings.HasPrefix(line, "push:") {
		return true
	}
	if line == "pull_request:" || strings.HasPrefix(line, "pull_request:") {
		return true
	}

	return false
}

// containsTriggerWord checks if the text contains "push" or "pull_request" as trigger words.
func containsTriggerWord(text string) bool {
	// Handle inline list: [push, pull_request]
	// Handle single value: push
	for _, word := range []string{"push", "pull_request"} {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}
