package ui

import (
	"fmt"
	"regexp"
	"strings"
)

// Header formats a major section header (e.g., "Implementing next task").
func Header(text string) string {
	// Calculate content width: BoxWidth - border chars (2) - padding (2 + 2)
	contentWidth := BoxWidth - 2 - BoxPaddingLeft - BoxPaddingRight
	colorCode := ResolveColor(ColorPrimary)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)

	// Top border
	topBorder := fmt.Sprintf("%s%s╔%s╗%s\n",
		styleCode, colorCode,
		strings.Repeat("═", BoxWidth-2),
		resetCode)

	// Empty padding line
	emptyLine := fmt.Sprintf("%s%s║%s║%s\n",
		styleCode, colorCode,
		strings.Repeat(" ", BoxWidth-2),
		resetCode)

	// Text line with padding
	textLine := fmt.Sprintf("%s%s║%s%-*s%s║%s\n",
		styleCode, colorCode,
		strings.Repeat(" ", BoxPaddingLeft),
		contentWidth,
		text,
		strings.Repeat(" ", BoxPaddingRight),
		resetCode)

	// Bottom border
	bottomBorder := fmt.Sprintf("%s%s╚%s╝%s",
		styleCode, colorCode,
		strings.Repeat("═", BoxWidth-2),
		resetCode)

	return VerticalSpace(SpaceMD) +
		topBorder +
		emptyLine +
		textLine +
		emptyLine +
		bottomBorder +
		VerticalSpace(SpaceXS)
}

// Step formats a step header (e.g., "Step: Implement next task").
func Step(text string) string {
	colorCode := ResolveColor(ColorSecondary)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("\n%s%s▶ %s%s\n", styleCode, colorCode, text, resetCode)
}

// StepNumbered formats a step header with numbering (e.g., "Step 2/9: Implement TASK2").
func StepNumbered(current, total int, text string) string {
	colorCode := ResolveColor(ColorSecondary)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("\n%s%s▶ Step %d/%d: %s%s%s",
		styleCode, colorCode, current, total, text, resetCode,
		VerticalSpace(SpaceXS))
}

// Success formats a success message with checkmark.
func Success(text string) string {
	sanitized := StripColors(text)
	colorCode := ResolveColor(ColorSuccess)
	styleCode := ResolveStyle(WeightBold)
	dimCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	indent := strings.Repeat(" ", IndentResult)
	return fmt.Sprintf("%s%s%s✓%s %s%s%s",
		indent, styleCode, colorCode, resetCode, dimCode, sanitized, resetCode)
}

// Error formats an error message with X mark.
func Error(text string) string {
	sanitized := StripColors(text)
	colorCode := ResolveColor(ColorError)
	styleCode := ResolveStyle(WeightBold)
	dimCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	indent := strings.Repeat(" ", IndentResult)
	return fmt.Sprintf("%s%s%s✗%s %s%s%s",
		indent, styleCode, colorCode, resetCode, dimCode, sanitized, resetCode)
}

// ErrorWithDetails formats an error message with X mark and multi-line details in tree format.
// details should be a slice of strings, each representing a detail line (e.g., "Error: ...", "File: ...").
func ErrorWithDetails(message string, details []string) string {
	colorCode := ResolveColor(ColorError)
	styleCode := ResolveStyle(WeightBold)
	dimCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	indent := strings.Repeat(" ", IndentResult)
	detailIndent := strings.Repeat(" ", IndentResult+1)

	var builder strings.Builder

	// Main error line with X mark
	fmt.Fprintf(&builder, "%s%s%s✗%s %s%s%s",
		indent, styleCode, colorCode, resetCode, dimCode, StripColors(message), resetCode)

	// Add detail lines with tree structure
	for _, detail := range details {
		fmt.Fprintf(&builder, "\n%s%s└─ %s%s",
			detailIndent, dimCode, StripColors(detail), resetCode)
	}

	return builder.String()
}

// Info formats an informational message.
func Info(text string) string {
	sanitized := StripColors(text)
	styleCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("%s%s%s\n", styleCode, sanitized, resetCode)
}

// DimError formats an error message in dimmed red.
func DimError(text string) string {
	sanitized := StripColors(text)
	colorCode := ResolveColor(ColorError)
	styleCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("%s%s%s%s", styleCode, colorCode, sanitized, resetCode)
}

