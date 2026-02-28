package postrun

import (
	"fmt"
	"strings"
)

// parsePROutput extracts the PR title and body from LLM output.
// Expected format: title on first line, blank line, then body.
func parsePROutput(output string) (title, body string, err error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", "", fmt.Errorf("empty LLM output for PR generation")
	}

	// Split on first blank line (double newline)
	parts := strings.SplitN(trimmed, "\n\n", 2)
	title = strings.TrimSpace(parts[0])
	if title == "" {
		return "", "", fmt.Errorf("empty title in LLM output")
	}

	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}

	return title, body, nil
}
