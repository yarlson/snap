package prompts

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

//go:embed implement.md
var implementTmpl string

//go:embed ensure_completeness.md
var ensureCompletenessTmpl string

//go:embed lint_and_test.md
var lintAndTest string

//go:embed code_review.md
var codeReview string

//go:embed apply_fixes.md
var applyFixes string

//go:embed update_docs.md
var updateDocs string

//go:embed commit.md
var commit string

//go:embed memory_update.md
var memoryUpdate string

//go:embed task_summary.md
var taskSummaryTmpl string

// ImplementData holds template parameters for the implement prompt.
type ImplementData struct {
	PRDPath  string
	TaskPath string // empty when auto-selecting
	TaskID   string // empty when auto-selecting
}

// Implement renders the implementation prompt template with the given data.
func Implement(data ImplementData) (string, error) {
	tmpl, err := template.New("implement").Parse(implementTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// EnsureCompletenessData holds template parameters for the ensure-completeness prompt.
type EnsureCompletenessData struct {
	TaskPath string
	TaskID   string
}

// EnsureCompleteness renders the ensure-completeness prompt template with the given data.
func EnsureCompleteness(data EnsureCompletenessData) (string, error) {
	tmpl, err := template.New("ensure_completeness").Parse(ensureCompletenessTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// LintAndTest returns the lint-and-test prompt.
func LintAndTest() string { return strings.TrimSpace(lintAndTest) }

// CodeReview returns the full embedded code review prompt.
func CodeReview() string { return strings.TrimSpace(codeReview) }

// ApplyFixes returns the apply-fixes prompt.
func ApplyFixes() string { return strings.TrimSpace(applyFixes) }

// UpdateDocs returns the update-docs prompt.
func UpdateDocs() string { return strings.TrimSpace(updateDocs) }

// Commit returns the commit prompt.
func Commit() string { return strings.TrimSpace(commit) }

// MemoryUpdate returns the full embedded memory vault update prompt.
func MemoryUpdate() string { return strings.TrimSpace(memoryUpdate) }

// TaskSummaryData holds template parameters for the task-summary prompt.
type TaskSummaryData struct {
	TaskContent string
}

// TaskSummary renders the task-summary prompt template with the given data.
func TaskSummary(data TaskSummaryData) (string, error) {
	tmpl, err := template.New("task_summary").Parse(taskSummaryTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
