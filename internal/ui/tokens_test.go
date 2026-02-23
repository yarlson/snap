package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/ui"
)

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
