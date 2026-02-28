package plan

import (
	"bytes"
	"embed"
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

// RenderRequirementsPrompt returns the Phase 1 requirements-gathering prompt.
func RenderRequirementsPrompt() (string, error) {
	return renderTemplate("prompts/requirements.md", promptData{})
}

// RenderPRDPrompt renders the PRD generation prompt with the given tasks directory and optional brief.
func RenderPRDPrompt(tasksDir, brief string) (string, error) {
	return renderTemplate("prompts/prd.md", promptData{TasksDir: tasksDir, Brief: brief})
}

// RenderTechnologyPrompt renders the technology plan generation prompt.
func RenderTechnologyPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/technology.md", promptData{TasksDir: tasksDir})
}

// RenderDesignPrompt renders the design spec generation prompt.
func RenderDesignPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/design.md", promptData{TasksDir: tasksDir})
}

// RenderTasksPrompt renders the task split generation prompt.
func RenderTasksPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/slices.md", promptData{TasksDir: tasksDir})
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
