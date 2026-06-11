//go:build integration

package theme_test

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

func TestNewReturnsTheme(t *testing.T) {
	// When
	th := theme.New()

	// Then
	require.NotNil(t, th)
}

func TestThemeColor(t *testing.T) {
	// Given
	th := theme.New()

	tests := []struct {
		name      string
		colorName fyne.ThemeColorName
		expected  color.Color
	}{
		{"background", fyneTheme.ColorNameBackground, theme.Gray950},
		{"button", fyneTheme.ColorNameButton, theme.Gray700},
		{"primary", fyneTheme.ColorNamePrimary, theme.Yellow500},
		{"focus", fyneTheme.ColorNameFocus, theme.Yellow400},
		{"foreground", fyneTheme.ColorNameForeground, color.White},
		{"foreground on primary", fyneTheme.ColorNameForegroundOnPrimary, theme.ForegroundOnPrimary},
		{"foreground on error", fyneTheme.ColorNameForegroundOnError, color.White},
		{"foreground on success", fyneTheme.ColorNameForegroundOnSuccess, theme.ForegroundOnAccent},
		{"foreground on warning", fyneTheme.ColorNameForegroundOnWarning, theme.ForegroundOnAccent},
		{"scrollbar background", fyneTheme.ColorNameScrollBarBackground, theme.Gray950},
		{"error", fyneTheme.ColorNameError, theme.ErrorRed},
		{"warning", fyneTheme.ColorNameWarning, theme.WarningAmber},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			gotLight := th.Color(tt.colorName, fyneTheme.VariantLight)
			gotDark := th.Color(tt.colorName, fyneTheme.VariantDark)

			// Then
			require.Equal(t, tt.expected, gotDark, "Color(%s) dark", tt.colorName)
			require.Equal(t, tt.expected, gotLight, "Color(%s) light", tt.colorName)
		})
	}
}

func TestThemeAllKnownColorNames(t *testing.T) {
	// Given
	th := theme.New()

	// When / Then
	test.AssertAllColorNamesDefined(t, th, "deckcheck")
}

func TestThemeSize(t *testing.T) {
	// Given
	th := theme.New()

	tests := []struct {
		name     string
		sizeName fyne.ThemeSizeName
		expected float32
	}{
		{"caption", fyneTheme.SizeNameCaptionText, 12},
		{"heading", fyneTheme.SizeNameHeadingText, 26},
		{"inline icon", fyneTheme.SizeNameInlineIcon, 20},
		{"inner padding", fyneTheme.SizeNameInnerPadding, 8},
		{"input border", fyneTheme.SizeNameInputBorder, 2},
		{"input radius", fyneTheme.SizeNameInputRadius, 8},
		{"line spacing", fyneTheme.SizeNameLineSpacing, 4},
		{"padding", fyneTheme.SizeNamePadding, 10},
		{"scrollbar", fyneTheme.SizeNameScrollBar, 12},
		{"scrollbar small", fyneTheme.SizeNameScrollBarSmall, 4},
		{"selection radius", fyneTheme.SizeNameSelectionRadius, 4},
		{"separator thickness", fyneTheme.SizeNameSeparatorThickness, 1},
		{"subheading", fyneTheme.SizeNameSubHeadingText, 17},
		{"text", fyneTheme.SizeNameText, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := th.Size(tt.sizeName)

			// Then
			require.Equal(t, tt.expected, got, "Size(%s)", tt.sizeName)
		})
	}
}

func TestThemeFont(t *testing.T) {
	// Given
	th := theme.New()
	style := fyne.TextStyle{}

	// When
	font := th.Font(style)

	// Then
	require.NotNil(t, font)
}

func TestThemeIcon(t *testing.T) {
	// Given
	th := theme.New()

	// When
	icon := th.Icon(fyneTheme.IconNameHome)

	// Then
	require.NotNil(t, icon)
}
