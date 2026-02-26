package ui_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/ui"
)

func TestStripColors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips color codes",
			input:    "\033[1;36mTest\033[0m",
			expected: "Test",
		},
		{
			name:     "handles plain text",
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			name:     "strips multiple colors",
			input:    "\033[32mGreen\033[0m \033[31mRed\033[0m",
			expected: "Green Red",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.StripColors(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatters(t *testing.T) {
	// Test that formatters return non-empty strings
	assert.NotEmpty(t, ui.Header("Test", ""))
	assert.NotEmpty(t, ui.Step("Test"))
	assert.NotEmpty(t, ui.Success("Test"))
	assert.NotEmpty(t, ui.Error("Test"))
	assert.NotEmpty(t, ui.Info("Test"))
	assert.NotEmpty(t, ui.DimError("Test"))
	assert.NotEmpty(t, ui.Tool("Test"))
	assert.NotEmpty(t, ui.Separator())

	// Test that stripped output contains expected text
	assert.Contains(t, ui.StripColors(ui.Header("Test", "")), "Test")
	assert.Contains(t, ui.StripColors(ui.Step("Test")), "▶ Test")
	assert.Equal(t, " ✓ Test", ui.StripColors(ui.Success("Test")))
	assert.Equal(t, " ✗ Test", ui.StripColors(ui.Error("Test")))
	assert.Equal(t, "Test\n", ui.StripColors(ui.Info("Test")))
	assert.Equal(t, "Test", ui.StripColors(ui.DimError("Test")))
	assert.Equal(t, " 🔧 Test", ui.StripColors(ui.Tool("Test")))
}

func TestFormattersSanitizeInput(t *testing.T) {
	// Test that formatters sanitize malicious ANSI codes
	malicious := "\033[2J\033[HMalicious"

	tests := []struct {
		name      string
		formatter func(string) string
		expected  string
	}{
		{"Error", ui.Error, " ✗ Malicious"},
		{"Success", ui.Success, " ✓ Malicious"},
		{"Info", ui.Info, "Malicious\n"},
		{"DimError", ui.DimError, "Malicious"},
		{"Tool", ui.Tool, " 🔧 Malicious"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.StripColors(tt.formatter(malicious))
			assert.Equal(t, tt.expected, result,
				"Formatter should sanitize ANSI codes from input")
		})
	}
}

func TestHeaderWithDescription(t *testing.T) {
	result := ui.Header("Test Header", "A short description")
	stripped := ui.StripColors(result)

	// Should contain ▶ prefix and text
	assert.Contains(t, stripped, "▶ Test Header", "Header should use ▶ prefix")

	// Should contain description line indented
	assert.Contains(t, stripped, "  A short description", "Header should show indented description")

	// Should NOT contain any box drawing characters
	assert.NotContains(t, stripped, "╔", "Header should not contain box characters")
	assert.NotContains(t, stripped, "═", "Header should not contain box characters")
	assert.NotContains(t, stripped, "╗", "Header should not contain box characters")
	assert.NotContains(t, stripped, "║", "Header should not contain box characters")
	assert.NotContains(t, stripped, "╚", "Header should not contain box characters")
	assert.NotContains(t, stripped, "╝", "Header should not contain box characters")
}

func TestHeaderWithoutDescription(t *testing.T) {
	result := ui.Header("Test Header", "")
	stripped := ui.StripColors(result)

	// Should contain ▶ prefix and text
	assert.Contains(t, stripped, "▶ Test Header", "Header should use ▶ prefix")

	// Should NOT contain any box drawing characters
	assert.NotContains(t, stripped, "╔")
	assert.NotContains(t, stripped, "║")
	assert.NotContains(t, stripped, "╝")

	// Count lines in trimmed output — should be only the title line
	lines := strings.Split(strings.TrimSpace(stripped), "\n")
	assert.Equal(t, 1, len(lines), "Header without description should have only title line")
}

func TestHeaderTruncatesLongDescription(t *testing.T) {
	longDesc := strings.Repeat("a", 100) // longer than SeparatorWidth - 2
	result := ui.Header("Title", longDesc)
	stripped := ui.StripColors(result)

	lines := strings.Split(strings.TrimSpace(stripped), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "Should have title and description lines")

	// Description line (second line) should be truncated with …
	descLine := lines[1]
	assert.Contains(t, descLine, "…", "Long description should be truncated with …")
	// The description content (after indent) should be at most SeparatorWidth - 2 runes
	trimmedDesc := strings.TrimSpace(descLine)
	assert.LessOrEqual(t, utf8.RuneCountInString(trimmedDesc), ui.SeparatorWidth-2,
		"Truncated description should fit within SeparatorWidth-2")
}

func TestStepNumbering(t *testing.T) {
	result := ui.StepNumbered(3, 9, "Implement TASK2")
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "▶ Step 3/9:", "Step should include numbering")
	assert.Contains(t, stripped, "Implement TASK2", "Step should include description")
}

