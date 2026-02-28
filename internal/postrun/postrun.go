package postrun

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/postrun/prompts"
	"github.com/yarlson/snap/internal/ui"
)

var prNumberRe = regexp.MustCompile(`/pull/(\d+)`)

// Executor runs an LLM call. Matches the workflow.Executor interface.
type Executor interface {
	Run(ctx context.Context, w io.Writer, mt model.Type, args ...string) error
}

// Config holds configuration for the post-run step.
type Config struct {
	Output    io.Writer
	Executor  Executor // LLM executor for PR generation (nil = skip LLM, use default title)
	RemoteURL string   // Pre-detected remote URL (empty = no remote)
	IsGitHub  bool     // Pre-detected GitHub flag
	PRDPath   string   // Path to PRD.md for PR body context
	TasksDir  string   // Tasks directory
}

// Run executes the post-run step: push to remote, create PR if on GitHub.
func Run(ctx context.Context, cfg Config) error {
	if cfg.RemoteURL == "" {
		fmt.Fprint(cfg.Output, ui.Info("No remote configured, skipping push"))
		return nil
	}

	// Push to origin
	fmt.Fprint(cfg.Output, ui.Step("Pushing to origin..."))
	pushStart := time.Now()

	if err := Push(ctx); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	branch, err := CurrentBranch(ctx)
	if err != nil {
		branch = "unknown"
	}
	fmt.Fprint(cfg.Output, ui.StepComplete(fmt.Sprintf("Pushed to origin/%s", branch), time.Since(pushStart)))

	if !cfg.IsGitHub {
		fmt.Fprint(cfg.Output, ui.Info("Non-GitHub remote, skipping PR and CI"))
		return nil
	}

	// PR creation flow
	return createPRFlow(ctx, cfg, branch)
}

func createPRFlow(ctx context.Context, cfg Config, currentBranch string) error {
	// Detached HEAD — skip PR creation silently
	if currentBranch == "" || currentBranch == "unknown" {
		return nil
	}

	// Get default branch
	defaultBranch, err := DefaultBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect default branch: %w", err)
	}

	// On default branch — skip PR creation
	if currentBranch == defaultBranch {
		fmt.Fprint(cfg.Output, ui.Info("On default branch, skipping PR creation"))
		return nil
	}

	// Check if PR already exists
	exists, existingURL, err := PRExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for existing PR: %w", err)
	}
	if exists {
		fmt.Fprint(cfg.Output, ui.Info(fmt.Sprintf("PR already exists: %s", existingURL)))
		return nil
	}

	// Generate PR title and body via LLM
	fmt.Fprint(cfg.Output, ui.Step("Creating pull request..."))
	prStart := time.Now()

	title, body := generatePR(ctx, cfg, defaultBranch)

	// Create PR
	prURL, err := CreatePR(ctx, title, body)
	if err != nil {
		fmt.Fprint(cfg.Output, ui.Error(fmt.Sprintf("PR creation failed: %s", err)))
		return fmt.Errorf("PR creation failed: %w", err)
	}

	// Extract PR number from URL
	prNumber := extractPRNumber(prURL)
	fmt.Fprint(cfg.Output, ui.StepComplete(fmt.Sprintf("PR %s created: %s", prNumber, prURL), time.Since(prStart)))

	// TASK3 extends from here: CI monitoring
	return nil
}

func generatePR(ctx context.Context, cfg Config, defaultBranch string) (title, body string) {
	if cfg.Executor == nil {
		return "Update", ""
	}

	// Read PRD content
	var prdContent string
	if cfg.PRDPath != "" {
		data, readErr := os.ReadFile(cfg.PRDPath)
		if readErr == nil {
			prdContent = string(data)
		}
	}

	// Get diff stat (best-effort, ignore errors).
	diffStat, _ := DiffStat(ctx, defaultBranch) //nolint:errcheck // best-effort diff stat

	// Render prompt
	prompt, err := prompts.PR(prompts.PRData{
		PRDContent: prdContent,
		DiffStat:   diffStat,
	})
	if err != nil {
		return "Update", ""
	}

	// Call LLM
	var buf strings.Builder
	if err := cfg.Executor.Run(ctx, &buf, model.Fast, prompt); err != nil {
		return "Update", ""
	}

	// Parse LLM output
	title, body, parseErr := parsePROutput(buf.String())
	if parseErr != nil {
		return "Update", ""
	}

	return title, body
}

// extractPRNumber extracts "#N" from a GitHub PR URL like "https://github.com/user/repo/pull/42".
func extractPRNumber(url string) string {
	matches := prNumberRe.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return "#" + matches[1]
	}
	return ""
}
