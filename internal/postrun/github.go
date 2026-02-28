package postrun

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

// DefaultBranch returns the default branch name of the GitHub repository.
// Runs: gh repo view --json defaultBranchRef -q .defaultBranchRef.name.
func DefaultBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GHError{Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// prViewResult represents the JSON output from gh pr view.
type prViewResult struct {
	State string `json:"state"`
	URL   string `json:"url"`
}

// PRExists checks if a PR already exists for the current branch.
// Returns (exists, url, error). Exit code 1 from gh means no PR exists (not an error).
func PRExists(ctx context.Context) (exists bool, prURL string, err error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "state,url")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Exit code 1 = no PR for this branch — not an error
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 1 {
			return false, "", nil
		}
		return false, "", &GHError{Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}

	var result prViewResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return false, "", err
	}
	return true, result.URL, nil
}

// CreatePR creates a new pull request with the given title and body.
// Returns the PR URL.
func CreatePR(ctx context.Context, title, body string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", "--title", title, "--body", body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GHError{Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// GHError wraps a gh CLI failure with stderr output.
type GHError struct {
	Stderr string
	Err    error
}

func (e *GHError) Error() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	return e.Err.Error()
}

func (e *GHError) Unwrap() error {
	return e.Err
}
