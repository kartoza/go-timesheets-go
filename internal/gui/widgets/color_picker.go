package widgets

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Preset colors for the colour picker (curated palette)
var presetColors = []struct {
	Name  string
	Color color.NRGBA
	Hex   string
}{
	{"Ubuntu Orange", color.NRGBA{R: 0xE9, G: 0x54, B: 0x20, A: 0xFF}, "#E95420"},
	{"Red", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}, "#E74C3C"},
	{"Coral", color.NRGBA{R: 0xFF, G: 0x6B, B: 0x6B, A: 0xFF}, "#FF6B6B"},
	{"Rose", color.NRGBA{R: 0xFD, G: 0x79, B: 0xA8, A: 0xFF}, "#FD79A8"},
	{"Purple", color.NRGBA{R: 0x9B, G: 0x59, B: 0xB6, A: 0xFF}, "#9B59B6"},
	{"Violet", color.NRGBA{R: 0xA2, G: 0x9B, B: 0xFE, A: 0xFF}, "#A29BFE"},
	{"Indigo", color.NRGBA{R: 0x63, G: 0x5E, B: 0xC0, A: 0xFF}, "#635EC0"},
	{"Blue", color.NRGBA{R: 0x34, G: 0x98, B: 0xDB, A: 0xFF}, "#3498DB"},
	{"Sky", color.NRGBA{R: 0x74, G: 0xB9, B: 0xFF, A: 0xFF}, "#74B9FF"},
	{"Cyan", color.NRGBA{R: 0x00, G: 0xCE, B: 0xC9, A: 0xFF}, "#00CEC9"},
	{"Teal", color.NRGBA{R: 0x1A, G: 0xBC, B: 0x9C, A: 0xFF}, "#1ABC9C"},
	{"Green", color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}, "#2ECC71"},
	{"Lime", color.NRGBA{R: 0x00, G: 0xB8, B: 0x94, A: 0xFF}, "#00B894"},
	{"Yellow", color.NRGBA{R: 0xFD, G: 0xCB, B: 0x6E, A: 0xFF}, "#FDCB6E"},
	{"Gold", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}, "#DDA036"},
	{"Amber", color.NRGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF}, "#F39C12"},
	{"Brown", color.NRGBA{R: 0xA0, G: 0x6E, B: 0x3C, A: 0xFF}, "#A06E3C"},
	{"Slate", color.NRGBA{R: 0x63, G: 0x6E, B: 0x72, A: 0xFF}, "#636E72"},
	{"Default", color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}, "#569FC6"},
}

// ColorPicker is a simple grid-based colour picker widget
type ColorPicker struct {
	widget.BaseWidget

	Selected  string // hex color string
	OnChanged func(hex string)

	swatches []*canvas.Rectangle
	preview  *canvas.Rectangle
	hexLabel *canvas.Text
}

// NewColorPicker creates a new colour picker
func NewColorPicker(initialHex string, onChanged func(hex string)) *ColorPicker {
	cp := &ColorPicker{
		Selected:  initialHex,
		OnChanged: onChanged,
	}
	cp.ExtendBaseWidget(cp)
	return cp
}

// CreateRenderer creates the renderer for the colour picker
func (cp *ColorPicker) CreateRenderer() fyne.WidgetRenderer {
	// Preview swatch
	cp.preview = canvas.NewRectangle(color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF})
	cp.preview.SetMinSize(fyne.NewSize(30, 30))
	cp.preview.CornerRadius = 4

	if cp.Selected != "" {
		cp.preview.FillColor = ParseHexColor(cp.Selected)
	}

	cp.hexLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	cp.hexLabel.TextSize = 11
	if cp.Selected != "" {
		cp.hexLabel.Text = cp.Selected
	}

	// Build swatch grid
	swatchGrid := container.NewGridWithColumns(6)
	for _, pc := range presetColors {
		pc := pc // capture
		swatch := canvas.NewRectangle(pc.Color)
		swatch.SetMinSize(fyne.NewSize(28, 28))
		swatch.CornerRadius = 4

		// Wrap in a tappable button
		btn := newColorSwatchButton(swatch, func() {
			cp.Selected = pc.Hex
			cp.preview.FillColor = pc.Color
			cp.preview.Refresh()
			cp.hexLabel.Text = pc.Hex
			cp.hexLabel.Refresh()
			if cp.OnChanged != nil {
				cp.OnChanged(pc.Hex)
			}
		})
		swatchGrid.Add(btn)
	}

	content := container.NewVBox(
		container.NewHBox(cp.preview, cp.hexLabel),
		swatchGrid,
	)

	return widget.NewSimpleRenderer(content)
}

// ParseHexColor parses a hex color string like "#E95420" into color.NRGBA
func ParseHexColor(hex string) color.NRGBA {
	if len(hex) == 0 {
		return color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF} // default blue
	}
	if hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return color.NRGBA{R: r, G: g, B: b, A: 0xFF}
}

// colorSwatchButton is a tappable colour swatch
type colorSwatchButton struct {
	widget.BaseWidget
	swatch   *canvas.Rectangle
	onTapped func()
}

func newColorSwatchButton(swatch *canvas.Rectangle, onTapped func()) *colorSwatchButton {
	b := &colorSwatchButton{swatch: swatch, onTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *colorSwatchButton) Tapped(*fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

func (b *colorSwatchButton) TappedSecondary(*fyne.PointEvent) {}

func (b *colorSwatchButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(b.swatch))
}