// Tool formats a tool use message.
func Tool(text string) string {
	sanitized := StripColors(text)
	colorCode := ResolveColor(ColorTool)
	resetCode := ResolveStyle(WeightNormal)
	indent := strings.Repeat(" ", IndentTool)
	return fmt.Sprintf("%s%s🔧 %s%s", indent, colorCode, sanitized, resetCode)
}

// Separator returns a visual separator line.
func Separator() string {
	styleCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("%s%s%s\n", styleCode, strings.Repeat("─", SeparatorWidth), resetCode)
}

// Complete formats a completion message.
func Complete(text string) string {
	colorCode := ResolveColor(ColorCelebrate)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("\n%s%s✨ %s%s\n", styleCode, colorCode, text, resetCode)
}

// CompleteBoxed formats a completion message with boxed format and statistics.
func CompleteBoxed(taskName string, filesChanged, linesAdded int, testsPassing bool) string {
	colorCode := ResolveColor(ColorCelebrate)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)

	// Top border
	topBorder := fmt.Sprintf("%s%s┌%s┐%s\n",
		styleCode, colorCode,
		strings.Repeat("─", BoxWidth-2),
		resetCode)

	// Title line
	title := fmt.Sprintf("✨ %s implementation complete", taskName)
	titleLine := fmt.Sprintf("%s%s│  %-*s│%s\n",
		styleCode, colorCode,
		BoxWidth-4,
		title,
		resetCode)

	// Detail lines
	filesLine := fmt.Sprintf("%s%s│     • %-*s│%s\n",
		styleCode, colorCode,
		BoxWidth-9,
		fmt.Sprintf("%d files changed", filesChanged),
		resetCode)

	linesLine := fmt.Sprintf("%s%s│     • %-*s│%s\n",
		styleCode, colorCode,
		BoxWidth-9,
		fmt.Sprintf("%d lines added", linesAdded),
		resetCode)

	testStatus := "All tests passing"
	if !testsPassing {
		testStatus = "Tests need attention"
	}
	testsLine := fmt.Sprintf("%s%s│     • %-*s│%s\n",
		styleCode, colorCode,
		BoxWidth-9,
		testStatus,
		resetCode)

	// Bottom border
	bottomBorder := fmt.Sprintf("%s%s└%s┘%s",
		styleCode, colorCode,
		strings.Repeat("─", BoxWidth-2),
		resetCode)

	return VerticalSpace(SpaceMD) +
		topBorder +
		titleLine +
		filesLine +
		linesLine +
		testsLine +
		bottomBorder +
		VerticalSpace(SpaceSM)
}

// Interrupted formats an interruption message.
func Interrupted(text string) string {
	colorCode := ResolveColor(ColorWarning)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)
	return fmt.Sprintf("\n%s%s⚠ %s%s\n", styleCode, colorCode, text, resetCode)
}

// InterruptedWithContext formats an interruption message with step context and resume instructions.
func InterruptedWithContext(text string, currentStep, totalSteps int) string {
	colorCode := ResolveColor(ColorWarning)
	styleCode := ResolveStyle(WeightBold)
	dimCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)

	mainLine := fmt.Sprintf("%s%s⚠  %s%s\n", styleCode, colorCode, text, resetCode)
	contextLine := fmt.Sprintf("   %sState saved at step %d/%d - resume with 'snap'%s",
		dimCode, currentStep, totalSteps, resetCode)

	return VerticalSpace(SpaceSM) + mainLine + contextLine + VerticalSpace(SpaceSM)
}

// StripColors removes ANSI escape sequences from a string (useful for testing and sanitization).
func StripColors(s string) string {
	// Match all ANSI escape sequences:
	// - \x1b\[[0-9;]*m  (color codes like \033[31m)
	// - \x1b\[[0-9;]*[A-Za-z]  (cursor control like \033[2J, \033[H)
	// - \x1b[()][A-Z0-9]  (character set selection)
	re := regexp.MustCompile(`\x1b(?:\[[0-9;]*[A-Za-z]|\([A-Z0-9])`)
	return re.ReplaceAllString(s, "")
}
