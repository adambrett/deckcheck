package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	Gray950 = color.NRGBA{R: 15, G: 23, B: 36, A: 255}    // #0f1724
	Gray900 = color.NRGBA{R: 17, G: 25, B: 40, A: 255}    // #111928
	Gray800 = color.NRGBA{R: 33, G: 45, B: 61, A: 255}    // #212d3d
	Gray700 = color.NRGBA{R: 55, G: 65, B: 80, A: 255}    // #374150
	Gray600 = color.NRGBA{R: 82, G: 95, B: 114, A: 255}   // #525f72
	Gray500 = color.NRGBA{R: 107, G: 114, B: 128, A: 255} // #6b7280
	Gray400 = color.NRGBA{R: 156, G: 163, B: 175, A: 255} // #9ca3af
	Gray200 = color.NRGBA{R: 229, G: 231, B: 235, A: 255} // #e5e7eb

	Yellow600 = color.NRGBA{R: 213, G: 153, B: 6, A: 255}  // #d59906
	Yellow500 = color.NRGBA{R: 246, G: 187, B: 12, A: 255} // #f6bb0c
	Yellow400 = color.NRGBA{R: 255, G: 211, B: 25, A: 255} // #ffd319

	ErrorRed     = color.NRGBA{R: 239, G: 68, B: 68, A: 255}  // #ef4444
	WarningAmber = color.NRGBA{R: 245, G: 158, B: 11, A: 255} // #f59e0b

	// ForegroundOnPrimary is text/icons on primary (yellow) buttons.
	ForegroundOnPrimary = Gray900
	// ForegroundOnAccent is for bright warm surfaces (success uses the same yellow as primary).
	ForegroundOnAccent = Gray900

	OverlayDark = color.NRGBA{R: 0, G: 0, B: 0, A: 180}
)

// Theme implements fyne.Theme with DeckCheck's dark palette.
type Theme struct{}

// New creates DeckCheck's dark theme.
func New() fyne.Theme {
	return &Theme{}
}

// Color returns the color for the specified theme color name.
func (t *Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Intentionally ignore variant: this theme is a single dark palette regardless of OS light/dark.
	switch name {
	case theme.ColorNameBackground:
		return Gray950
	case theme.ColorNameButton:
		return Gray700
	case theme.ColorNameDisabledButton:
		return Gray600
	case theme.ColorNameDisabled:
		return Gray500
	case theme.ColorNameError:
		return ErrorRed
	case theme.ColorNameFocus:
		return Yellow400
	case theme.ColorNameForeground:
		return color.White
	case theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameForegroundOnPrimary:
		return ForegroundOnPrimary
	case theme.ColorNameForegroundOnSuccess:
		return ForegroundOnAccent
	case theme.ColorNameForegroundOnWarning:
		return ForegroundOnAccent
	case theme.ColorNameHeaderBackground:
		return Gray900
	case theme.ColorNameHover:
		return Gray700
	case theme.ColorNameHyperlink:
		return Yellow400
	case theme.ColorNameInputBackground:
		return Gray800
	case theme.ColorNameInputBorder:
		return Gray600
	case theme.ColorNameMenuBackground:
		return Gray900
	case theme.ColorNameOverlayBackground:
		return OverlayDark
	case theme.ColorNamePlaceHolder:
		return Gray400
	case theme.ColorNamePressed:
		return Gray950
	case theme.ColorNamePrimary:
		return Yellow500
	case theme.ColorNameScrollBar:
		return Gray600
	case theme.ColorNameScrollBarBackground:
		return Gray950
	case theme.ColorNameSelection:
		return Yellow600
	case theme.ColorNameSeparator:
		return Gray600
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 100}
	case theme.ColorNameSuccess:
		return Yellow500
	case theme.ColorNameWarning:
		return WarningAmber
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

// Font returns the font for the specified text style.
func (t *Theme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon for the specified icon name.
func (t *Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size returns the size for the specified size name.
func (t *Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameLineSpacing:
		return 4
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 15
	case theme.SizeNameHeadingText:
		return 26
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 2
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 4
	default:
		return theme.DefaultTheme().Size(name)
	}
}