func TestToolIndentation(t *testing.T) {
	result := ui.Tool("Write file.go")
	stripped := ui.StripColors(result)

	// Tool should be indented 1 char
	assert.True(t, strings.HasPrefix(stripped, " 🔧"), "Tool should be indented 1 char")
}

func TestSuccessIndentation(t *testing.T) {
	result := ui.Success("Tests passed")
	stripped := ui.StripColors(result)

	// Success should be indented 1 char
	assert.True(t, strings.HasPrefix(stripped, " ✓"), "Success should be indented 1 char")
}

func TestErrorIndentation(t *testing.T) {
	result := ui.Error("Build failed")
	stripped := ui.StripColors(result)

	// Error should be indented 1 char
	assert.True(t, strings.HasPrefix(stripped, " ✗"), "Error should be indented 1 char")
}

func TestErrorWithDetails(t *testing.T) {
	message := "Tests failed"
	details := []string{
		"Error: undefined: Calculate",
		"File: calc/add_test.go:15",
	}
	result := ui.ErrorWithDetails(message, details)
	stripped := ui.StripColors(result)
	lines := strings.Split(stripped, "\n")

	// Should have main error line + detail lines
	assert.Equal(t, 3, len(lines), "Should have 3 lines (main + 2 details)")

	// Main error line should be indented 1 char
	assert.True(t, strings.HasPrefix(lines[0], " ✗"), "Main error should be indented 1 char")
	assert.Contains(t, lines[0], "Tests failed", "Should contain error message")

	// Detail lines should have tree structure and be indented 2 chars
	assert.True(t, strings.HasPrefix(lines[1], "  └─"), "Detail 1 should have tree prefix and 2-char indent")
	assert.Contains(t, lines[1], "Error: undefined: Calculate", "Should contain detail 1")

	assert.True(t, strings.HasPrefix(lines[2], "  └─"), "Detail 2 should have tree prefix and 2-char indent")
	assert.Contains(t, lines[2], "File: calc/add_test.go:15", "Should contain detail 2")
}

func TestErrorWithDetailsEmpty(t *testing.T) {
	message := "Tests failed"
	details := []string{}
	result := ui.ErrorWithDetails(message, details)
	stripped := ui.StripColors(result)
	lines := strings.Split(stripped, "\n")

	// Should only have main error line
	assert.Equal(t, 1, len(lines), "Should have only main error line")
	assert.True(t, strings.HasPrefix(lines[0], " ✗"), "Main error should be indented 1 char")
}

func TestCompleteBoxed(t *testing.T) {
	result := ui.CompleteBoxed("TASK2", 3, 47, true)
	stripped := ui.StripColors(result)
	lines := strings.Split(strings.TrimSpace(stripped), "\n")

	// Should have: top border, title line, details lines, bottom border
	assert.GreaterOrEqual(t, len(lines), 5, "Complete should have boxed format")

	// Verify box characters
	assert.Contains(t, lines[0], "┌", "Should have top-left corner")
	assert.Contains(t, lines[0], "┐", "Should have top-right corner")
	assert.Contains(t, lines[len(lines)-1], "└", "Should have bottom-left corner")
	assert.Contains(t, lines[len(lines)-1], "┘", "Should have bottom-right corner")

	// Verify content
	assert.Contains(t, stripped, "✨ TASK2 implementation complete", "Should contain title")
	assert.Contains(t, stripped, "• 3 files changed", "Should contain files count")
	assert.Contains(t, stripped, "• 47 lines added", "Should contain lines count")
	assert.Contains(t, stripped, "• All tests passing", "Should contain test status")
}

