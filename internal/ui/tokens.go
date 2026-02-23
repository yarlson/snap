package ui

import "strings"

// ColorToken represents a semantic color role in the UI design system.
type ColorToken string

const (
	// Hierarchy tokens.
	ColorPrimary   ColorToken = "primary"   // Major sections (headers)
	ColorSecondary ColorToken = "secondary" // Steps/subsections
	ColorTertiary  ColorToken = "tertiary"  // Supporting text

	// Status tokens.
	ColorSuccess ColorToken = "success" // Completed actions
	ColorError   ColorToken = "error"   // Failures
	ColorWarning ColorToken = "warning" // Warnings/interrupts
	ColorInfo    ColorToken = "info"    // Neutral information

	// Special tokens.
	ColorTool      ColorToken = "tool"      // Tool invocations
	ColorCelebrate ColorToken = "celebrate" // Completions
	ColorDim       ColorToken = "dim"       // Low-priority output
)

// StyleToken represents a font weight or text decoration.
type StyleToken string

const (
	WeightNormal StyleToken = "normal" // Regular text
	WeightBold   StyleToken = "bold"   // Emphasis
	WeightDim    StyleToken = "dim"    // De-emphasis
)

// Spacing constants define vertical spacing in lines.
const (
	SpaceNone = 0 // No space
	SpaceXS   = 1 // 1 line
	SpaceSM   = 2 // 2 lines
	SpaceMD   = 3 // 3 lines
	SpaceLG   = 4 // 4 lines
	SpaceXL   = 6 // 6 lines
)

// Box drawing constants.
const (
	BoxWidth        = 70 // Main box width
	BoxPaddingLeft  = 2  // Chars of padding inside box
	BoxPaddingRight = 2

	IndentStep   = 0 // Steps flush left
	IndentTool   = 1 // Tools indented 1 char
	IndentResult = 1 // Results aligned with tools

	SeparatorWidth = 70 // Match box width
)

// ANSI color mappings for each ColorToken (256-color palette).
const (
	ansiPrimary   = "\033[38;5;37m"  // #00af87 (teal)
	ansiSecondary = "\033[38;5;75m"  // #5fafff (sky blue)
	ansiTertiary  = "\033[38;5;245m" // #8a8a8a (medium gray)
	ansiSuccess   = "\033[38;5;35m"  // #00af5f (forest green)
	ansiError     = "\033[38;5;167m" // #d75f5f (warm red)
	ansiWarning   = "\033[38;5;214m" // #ffaf00 (amber)
	ansiInfo      = "\033[38;5;80m"  // #5fd7d7 (soft cyan)
	ansiTool      = "\033[38;5;141m" // #af87ff (lavender)
	ansiCelebrate = "\033[38;5;120m" // #87ff87 (lime)
	ansiDim       = "\033[38;5;240m" // #585858 (charcoal)
)

// ANSI style codes.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDimmed = "\033[2m"
)

// ResolveColor maps a ColorToken to its ANSI escape code.
func ResolveColor(token ColorToken) string {
	switch token {
	case ColorPrimary:
		return ansiPrimary
	case ColorSecondary:
		return ansiSecondary
	case ColorTertiary:
		return ansiTertiary
	case ColorSuccess:
		return ansiSuccess
	case ColorError:
		return ansiError
	case ColorWarning:
		return ansiWarning
	case ColorInfo:
		return ansiInfo
	case ColorTool:
		return ansiTool
	case ColorCelebrate:
		return ansiCelebrate
	case ColorDim:
		return ansiDim
	default:
		return ansiReset
	}
}

// ResolveStyle maps a StyleToken to its ANSI escape code.
func ResolveStyle(token StyleToken) string {
	switch token {
	case WeightBold:
		return ansiBold
	case WeightDim:
		return ansiDimmed
	case WeightNormal:
		return ansiReset
	default:
		return ansiReset
	}
}

// VerticalSpace returns a string of newlines for the given number of lines.
func VerticalSpace(lines int) string {
	if lines <= 0 {
		return ""
	}
	return strings.Repeat("\n", lines)
}
