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

// RenderAnalyzeTasksPrompt renders the combined task analysis prompt (create + assess + refine).
func RenderAnalyzeTasksPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/analyze-tasks.md", promptData{TasksDir: tasksDir})
	if err != nil {
		return "", err
	}
	return prependPreamble(prompt)
}

// RenderGenerateTasksPrompt renders the task generation prompt (TASKS.md + TASK<N>.md subagents).
func RenderGenerateTasksPrompt(tasksDir string) (string, error) {
	prompt, err := renderTemplate("prompts/generate-tasks.md", promptData{TasksDir: tasksDir})
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