func TestInterruptedWithContext(t *testing.T) {
	result := ui.InterruptedWithContext("Stopped by user", 5, 9)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "⚠", "Should contain warning symbol")
	assert.Contains(t, stripped, "Stopped by user", "Should contain message")
	assert.Contains(t, stripped, "State saved at step 5/9", "Should contain step context")
	assert.Contains(t, stripped, "resume with 'snap'", "Should contain resume instruction")
}

func TestHeaderSpacing(t *testing.T) {
	result := ui.Header("Test", "")
	stripped := ui.StripColors(result)

	// Header should have SpaceMD before (3 newlines) and SpaceXS after (1 newline)
	assert.True(t, strings.HasPrefix(stripped, "\n\n\n"), "Header should start with SpaceMD (3 newlines)")
	assert.True(t, strings.HasSuffix(stripped, "\n"), "Header should end with SpaceXS (1 newline)")
	// Verify it's exactly 1 newline, not 2
	assert.False(t, strings.HasSuffix(stripped, "\n\n"), "Header should NOT end with 2 newlines")
}

func TestStepNumberedSpacing(t *testing.T) {
	result := ui.StepNumbered(1, 9, "Test")
	stripped := ui.StripColors(result)

	// Step should end with SpaceXS (1 newline)
	assert.True(t, strings.HasSuffix(stripped, "\n"), "Step should end with SpaceXS (1 newline)")
}

func TestCompleteBoxedSpacing(t *testing.T) {
	result := ui.CompleteBoxed("TASK1", 2, 45, true)
	stripped := ui.StripColors(result)

	// CompleteBoxed should have SpaceMD before (3 newlines) and SpaceSM after (2 newlines)
	assert.True(t, strings.HasPrefix(stripped, "\n\n\n"), "CompleteBoxed should start with SpaceMD (3 newlines)")
	assert.True(t, strings.HasSuffix(stripped, "\n\n"), "CompleteBoxed should end with SpaceSM (2 newlines)")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{name: "zero", input: 0, expected: "0s"},
		{name: "sub-second", input: 500 * time.Millisecond, expected: "0s"},
		{name: "exactly 1 second", input: 1 * time.Second, expected: "1s"},
		{name: "seconds only", input: 45 * time.Second, expected: "45s"},
		{name: "59 seconds", input: 59 * time.Second, expected: "59s"},
		{name: "exactly 1 minute", input: 1 * time.Minute, expected: "1m"},
		{name: "minutes with zero seconds omitted", input: 2 * time.Minute, expected: "2m"},
		{name: "minutes and seconds", input: 2*time.Minute + 34*time.Second, expected: "2m 34s"},
		{name: "59 minutes 59 seconds", input: 59*time.Minute + 59*time.Second, expected: "59m 59s"},
		{name: "exactly 1 hour", input: 1 * time.Hour, expected: "1h"},
		{name: "hours with zero minutes omitted", input: 2 * time.Hour, expected: "2h"},
		{name: "hours and minutes", input: 1*time.Hour + 12*time.Minute, expected: "1h 12m"},
		{name: "hours minutes seconds truncated to hours minutes", input: 1*time.Hour + 12*time.Minute + 30*time.Second, expected: "1h 12m"},
		{name: "truncates milliseconds", input: 45*time.Second + 999*time.Millisecond, expected: "45s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ui.FormatDuration(tt.input))
		})
	}
}

func TestStepComplete(t *testing.T) {
	result := ui.StepComplete("Step complete", 2*time.Minute+34*time.Second)
	stripped := ui.StripColors(result)

	// Should contain checkmark, text, and duration
	assert.Contains(t, stripped, "✓")
	assert.Contains(t, stripped, "Step complete")
	assert.Contains(t, stripped, "2m 34s")

	// Should be indented 1 char (matches Success)
	assert.True(t, strings.HasPrefix(stripped, " ✓"), "StepComplete should be indented 1 char")

	// Duration should be right-aligned to SeparatorWidth (70 runes)
	assert.Equal(t, 70, utf8.RuneCountInString(stripped), "StepComplete should be exactly SeparatorWidth runes")
}

