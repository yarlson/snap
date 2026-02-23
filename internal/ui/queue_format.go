package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// boxContentWidth is the usable width inside a box line ("│ " content " │").
const boxContentWidth = BoxWidth - 4

// QueuedPrompt formats the boxed acknowledgment shown when a user prompt is queued.
// It shows the prompt text, which step is currently running, and how many prompts are queued.
func QueuedPrompt(prompt string, currentStep, totalSteps int, stepName string, queueLen int) string {
	colorCode := ResolveColor(ColorInfo)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)
	dimCode := ResolveStyle(WeightDim)

	// Top border with title
	topBorder := boxTopBorder("📌 Queued", styleCode, colorCode, resetCode)

	// Prompt text line
	promptLine := boxLine(fitText(prompt), styleCode, colorCode, resetCode)

	// Empty line
	emptyLine := boxLine("", styleCode, colorCode, resetCode)

	// Waiting indicator
	waitText := fmt.Sprintf("⏳ Waiting for Step %d/%d: %s", currentStep, totalSteps, stepName)
	waitLine := boxLine(fitText(waitText), dimCode, colorCode, resetCode)

	// Queue count
	noun := "prompts"
	if queueLen == 1 {
		noun = "prompt"
	}
	countText := fmt.Sprintf("📋 %d %s in queue", queueLen, noun)
	countLine := boxLine(fitText(countText), dimCode, colorCode, resetCode)

	// Bottom border
	bottomBorder := boxBottomBorder(styleCode, colorCode, resetCode)

	return VerticalSpace(SpaceXS) +
		topBorder +
		promptLine +
		emptyLine +
		waitLine +
		countLine +
		bottomBorder +
		VerticalSpace(SpaceXS)
}

// QueueRunning formats the boxed header shown when a queued prompt is being executed.
func QueueRunning(prompt string, current, total int) string {
	colorCode := ResolveColor(ColorInfo)
	styleCode := ResolveStyle(WeightBold)
	resetCode := ResolveStyle(WeightNormal)

	title := fmt.Sprintf("📌 Running queued prompt (%d/%d)", current, total)
	topBorder := boxTopBorder(title, styleCode, colorCode, resetCode)
	promptLine := boxLine(fitText(prompt), styleCode, colorCode, resetCode)
	bottomBorder := boxBottomBorder(styleCode, colorCode, resetCode)

	return VerticalSpace(SpaceSM) +
		topBorder +
		promptLine +
		bottomBorder +
		VerticalSpace(SpaceXS)
}

// QueueStatus formats the queue status display for when the user presses Enter with empty input.
// Pass nil or empty slice for an empty queue.
func QueueStatus(prompts []string) string {
	dimCode := ResolveStyle(WeightDim)
	resetCode := ResolveStyle(WeightNormal)

	if len(prompts) == 0 {
		return fmt.Sprintf("\n%s📋 Queue empty — no prompts pending%s\n", dimCode, resetCode)
	}

	noun := "prompts"
	if len(prompts) == 1 {
		noun = "prompt"
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "\n%s📋 Queue (%d %s pending):%s\n",
		dimCode, len(prompts), noun, resetCode)

	for i, p := range prompts {
		fmt.Fprintf(&builder, "%s  %d. %s%s\n", dimCode, i+1, p, resetCode)
	}

	return builder.String()
}

// boxTopBorder builds a titled top border (e.g. ┌─ title ────┐).
func boxTopBorder(title, styleCode, colorCode, resetCode string) string {
	titleLen := utf8.RuneCountInString(title)
	dashCount := BoxWidth - 4 - titleLen - 2 // "┌─ " + title + " " + dashes + "┐"
	if dashCount < 1 {
		dashCount = 1
	}
	return fmt.Sprintf("%s%s┌─ %s %s┐%s\n",
		styleCode, colorCode,
		title,
		strings.Repeat("─", dashCount),
		resetCode)
}

// boxLine builds a content line (e.g. │ text │).
func boxLine(text, styleCode, colorCode, resetCode string) string {
	padding := boxContentWidth - utf8.RuneCountInString(text)
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("%s%s│ %s%s │%s\n",
		styleCode, colorCode,
		text,
		strings.Repeat(" ", padding),
		resetCode)
}

// boxBottomBorder builds a bottom border (e.g. └──────────┘).
func boxBottomBorder(styleCode, colorCode, resetCode string) string {
	return fmt.Sprintf("%s%s└%s┘%s",
		styleCode, colorCode,
		strings.Repeat("─", BoxWidth-2),
		resetCode)
}

// fitText shortens text to boxContentWidth runes, adding "…" if truncated.
func fitText(s string) string {
	if utf8.RuneCountInString(s) <= boxContentWidth {
		return s
	}
	runes := []rune(s)
	return string(runes[:boxContentWidth-1]) + "…"
}
