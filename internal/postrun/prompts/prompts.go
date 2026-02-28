package prompts

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

//go:embed pr.md
var prTmpl string

// PRData holds template parameters for the PR generation prompt.
type PRData struct {
	PRDContent string
	DiffStat   string
}

// PR renders the PR generation prompt template with the given data.
func PR(data PRData) (string, error) {
	tmpl, err := template.New("pr").Parse(prTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
