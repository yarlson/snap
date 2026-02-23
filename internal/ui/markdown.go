package ui

import (
	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer renders markdown text with styling.
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
}

// NewMarkdownRenderer creates a new markdown renderer with snap's custom theme.
func NewMarkdownRenderer() *MarkdownRenderer {
	// Create renderer with snap's custom theme matching the design system palette
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(snapTheme()),
		glamour.WithWordWrap(0), // No wrapping, let terminal handle it
	)
	// If renderer creation fails, fallback to nil which will return raw text.
	// We don't log this error because:
	// 1. Output is still readable (raw markdown is user-friendly)
	// 2. Logging would clutter the streamed output
	// 3. Failure is rare (only in unusual terminal environments)
	if err != nil {
		renderer = nil
	}

	return &MarkdownRenderer{
		renderer: renderer,
	}
}

// Render converts markdown text to styled terminal output.
// If rendering fails, returns the original text unchanged.
func (m *MarkdownRenderer) Render(markdown string) (string, error) {
	// Handle empty input
	if markdown == "" {
		return "", nil
	}

	// If renderer is nil (initialization failed), return raw text
	if m.renderer == nil {
		return markdown, nil
	}

	// Render markdown with glamour
	result, err := m.renderer.Render(markdown)
	if err != nil {
		// On error, fall back to raw text (preserve streaming)
		return markdown, err
	}

	return result, nil
}
