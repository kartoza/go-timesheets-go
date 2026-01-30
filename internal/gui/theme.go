package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Brand colors matching TUI (from internal/tui/widgets.go)
var (
	ColorOrange   = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF} // #DDA036 - Primary/accent
	ColorBlue     = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF} // #569FC6 - Focus/links
	ColorGray     = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF} // #9A9EA0 - Secondary text
	ColorWhite    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // #FFFFFF - Primary text
	ColorDarkGray = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF} // #3D3E40 - Disabled/background
	ColorRed      = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF} // #E74C3C - Errors/stop
	ColorGreen    = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // #2ECC71 - Success/running
	ColorBgDark   = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF} // Dark background
)

// KartozaTheme implements fyne.Theme with Ubuntu-inspired colors
type KartozaTheme struct{}

var _ fyne.Theme = (*KartozaTheme)(nil)

// NewUbuntuTheme creates a new Ubuntu-inspired theme
func NewUbuntuTheme() fyne.Theme {
	return &KartozaTheme{}
}

// Color returns the color for the specified theme color name
func (t *KartozaTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return ColorOrange
	case theme.ColorNameFocus:
		return ColorBlue
	case theme.ColorNameSelection:
		return ColorBlue
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0x40} // Orange with alpha
	case theme.ColorNameButton:
		return ColorDarkGray
	case theme.ColorNameDisabled:
		return ColorGray
	case theme.ColorNameError:
		return ColorRed
	case theme.ColorNameSuccess:
		return ColorGreen
	case theme.ColorNameForeground:
		return ColorWhite
	case theme.ColorNameBackground:
		return ColorBgDark
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	case theme.ColorNamePlaceHolder:
		return ColorGray
	case theme.ColorNameScrollBar:
		return ColorGray
	case theme.ColorNameSeparator:
		return ColorDarkGray
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66}
	case theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	default:
		// Force dark variant for any unhandled color names
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

// Font returns the font for the specified text style
func (t *KartozaTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon for the specified icon name
func (t *KartozaTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size returns the size for the specified size name
func (t *KartozaTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 4
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	default:
		return theme.DefaultTheme().Size(name)
	}
}
