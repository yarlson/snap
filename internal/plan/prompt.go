package plan

import (
	"bytes"
	"embed"
	"strings"
	"text/template"
)

//go:embed prompts/*.md.tmpl
var promptFS embed.FS

// promptData holds template parameters for plan prompt rendering.
type promptData struct {
	TasksDir string
	Brief    string
}

// RenderRequirementsPrompt returns the Phase 1 requirements-gathering prompt.
func RenderRequirementsPrompt() string {
	return `You are helping a user plan a feature. Ask questions to understand:
- What problem they are solving
- Who the users are
- What the scope and constraints are
- What success looks like

Ask one or two focused questions at a time. Be conversational and specific.
The user will type /done when they are finished providing requirements.`
}

// RenderPRDPrompt renders the PRD generation prompt with the given tasks directory and optional brief.
func RenderPRDPrompt(tasksDir, brief string) (string, error) {
	return renderTemplate("prompts/prd.md.tmpl", promptData{TasksDir: tasksDir, Brief: brief})
}

// RenderTechnologyPrompt renders the technology plan generation prompt.
func RenderTechnologyPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/technology.md.tmpl", promptData{TasksDir: tasksDir})
}

// RenderDesignPrompt renders the design spec generation prompt.
func RenderDesignPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/design.md.tmpl", promptData{TasksDir: tasksDir})
}

// RenderTasksPrompt renders the task split generation prompt.
func RenderTasksPrompt(tasksDir string) (string, error) {
	return renderTemplate("prompts/slices.md.tmpl", promptData{TasksDir: tasksDir})
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