func TestStepCompleteZeroDuration(t *testing.T) {
	result := ui.StepComplete("Step complete", 0)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "0s")
	assert.Equal(t, 70, utf8.RuneCountInString(stripped))
}

func TestStepFailed(t *testing.T) {
	result := ui.StepFailed("Step failed", 2*time.Minute+34*time.Second)
	stripped := ui.StripColors(result)

	// Should contain X mark, text, and duration
	assert.Contains(t, stripped, "✗")
	assert.Contains(t, stripped, "Step failed")
	assert.Contains(t, stripped, "2m 34s")

	// Should be indented 1 char (matches Error)
	assert.True(t, strings.HasPrefix(stripped, " ✗"), "StepFailed should be indented 1 char")

	// Duration should be right-aligned to SeparatorWidth (70 runes)
	assert.Equal(t, 70, utf8.RuneCountInString(stripped), "StepFailed should be exactly SeparatorWidth runes")
}

func TestCompleteWithDuration(t *testing.T) {
	result := ui.CompleteWithDuration("Iteration complete", 12*time.Minute+47*time.Second)
	stripped := ui.StripColors(result)

	// Should contain sparkle, text, and duration
	assert.Contains(t, stripped, "✨")
	assert.Contains(t, stripped, "Iteration complete")
	assert.Contains(t, stripped, "12m 47s")

	// Should start with newline (matches Complete)
	assert.True(t, strings.HasPrefix(stripped, "\n"), "CompleteWithDuration should start with newline")

	// Content line (after leading newline, before trailing newline) should be SeparatorWidth
	trimmed := strings.TrimSpace(stripped)
	assert.Equal(t, 70, utf8.RuneCountInString(trimmed), "CompleteWithDuration content should be exactly SeparatorWidth runes")

	// Should end with newline (matches Complete)
	assert.True(t, strings.HasSuffix(stripped, "\n"), "CompleteWithDuration should end with newline")
}

func TestDurationUsesDimStyling(t *testing.T) {
	dimWeight := "\033[2m"       // ansiDimmed (WeightDim)
	dimColor := "\033[38;5;240m" // ansiDim (ColorDim)

	t.Run("StepComplete duration has dim color and weight", func(t *testing.T) {
		raw := ui.StepComplete("Step complete", 45*time.Second)
		assert.Contains(t, raw, dimWeight, "StepComplete should use dim weight for duration")
		assert.Contains(t, raw, dimColor, "StepComplete should use dim color for duration")
	})

	t.Run("StepFailed duration has dim color and weight", func(t *testing.T) {
		raw := ui.StepFailed("Step failed", 45*time.Second)
		assert.Contains(t, raw, dimWeight, "StepFailed should use dim weight for duration")
		assert.Contains(t, raw, dimColor, "StepFailed should use dim color for duration")
	})

	t.Run("CompleteWithDuration duration has dim color and weight", func(t *testing.T) {
		raw := ui.CompleteWithDuration("Iteration complete", 12*time.Minute+47*time.Second)
		assert.Contains(t, raw, dimWeight, "CompleteWithDuration should use dim weight for duration")
		assert.Contains(t, raw, dimColor, "CompleteWithDuration should use dim color for duration")
	})
}

func TestInterruptedWithContextSpacing(t *testing.T) {
	result := ui.InterruptedWithContext("Stopped", 5, 9)
	stripped := ui.StripColors(result)

	// InterruptedWithContext should have SpaceSM before and after (2 newlines each)
	assert.True(t, strings.HasPrefix(stripped, "\n\n"), "InterruptedWithContext should start with SpaceSM (2 newlines)")
	assert.True(t, strings.HasSuffix(stripped, "\n\n"), "InterruptedWithContext should end with SpaceSM (2 newlines)")
}
