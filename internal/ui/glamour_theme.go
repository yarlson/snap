package ui

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// snapTheme returns a custom glamour theme matching snap's design system palette.
func snapTheme() ansi.StyleConfig {
	// Start with the dark style as a base
	theme := styles.DarkStyleConfig

	// Override colors to match snap's design tokens
	// Using hex colors from the design system (Appendix A)

	// Code and inline code - Info cyan (#5fd7d7, ANSI 80)
	theme.Code.Color = stringPtr("#5fd7d7")
	theme.CodeBlock.Chroma.Text.Color = stringPtr("#5fd7d7")
	theme.CodeBlock.Chroma.Background.BackgroundColor = stringPtr("#1a1a1a")

	// Headings - Primary teal (#00af87, ANSI 37)
	theme.Heading.Color = stringPtr("#00af87") // Generic heading fallback
	theme.H1.Color = stringPtr("#00af87")
	theme.H1.Bold = boolPtr(true)
	theme.H2.Color = stringPtr("#00af87")
	theme.H2.Bold = boolPtr(true)
	theme.H3.Color = stringPtr("#5fafff") // Secondary sky blue for H3
	theme.H3.Bold = boolPtr(true)
	theme.H4.Color = stringPtr("#5fafff")
	theme.H5.Color = stringPtr("#5fafff")
	theme.H6.Color = stringPtr("#5fafff")

	// Links - Tool purple (#af87ff, ANSI 141)
	theme.Link.Color = stringPtr("#af87ff")
	theme.Link.Underline = boolPtr(true)

	// Lists - Match tool indentation (2 chars)
	theme.List.LevelIndent = 2

	// Emphasis
	theme.Emph.Italic = boolPtr(true)
	theme.Strong.Bold = boolPtr(true)

	// Code blocks - No extra margins (let terminal handle spacing)
	theme.CodeBlock.Margin = uintPtr(0)

	// Block quotes - Tertiary gray (#8a8a8a, ANSI 245)
	theme.BlockQuote.Indent = uintPtr(2)
	theme.BlockQuote.Color = stringPtr("#8a8a8a")

	// Tables - Keep clean with minimal styling
	theme.Table.CenterSeparator = stringPtr("┼")
	theme.Table.ColumnSeparator = stringPtr("│")
	theme.Table.RowSeparator = stringPtr("─")

	// Strikethrough - Dim (#585858, ANSI 240)
	theme.Strikethrough.Color = stringPtr("#585858")
	theme.Strikethrough.CrossedOut = boolPtr(true)

	return theme
}

// Helper functions to create pointers (glamour uses pointer fields for optional values).

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func uintPtr(u uint) *uint {
	return &u
}
