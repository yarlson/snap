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

// prCheckResult represents a single check from gh pr checks --json.
type prCheckResult struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Conclusion string `json:"conclusion"`
}

// runCheckResult represents a single run from gh run list --json.
type runCheckResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// CheckStatus returns the current CI check results.
// If hasPR is true, uses "gh pr checks --json"; otherwise uses "gh run list --json" scoped to the given branch.
func CheckStatus(ctx context.Context, hasPR bool, branch string) ([]CheckResult, error) {
	if hasPR {
		return checkStatusPR(ctx)
	}
	return checkStatusRun(ctx, branch)
}

func checkStatusPR(ctx context.Context) ([]CheckResult, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "checks", "--json", "name,state,conclusion")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, &GHError{Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}

	var raw []prCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	results := make([]CheckResult, len(raw))
	for i, r := range raw {
		results[i] = CheckResult{
			Name:       r.Name,
			Status:     normalizePRCheckStatus(r.State, r.Conclusion),
			Conclusion: r.Conclusion,
		}
	}
	return results, nil
}

func checkStatusRun(ctx context.Context, branch string) ([]CheckResult, error) {
	cmd := exec.CommandContext(ctx, "gh", "run", "list", "--branch", branch, "--json", "name,status,conclusion", "--limit", "1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, &GHError{Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}

	var raw []runCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	results := make([]CheckResult, len(raw))
	for i, r := range raw {
		results[i] = CheckResult{
			Name:       r.Name,
			Status:     normalizeRunStatus(r.Status, r.Conclusion),
			Conclusion: r.Conclusion,
		}
	}
	return results, nil
}

// normalizePRCheckStatus converts gh pr checks states to our standard statuses.
// gh pr checks states: PENDING, SUCCESS, FAILURE, ERROR, CANCELLED, etc.
func normalizePRCheckStatus(state, _ string) string {
	switch strings.ToUpper(state) {
	case "SUCCESS":
		return "passed"
	case "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STALE", "STARTUP_FAILURE":
		return "failed"
	default:
		return "pending"
	}
}

// normalizeRunStatus converts gh run list statuses to our standard statuses.
// gh run list statuses: completed, in_progress, queued, waiting, requested, pending.
func normalizeRunStatus(status, conclusion string) string {
	switch strings.ToLower(status) {
	case "completed":
		switch strings.ToLower(conclusion) {
		case "success":
			return "passed"
		default:
			return "failed"
		}
	case "in_progress":
		return "running"
	default:
		return "pending"
	}
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
