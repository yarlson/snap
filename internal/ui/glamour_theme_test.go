package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapTheme(t *testing.T) {
	theme := snapTheme()

	// Test that theme has been initialized
	assert.NotNil(t, theme, "theme should not be nil")

	// Test color customizations
	// Code should use info cyan
	assert.NotNil(t, theme.Code.Color)
	assert.Equal(t, "#5fd7d7", *theme.Code.Color, "code should use info cyan")

	// Generic Heading should use primary teal (fallback)
	assert.NotNil(t, theme.Heading.Color)
	assert.Equal(t, "#00af87", *theme.Heading.Color, "generic heading should use primary teal")

	// H1 should use primary teal
	assert.NotNil(t, theme.H1.Color)
	assert.Equal(t, "#00af87", *theme.H1.Color, "H1 should use primary teal")
	assert.NotNil(t, theme.H1.Bold)
	assert.True(t, *theme.H1.Bold, "H1 should be bold")

	// H2 should use primary teal
	assert.NotNil(t, theme.H2.Color)
	assert.Equal(t, "#00af87", *theme.H2.Color, "H2 should use primary teal")
	assert.NotNil(t, theme.H2.Bold)
	assert.True(t, *theme.H2.Bold, "H2 should be bold")

	// H3 should use secondary sky blue
	assert.NotNil(t, theme.H3.Color)
	assert.Equal(t, "#5fafff", *theme.H3.Color, "H3 should use secondary sky blue")

	// Links should use tool purple
	assert.NotNil(t, theme.Link.Color)
	assert.Equal(t, "#af87ff", *theme.Link.Color, "links should use tool purple")

	// Lists should use 2-char indentation to match tool indentation
	assert.Equal(t, uint(2), theme.List.LevelIndent, "lists should use 2-char indentation")

	// Code blocks should have no extra margins
	assert.NotNil(t, theme.CodeBlock.Margin)
	assert.Equal(t, uint(0), *theme.CodeBlock.Margin, "code blocks should have no extra margins")

	// Block quotes should use tertiary gray
	assert.NotNil(t, theme.BlockQuote.Color)
	assert.Equal(t, "#8a8a8a", *theme.BlockQuote.Color, "block quotes should use tertiary gray")

	// Strikethrough should use dim color
	assert.NotNil(t, theme.Strikethrough.Color)
	assert.Equal(t, "#585858", *theme.Strikethrough.Color, "strikethrough should use dim color")
}

func TestHelperFunctions(t *testing.T) {
	t.Run("stringPtr", func(t *testing.T) {
		s := "test"
		ptr := stringPtr(s)
		assert.NotNil(t, ptr)
		assert.Equal(t, s, *ptr)
	})

	t.Run("boolPtr", func(t *testing.T) {
		b := true
		ptr := boolPtr(b)
		assert.NotNil(t, ptr)
		assert.Equal(t, b, *ptr)
	})

	t.Run("uintPtr", func(t *testing.T) {
		u := uint(42)
		ptr := uintPtr(u)
		assert.NotNil(t, ptr)
		assert.Equal(t, u, *ptr)
	})
}

func TestSnapThemeIntegration(t *testing.T) {
	// Test that the theme can be used to create a renderer without errors
	renderer := NewMarkdownRenderer()
	assert.NotNil(t, renderer, "renderer should be created successfully")

	// Test rendering with custom theme
	testCases := []struct {
		name     string
		input    string
		expected string // Content that should be present (after stripping colors)
	}{
		{
			name:     "heading with teal color",
			input:    "# Test Heading",
			expected: "Test Heading",
		},
		{
			name:     "code with cyan color",
			input:    "`code`",
			expected: "code",
		},
		{
			name:     "link with purple color",
			input:    "[link](https://example.com)",
			expected: "link",
		},
		{
			name:     "list with proper indentation",
			input:    "- Item 1\n- Item 2",
			expected: "Item 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := renderer.Render(tc.input)
			assert.NoError(t, err, "rendering should not error")
			stripped := StripColors(result)
			assert.Contains(t, stripped, tc.expected, "rendered output should contain expected text")
		})
	}
}
