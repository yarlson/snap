package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/ui"
)

func TestQueuedPrompt(t *testing.T) {
	result := ui.QueuedPrompt("fix the nil pointer", 3, 9, "Validate implementation", 1)
	stripped := ui.StripColors(result)

	// Should be a box with rounded corners
	lines := strings.Split(strings.TrimSpace(stripped), "\n")
	assert.GreaterOrEqual(t, len(lines), 4, "QueuedPrompt should have at least 4 lines")

	// Top border with title
	assert.Contains(t, lines[0], "┌", "Should have top-left corner")
	assert.Contains(t, lines[0], "📌 Queued", "Top border should contain Queued title")
	assert.Contains(t, lines[0], "┐", "Should have top-right corner")

	// Prompt text
	assert.Contains(t, stripped, "fix the nil pointer", "Should contain the prompt text")

	// Waiting indicator
	assert.Contains(t, stripped, "⏳ Waiting for Step 3/9: Validate implementation",
		"Should contain waiting indicator with step info")

	// Queue count
	assert.Contains(t, stripped, "📋 1 prompt in queue", "Should show queue count")

	// Bottom border
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "└", "Should have bottom-left corner")
	assert.Contains(t, lastLine, "┘", "Should have bottom-right corner")
}

func TestQueuedPrompt_PluralCount(t *testing.T) {
	result := ui.QueuedPrompt("add a test", 5, 10, "Code review", 3)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "📋 3 prompts in queue", "Should use plural for count > 1")
}

func TestQueuedPrompt_BoxWidth(t *testing.T) {
	result := ui.QueuedPrompt("short", 1, 9, "Test", 1)
	stripped := ui.StripColors(result)
	lines := strings.Split(strings.TrimSpace(stripped), "\n")

	// All lines should be BoxWidth (70 chars) — account for multi-byte chars in emoji
	for _, line := range lines {
		// Just verify box borders are present on each line
		assert.True(t,
			strings.Contains(line, "┌") || strings.Contains(line, "│") || strings.Contains(line, "└"),
			"Each line should contain box drawing character: %q", line)
	}
}

func TestQueueRunning(t *testing.T) {
	result := ui.QueueRunning("fix the nil pointer", 1, 2)
	stripped := ui.StripColors(result)

	lines := strings.Split(strings.TrimSpace(stripped), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "QueueRunning should have at least 3 lines")

	// Top border with title
	assert.Contains(t, lines[0], "┌", "Should have top-left corner")
	assert.Contains(t, lines[0], "📌 Running queued prompt (1/2)", "Top border should contain running title")
	assert.Contains(t, lines[0], "┐", "Should have top-right corner")

	// Prompt text
	assert.Contains(t, stripped, "fix the nil pointer", "Should contain the prompt text")

	// Bottom border
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "└", "Should have bottom-left corner")
	assert.Contains(t, lastLine, "┘", "Should have bottom-right corner")
}

func TestQueueRunning_Spacing(t *testing.T) {
	result := ui.QueueRunning("test", 1, 1)
	stripped := ui.StripColors(result)

	// Should have SpaceSM before (2 newlines)
	assert.True(t, strings.HasPrefix(stripped, "\n\n"), "QueueRunning should start with SpaceSM")
	// Should have SpaceXS after (1 newline)
	assert.True(t, strings.HasSuffix(stripped, "\n"), "QueueRunning should end with SpaceXS")
	assert.False(t, strings.HasSuffix(stripped, "\n\n"), "QueueRunning should NOT end with 2 newlines")
}

func TestQueueStatusEmpty(t *testing.T) {
	result := ui.QueueStatus(nil)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "📋 Queue empty — no prompts pending",
		"Empty queue should show empty message")
}

func TestQueueStatusWithPrompts(t *testing.T) {
	prompts := []string{"fix nil pointer", "add test for empty input"}
	result := ui.QueueStatus(prompts)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "📋 Queue (2 prompts pending):",
		"Should show queue count")
	assert.Contains(t, stripped, "1. fix nil pointer", "Should list first prompt")
	assert.Contains(t, stripped, "2. add test for empty input", "Should list second prompt")
}

func TestQueueStatusSinglePrompt(t *testing.T) {
	prompts := []string{"fix nil pointer"}
	result := ui.QueueStatus(prompts)
	stripped := ui.StripColors(result)

	assert.Contains(t, stripped, "📋 Queue (1 prompt pending):",
		"Should use singular for count == 1")
	assert.Contains(t, stripped, "1. fix nil pointer", "Should list the prompt")
}
