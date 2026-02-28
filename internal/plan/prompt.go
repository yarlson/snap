package plan

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/*.md
var promptFS embed.FS

// promptData holds template parameters for plan prompt rendering.
type promptData struct {
	TasksDir string
	Brief    string
}

// RenderPrinciplesPreamble renders the shared engineering principles preamble.
func RenderPrinciplesPreamble() (string, error) {
	return renderTemplate("prompts/principles.md", promptData{})
}

// prependPreamble renders the principles preamble and prepends it to the given prompt.
func prependPreamble(prompt string) (string, error) {
	preamble, err := RenderPrinciplesPreamble()
	if err != nil {
		return "", fmt.Errorf("render principles preamble: %w", err)
	}
	return preamble + "\n\n" + prompt, nil
}

// RenderRequirementsPrompt returns the Phase 1 requirements-gathering prompt.
func RenderRequirementsPrompt() (string, error) {
	return renderTemplate("prompts/requirements.md", promptData{})
}

// RenderPRDPrompt renders the PRD generation prompt with the given tasks directory and optional brief.
func RenderPRDPrompt(tasksDir, brief string) (string, error) {
	prompt, err := renderTemplate("prompts/prd.md", promptData{TasksDir: tasksDir, Brief: brief})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

// RenderTechnologyPrompt renders the technology plan generation prompt.
func RenderTechnologyPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/technology.md", promptData{TasksDir: tasksDir})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

// RenderDesignPrompt renders the design spec generation prompt.
func RenderDesignPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/design.md", promptData{TasksDir: tasksDir})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

// RenderCreateTasksPrompt renders the initial task list creation prompt.
func RenderCreateTasksPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/create-tasks.md", promptData{TasksDir: tasksDir})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

// RenderAssessTasksPrompt renders the task assessment/scoring prompt.
// It operates on conversation context and takes no tasksDir parameter.
func RenderAssessTasksPrompt() (string, error) {
	return renderTemplate("prompts/assess-tasks.md", promptData{})
}

// RenderMergeTasksPrompt renders the task merge/split/fix prompt.
// It operates on conversation context and takes no tasksDir parameter.
func RenderMergeTasksPrompt() (string, error) {
	return renderTemplate("prompts/merge-tasks.md", promptData{})
}

// RenderGenerateTaskSummaryPrompt renders the TASKS.md generation prompt.
func RenderGenerateTaskSummaryPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/generate-task-summary.md", promptData{TasksDir: tasksDir})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

func renderTemplate(name string, data promptData) (string, error) {
	content, err := promptFS.ReadFile(name)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}
