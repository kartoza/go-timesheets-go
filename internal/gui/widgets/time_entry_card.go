package widgets

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TimeEntryData holds the data for a time entry card
type TimeEntryData struct {
	ID           int
	ProjectName  string
	TaskName     string
	ActivityName string
	Description  string
	StartTime    time.Time
	EndTime      *time.Time
	Hours        float64
	IsSubmitted  bool
	HasPOW       bool
}

// TimeEntryCard is a custom widget for displaying a time entry
type TimeEntryCard struct {
	widget.BaseWidget

	Data     TimeEntryData
	OnEdit   func()
	OnDelete func()
	OnView   func()

	bg           *canvas.Rectangle
	projectLabel *canvas.Text
	taskLabel    *canvas.Text
	dateLabel    *canvas.Text
	hoursLabel   *canvas.Text
	statusLabel  *canvas.Text
}

// NewTimeEntryCard creates a new time entry card widget
func NewTimeEntryCard(data TimeEntryData) *TimeEntryCard {
	c := &TimeEntryCard{
		Data: data,
	}
	c.ExtendBaseWidget(c)
	return c
}

// Update updates the card data
func (c *TimeEntryCard) Update(data TimeEntryData) {
	c.Data = data
	c.Refresh()
}

// Tapped handles tap events
func (c *TimeEntryCard) Tapped(*fyne.PointEvent) {
	if c.OnView != nil {
		c.OnView()
	}
}

// TappedSecondary handles secondary tap events
func (c *TimeEntryCard) TappedSecondary(*fyne.PointEvent) {}

// CreateRenderer creates the renderer for the time entry card
func (c *TimeEntryCard) CreateRenderer() fyne.WidgetRenderer {
	// Background
	c.bg = canvas.NewRectangle(color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF})
	c.bg.CornerRadius = 6
	c.bg.StrokeWidth = 1
	c.bg.StrokeColor = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}

	// Project name
	c.projectLabel = canvas.NewText("", color.White)
	c.projectLabel.TextSize = 14
	c.projectLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Task name
	c.taskLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	c.taskLabel.TextSize = 12

	// Date
	c.dateLabel = canvas.NewText("", color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF})
	c.dateLabel.TextSize = 12

	// Hours
	c.hoursLabel = canvas.NewText("", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	c.hoursLabel.TextSize = 14
	c.hoursLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Status
	c.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	c.statusLabel.TextSize = 10

	return &timeEntryCardRenderer{
		card:         c,
		bg:           c.bg,
		projectLabel: c.projectLabel,
		taskLabel:    c.taskLabel,
		dateLabel:    c.dateLabel,
		hoursLabel:   c.hoursLabel,
		statusLabel:  c.statusLabel,
	}
}

type timeEntryCardRenderer struct {
	card         *TimeEntryCard
	bg           *canvas.Rectangle
	projectLabel *canvas.Text
	taskLabel    *canvas.Text
	dateLabel    *canvas.Text
	hoursLabel   *canvas.Text
	statusLabel  *canvas.Text
}

func (r *timeEntryCardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	padding := float32(12)

	// Project label top-left
	r.projectLabel.Move(fyne.NewPos(padding, padding))

	// Task label below project
	r.taskLabel.Move(fyne.NewPos(padding, padding+18))

	// Date on right side
	r.dateLabel.Move(fyne.NewPos(size.Width-120, padding))

	// Hours on right side below date
	r.hoursLabel.Move(fyne.NewPos(size.Width-80, padding+18))

	// Status bottom-right
	r.statusLabel.Move(fyne.NewPos(size.Width-80, size.Height-20))
}

func (r *timeEntryCardRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 70)
}

func (r *timeEntryCardRenderer) Refresh() {
	d := r.card.Data

	r.projectLabel.Text = d.ProjectName
	r.taskLabel.Text = "▸ " + d.TaskName
	r.dateLabel.Text = d.StartTime.Local().Format("2006-01-02")
	r.hoursLabel.Text = fmt.Sprintf("%.1fh", d.Hours)

	// Status — BMP Unicode symbols + colour. The bundled Noto Sans font
	// covers these glyphs cleanly.
	statusText := ""
	if d.EndTime == nil {
		statusText = "● Running"
		r.statusLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
		r.bg.StrokeColor = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
	} else if d.IsSubmitted {
		statusText = "✓ Submitted"
		r.statusLabel.Color = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}
		r.bg.StrokeColor = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}
	} else {
		statusText = "○ Pending"
		r.statusLabel.Color = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
		r.bg.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	}
	if d.HasPOW {
		statusText = "▶ " + statusText
	}
	r.statusLabel.Text = statusText

	r.bg.Refresh()
	r.projectLabel.Refresh()
	r.taskLabel.Refresh()
	r.dateLabel.Refresh()
	r.hoursLabel.Refresh()
	r.statusLabel.Refresh()
}

func (r *timeEntryCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.projectLabel, r.taskLabel, r.dateLabel, r.hoursLabel, r.statusLabel}
}

func (r *timeEntryCardRenderer) Destroy() {}

// TimeEntryList creates a scrollable list of time entry cards
func TimeEntryList(entries []TimeEntryData, onSelect func(int)) fyne.CanvasObject {
	list := widget.NewList(
		func() int { return len(entries) },
		func() fyne.CanvasObject {
			return NewTimeEntryCard(TimeEntryData{})
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(entries) {
				obj.(*TimeEntryCard).Update(entries[id])
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if onSelect != nil {
			onSelect(int(id))
		}
	}
	return container.NewVScroll(list)
}
