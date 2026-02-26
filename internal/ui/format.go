package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Header formats a major section header (e.g., "Implementing next task").
// If description is non-empty, a dim indented line is shown below the title.
func Header(text, description string) string {
	colorCode := ResolveColor(ColorPrimary)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)

	titleLine := fmt.Sprintf("%s%s▶ %s%s", styleCode, colorCode, text, resetCode)

	if description != "" {
		maxLen := SeparatorWidth - 2 // account for 2-char indent
		descRunes := []rune(description)
		if len(descRunes) > maxLen {
			description = string(descRunes[:maxLen-1]) + "…"
		}
		dimCode := ResolveStyle(WeightDim)
		descLine := fmt.Sprintf("  %s%s%s", dimCode, description, resetCode)
		return VerticalSpace(SpaceMD) + titleLine + "\n" + descLine + VerticalSpace(SpaceXS)
	}

	return VerticalSpace(SpaceMD) + titleLine + VerticalSpace(SpaceXS)
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

// FormatDuration converts a time.Duration to a compact human-readable string.
// Rules: <60s → "45s", 1m–59m → "2m 34s", >=60m → "1h 12m".
// Zero-value components are omitted. Sub-second durations show "0s".
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)

	totalSeconds := int(d.Seconds())
	if totalSeconds == 0 {
		return "0s"
	}

	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60

	if h > 0 {
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// StepComplete renders a step-done line with a right-aligned duration.
// Uses success color for the checkmark, dim weight for text and duration.
func StepComplete(text string, elapsed time.Duration) string {
	durationStr := FormatDuration(elapsed)
	prefix := " ✓ " + StripColors(text)
	return rightAlignedLine(prefix, durationStr,
		ResolveColor(ColorSuccess), ResolveStyle(WeightBold),
		ResolveStyle(WeightDim), ResolveColor(ColorDim))
}

// StepFailed renders a step-failed line with a right-aligned duration.
// Uses error color for the X mark, dim weight for text and duration.
func StepFailed(text string, elapsed time.Duration) string {
	durationStr := FormatDuration(elapsed)
	prefix := " ✗ " + StripColors(text)
	return rightAlignedLine(prefix, durationStr,
		ResolveColor(ColorError), ResolveStyle(WeightBold),
		ResolveStyle(WeightDim), ResolveColor(ColorDim))
}

// CompleteWithDuration renders a completion message with a right-aligned duration.
// Uses celebrate color for the sparkle and text, dim for the duration.
func CompleteWithDuration(text string, elapsed time.Duration) string {
	durationStr := FormatDuration(elapsed)
	prefix := "✨ " + text
	colorCode := ResolveColor(ColorCelebrate)
	styleCode := ResolveStyle(WeightBold)
	dimCode := ResolveStyle(WeightDim)
	dimColor := ResolveColor(ColorDim)
	resetCode := ResolveStyle(WeightNormal)

	padWidth := SeparatorWidth - utf8.RuneCountInString(prefix) - len(durationStr)
	if padWidth < 1 {
		padWidth = 1
	}

	return fmt.Sprintf("\n%s%s%s%s%s%s%s%s\n",
		styleCode, colorCode, prefix,
		strings.Repeat(" ", padWidth),
		dimCode, dimColor, durationStr,
		resetCode)
}

// rightAlignedLine renders a line with an icon prefix and right-aligned duration.
// The icon (first 2 runes of prefix) uses iconColor + iconStyle; the rest uses dimStyle;
// the duration uses dimStyle + dimColor. Total visible width = SeparatorWidth.
func rightAlignedLine(prefix, durationStr, iconColor, iconStyle, dimStyle, dimColor string) string {
	resetCode := ResolveStyle(WeightNormal)

	// Split prefix into icon part (e.g., " ✓") and text part (e.g., " Step complete")
	// The icon is the indent + symbol (first 2 runes: space, symbol)
	runes := []rune(prefix)
	icon := string(runes[:2]) // " ✓" or " ✗"
	text := string(runes[2:]) // " Step complete"

	padWidth := SeparatorWidth - utf8.RuneCountInString(prefix) - len(durationStr)
	if padWidth < 1 {
		padWidth = 1
	}

	return fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s",
		iconStyle, iconColor, icon, resetCode,
		dimStyle, text,
		strings.Repeat(" ", padWidth),
		dimColor, durationStr,
		resetCode)
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
