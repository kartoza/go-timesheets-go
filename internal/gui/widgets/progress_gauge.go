package widgets

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ProgressGauge is a custom widget showing daily progress toward a target
type ProgressGauge struct {
	widget.BaseWidget

	Current float64 // Hours worked today
	Target  float64 // Target hours (e.g., 8.0)

	percentLabel *canvas.Text
	hoursLabel   *canvas.Text
	progressBar  *canvas.Rectangle
	bgBar        *canvas.Rectangle
}

// NewProgressGauge creates a new progress gauge widget
func NewProgressGauge(current, target float64) *ProgressGauge {
	g := &ProgressGauge{
		Current: current,
		Target:  target,
	}
	g.ExtendBaseWidget(g)
	return g
}

// SetProgress updates the progress values
func (g *ProgressGauge) SetProgress(current, target float64) {
	g.Current = current
	g.Target = target
	g.Refresh()
}

// CreateRenderer creates the renderer for the progress gauge
func (g *ProgressGauge) CreateRenderer() fyne.WidgetRenderer {
	// Background bar
	g.bgBar = canvas.NewRectangle(color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF})
	g.bgBar.CornerRadius = 4

	// Progress bar
	g.progressBar = canvas.NewRectangle(color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF})
	g.progressBar.CornerRadius = 4

	// Percentage label
	g.percentLabel = canvas.NewText("0%", color.White)
	g.percentLabel.TextSize = 32
	g.percentLabel.TextStyle = fyne.TextStyle{Bold: true}
	g.percentLabel.Alignment = fyne.TextAlignCenter

	// Hours label
	g.hoursLabel = canvas.NewText("0.0h / 8.0h", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	g.hoursLabel.TextSize = 14
	g.hoursLabel.Alignment = fyne.TextAlignCenter

	return &progressGaugeRenderer{
		gauge:        g,
		bgBar:        g.bgBar,
		progressBar:  g.progressBar,
		percentLabel: g.percentLabel,
		hoursLabel:   g.hoursLabel,
	}
}

type progressGaugeRenderer struct {
	gauge        *ProgressGauge
	bgBar        *canvas.Rectangle
	progressBar  *canvas.Rectangle
	percentLabel *canvas.Text
	hoursLabel   *canvas.Text
}

func (r *progressGaugeRenderer) Layout(size fyne.Size) {
	barHeight := float32(20)
	barY := float32(60)
	padding := float32(20)

	// Background bar
	r.bgBar.Resize(fyne.NewSize(size.Width-padding*2, barHeight))
	r.bgBar.Move(fyne.NewPos(padding, barY))

	// Progress bar (width based on percentage)
	percentage := float32(0)
	if r.gauge.Target > 0 {
		percentage = float32(r.gauge.Current / r.gauge.Target)
		if percentage > 1 {
			percentage = 1
		}
	}
	progressWidth := (size.Width - padding*2) * percentage
	if progressWidth < 4 {
		progressWidth = 4
	}
	r.progressBar.Resize(fyne.NewSize(progressWidth, barHeight))
	r.progressBar.Move(fyne.NewPos(padding, barY))

	// Percentage label above bar
	r.percentLabel.Resize(fyne.NewSize(size.Width, 40))
	r.percentLabel.Move(fyne.NewPos(0, 15))

	// Hours label below bar
	r.hoursLabel.Resize(fyne.NewSize(size.Width, 20))
	r.hoursLabel.Move(fyne.NewPos(0, barY+barHeight+10))
}

func (r *progressGaugeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 120)
}

func (r *progressGaugeRenderer) Refresh() {
	g := r.gauge

	percentage := float64(0)
	if g.Target > 0 {
		percentage = (g.Current / g.Target) * 100
	}

	r.percentLabel.Text = fmt.Sprintf("%.0f%%", percentage)
	r.hoursLabel.Text = fmt.Sprintf("%.1fh / %.1fh", g.Current, g.Target)

	// Color based on progress
	if percentage >= 100 {
		r.progressBar.FillColor = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // Green
		r.percentLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
	} else if percentage >= 75 {
		r.progressBar.FillColor = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF} // Blue
		r.percentLabel.Color = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}
	} else if percentage >= 50 {
		r.progressBar.FillColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF} // Orange
		r.percentLabel.Color = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	} else {
		r.progressBar.FillColor = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF} // Red
		r.percentLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
	}

	// Update progress bar width
	if r.bgBar.Size().Width > 0 {
		pct := float32(g.Current / g.Target)
		if pct > 1 {
			pct = 1
		}
		progressWidth := r.bgBar.Size().Width * pct
		if progressWidth < 4 {
			progressWidth = 4
		}
		r.progressBar.Resize(fyne.NewSize(progressWidth, r.progressBar.Size().Height))
	}

	r.percentLabel.Refresh()
	r.hoursLabel.Refresh()
	r.progressBar.Refresh()
	r.bgBar.Refresh()
}

func (r *progressGaugeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bgBar, r.progressBar, r.percentLabel, r.hoursLabel}
}

func (r *progressGaugeRenderer) Destroy() {}

// ProgressBox creates a complete progress box with border and history/gaps buttons
func ProgressBox(gauge *ProgressGauge, historyButton *widget.Button, gapsButton *widget.Button) fyne.CanvasObject {
	title := canvas.NewText("Daily Progress", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Button row with both history and gaps buttons
	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		historyButton,
		gapsButton,
		layout.NewSpacer(),
	)

	content := container.NewVBox(
		container.NewCenter(title),
		gauge,
		widget.NewSeparator(),
		buttonRow,
	)

	// Create bordered container
	border := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	border.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	border.StrokeWidth = 2
	border.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	border.CornerRadius = 8

	return container.NewStack(border, container.NewPadded(content))
}
