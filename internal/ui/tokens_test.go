package ui_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/ui"
)

// TestMain resets color mode before running tests so that unit tests get
// predictable color output regardless of terminal state.  The production
// init() disables colors when stdout is not a TTY (e.g. CI), but unit tests
// need ANSI codes enabled to validate token values.
func TestMain(m *testing.M) {
	ui.ResetColorMode()
	os.Exit(m.Run())
}

func TestResolveColor(t *testing.T) {
	tests := []struct {
		name     string
		token    ui.ColorToken
		expected string
	}{
		{"Primary teal", ui.ColorPrimary, "\033[38;5;37m"},
		{"Secondary sky blue", ui.ColorSecondary, "\033[38;5;75m"},
		{"Tertiary gray", ui.ColorTertiary, "\033[38;5;245m"},
		{"Success forest green", ui.ColorSuccess, "\033[38;5;35m"},
		{"Error warm red", ui.ColorError, "\033[38;5;167m"},
		{"Warning amber", ui.ColorWarning, "\033[38;5;214m"},
		{"Info soft cyan", ui.ColorInfo, "\033[38;5;80m"},
		{"Tool lavender", ui.ColorTool, "\033[38;5;141m"},
		{"Celebrate lime green", ui.ColorCelebrate, "\033[38;5;120m"},
		{"Dim charcoal", ui.ColorDim, "\033[38;5;240m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ResolveColor(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerticalSpace(t *testing.T) {
	tests := []struct {
		name     string
		lines    int
		expected string
	}{
		{"No space", ui.SpaceNone, ""},
		{"XS - 1 line", ui.SpaceXS, "\n"},
		{"SM - 2 lines", ui.SpaceSM, "\n\n"},
		{"MD - 3 lines", ui.SpaceMD, "\n\n\n"},
		{"LG - 4 lines", ui.SpaceLG, "\n\n\n\n"},
		{"XL - 6 lines", ui.SpaceXL, "\n\n\n\n\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.VerticalSpace(tt.lines)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStyleTokens(t *testing.T) {
	tests := []struct {
		name     string
		token    ui.StyleToken
		expected string
	}{
		{"Normal weight", ui.WeightNormal, "\033[0m"},
		{"Bold weight", ui.WeightBold, "\033[1m"},
		{"Dim weight", ui.WeightDim, "\033[2m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ResolveStyle(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNoColor_ResolveColorReturnsEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	ui.ResetColorMode()
	t.Cleanup(func() { ui.ResetColorMode() })

	tokens := []ui.ColorToken{
		ui.ColorPrimary, ui.ColorSecondary, ui.ColorTertiary,
		ui.ColorSuccess, ui.ColorError, ui.ColorWarning,
		ui.ColorInfo, ui.ColorTool, ui.ColorCelebrate, ui.ColorDim,
	}

	for _, token := range tokens {
		assert.Equal(t, "", ui.ResolveColor(token), "ResolveColor(%q) should return empty when NO_COLOR=1", token)
	}
}

func TestNoColor_ResolveStyleReturnsEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	ui.ResetColorMode()
	t.Cleanup(func() { ui.ResetColorMode() })

	tokens := []ui.StyleToken{ui.WeightBold, ui.WeightDim, ui.WeightNormal}

	for _, token := range tokens {
		assert.Equal(t, "", ui.ResolveStyle(token), "ResolveStyle(%q) should return empty when NO_COLOR=1", token)
	}
}

func TestNoColor_WeightNormalNotResetCode(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	ui.ResetColorMode()
	t.Cleanup(func() { ui.ResetColorMode() })

	result := ui.ResolveStyle(ui.WeightNormal)
	assert.Equal(t, "", result, "WeightNormal must return empty string, not reset code, when colors disabled")
	assert.NotEqual(t, "\033[0m", result)
}

func TestNoColor_UnsetRestoresColors(t *testing.T) {
	// t.Setenv automatically cleans up (unsets) when the test ends.
	// Explicitly test: set NO_COLOR, reset, unset, reset again → colors restored.
	t.Setenv("NO_COLOR", "1")
	ui.ResetColorMode()
	assert.Equal(t, "", ui.ResolveColor(ui.ColorPrimary))

	// Simulate unsetting by setting to empty (t.Setenv cleanup will fully unset).
	t.Setenv("NO_COLOR", "")
	ui.ResetColorMode()
	assert.NotEmpty(t, ui.ResolveColor(ui.ColorPrimary), "colors should be enabled when NO_COLOR is empty")
	assert.NotEmpty(t, ui.ResolveStyle(ui.WeightBold), "styles should be enabled when NO_COLOR is empty")

	t.Cleanup(func() { ui.ResetColorMode() })
}

func TestNoColor_EmptyValueKeepsColorsEnabled(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	ui.ResetColorMode()
	t.Cleanup(func() { ui.ResetColorMode() })

	assert.NotEmpty(t, ui.ResolveColor(ui.ColorPrimary), "empty NO_COLOR should keep colors enabled")
	assert.NotEmpty(t, ui.ResolveStyle(ui.WeightBold), "empty NO_COLOR should keep styles enabled")
}

func TestNoColor_AnyNonEmptyValueDisables(t *testing.T) {
	values := []string{"1", "true", "yes", "anything"}
	for _, val := range values {
		t.Run("NO_COLOR="+val, func(t *testing.T) {
			t.Setenv("NO_COLOR", val)
			ui.ResetColorMode()
			t.Cleanup(func() { ui.ResetColorMode() })

			assert.Equal(t, "", ui.ResolveColor(ui.ColorPrimary), "NO_COLOR=%q should disable colors", val)
			assert.Equal(t, "", ui.ResolveStyle(ui.WeightBold), "NO_COLOR=%q should disable styles", val)
		})
	}
}
