package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains []string // Strings that should be in the output
	}{
		{
			name:     "plain text",
			input:    "Hello, world!",
			contains: []string{"Hello, world!"},
		},
		{
			name:     "heading",
			input:    "# Main Title\n\nSome content",
			contains: []string{"Main Title", "Some content"},
		},
		{
			name:     "bold text",
			input:    "This is **bold** text",
			contains: []string{"bold"},
		},
		{
			name:     "code block",
			input:    "```go\nfunc main() {}\n```",
			contains: []string{"func main()"},
		},
		{
			name:     "inline code",
			input:    "Use `fmt.Println()` to print",
			contains: []string{"fmt.Println()"},
		},
		{
			name:     "list items",
			input:    "- Item 1\n- Item 2\n- Item 3",
			contains: []string{"Item 1", "Item 2", "Item 3"},
		},
		{
			name:  "empty string",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMarkdownRenderer()
			result, err := renderer.Render(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Strip ANSI codes before checking content (glamour adds color codes)
			stripped := StripColors(result)
			for _, want := range tt.contains {
				assert.Contains(t, stripped, want)
			}
		})
	}
}

func TestMarkdownRenderer_RenderFallback(t *testing.T) {
	renderer := NewMarkdownRenderer()

	// Test that we can render various markdown without panicking
	inputs := []string{
		"# Title\n\nParagraph with **bold** and *italic*",
		"```\ncode\n```",
		"[link](https://example.com)",
		"> Quote",
		"1. First\n2. Second",
	}

	for _, input := range inputs {
		result, err := renderer.Render(input)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	}
}

func TestMarkdownRenderer_PreservesContent(t *testing.T) {
	renderer := NewMarkdownRenderer()

	input := "Line 1\nLine 2\nLine 3"
	result, err := renderer.Render(input)

	require.NoError(t, err)
	// Check that all lines are present (formatting may change but content should remain)
	// Strip ANSI codes before checking (glamour adds color codes)
	stripped := StripColors(result)
	assert.True(t, strings.Contains(stripped, "Line 1") &&
		strings.Contains(stripped, "Line 2") &&
		strings.Contains(stripped, "Line 3"),
		"rendered output should contain all input lines")
}

func TestMarkdownRenderer_NilRenderer(t *testing.T) {
	// Test fallback behavior when renderer initialization fails
	renderer := &MarkdownRenderer{renderer: nil}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "plain text with nil renderer",
			input: "Hello, world!",
		},
		{
			name:  "markdown with nil renderer",
			input: "**bold** and *italic*",
		},
		{
			name:  "code block with nil renderer",
			input: "```go\nfunc main() {}\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.Render(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.input, result, "should return raw text when renderer is nil")
		})
	}
}

func TestMarkdownRenderer_EmptyString(t *testing.T) {
	renderer := NewMarkdownRenderer()

	result, err := renderer.Render("")
	require.NoError(t, err)
	assert.Empty(t, result, "empty input should return empty output")
}

func TestMarkdownRenderer_VeryLongContent(t *testing.T) {
	renderer := NewMarkdownRenderer()

	// Test with very long content to ensure no buffer issues
	var builder strings.Builder
	for range 1000 {
		builder.WriteString("Line ")
		builder.WriteString(strings.Repeat("x", 100))
		builder.WriteString("\n")
	}
	input := builder.String()

	result, err := renderer.Render(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result, "should handle very long content")
}
