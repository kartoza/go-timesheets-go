package widgets

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// CalendarPanel displays calendar events for a specific date
type CalendarPanel struct {
	Container fyne.CanvasObject

	// Service
	calendarService *service.CalendarService

	// State
	date      time.Time
	events    []models.CalendarEvent
	isLoading bool
	err       error

	// Widgets
	titleLabel  *canvas.Text
	eventsList  *fyne.Container
	statusLabel *canvas.Text
	loadingBar  *widget.ProgressBarInfinite
	setupButton *widget.Button
	refreshBtn  *widget.Button

	// Callbacks
	OnEventSelected func(event models.CalendarEvent) // Called when user clicks an event
	OnSetupClicked  func()                           // Called when user clicks setup
}

// NewCalendarPanel creates a new calendar events panel
func NewCalendarPanel(calendarService *service.CalendarService) *CalendarPanel {
	p := &CalendarPanel{
		calendarService: calendarService,
	}
	p.build()
	return p
}

func (p *CalendarPanel) build() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	bgColor := color.NRGBA{R: 0x2D, G: 0x2D, B: 0x2D, A: 0xFF}

	// Title
	p.titleLabel = canvas.NewText("Calendar Events", goldColor)
	p.titleLabel.TextSize = 14
	p.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Loading bar
	p.loadingBar = widget.NewProgressBarInfinite()
	p.loadingBar.Hide()

	// Status label
	p.statusLabel = canvas.NewText("", grayColor)
	p.statusLabel.TextSize = 11

	// Events list container
	p.eventsList = container.NewVBox()

	// Setup button (shown when not connected)
	p.setupButton = widget.NewButton("Setup Google Calendar", func() {
		if p.OnSetupClicked != nil {
			p.OnSetupClicked()
		}
	})
	p.setupButton.Hide()

	// Refresh button
	p.refreshBtn = widget.NewButton("Refresh", func() {
		p.LoadEvents(p.date)
	})
	p.refreshBtn.Importance = widget.LowImportance

	// Hint text
	hintLabel := canvas.NewText("Click an event to use its times", grayColor)
	hintLabel.TextSize = 10
	hintLabel.Alignment = fyne.TextAlignCenter

	// Build layout
	header := container.NewVBox(
		container.NewCenter(p.titleLabel),
		p.loadingBar,
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(hintLabel),
		container.NewCenter(p.refreshBtn),
		container.NewCenter(p.setupButton),
		container.NewCenter(p.statusLabel),
	)

	// Scrollable events area
	scrollContent := container.NewVBox(p.eventsList)
	scroll := container.NewVScroll(scrollContent)
	scroll.SetMinSize(fyne.NewSize(200, 150))

	// Main content
	content := container.NewBorder(
		header,
		footer,
		nil,
		nil,
		scroll,
	)

	// Background
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 6

	border := canvas.NewRectangle(goldColor)
	border.StrokeColor = goldColor
	border.StrokeWidth = 1
	border.FillColor = color.Transparent
	border.CornerRadius = 6

	p.Container = container.NewStack(
		bg,
		border,
		container.NewPadded(content),
	)
}

// SetDate sets the date and loads events
func (p *CalendarPanel) SetDate(date time.Time) {
	p.date = date
	p.titleLabel.Text = fmt.Sprintf("Calendar - %s", date.Format("Mon Jan 2"))
	p.titleLabel.Refresh()
	p.LoadEvents(date)
}

// LoadEvents loads calendar events for the specified date
func (p *CalendarPanel) LoadEvents(date time.Time) {
	p.date = date

	// Check if service is available and connected
	if p.calendarService == nil {
		p.showNotConnected()
		return
	}

	if !p.calendarService.IsConnected() {
		p.showNotConnected()
		return
	}

	// Show loading state
	p.isLoading = true
	p.loadingBar.Show()
	p.statusLabel.Text = "Loading events..."
	p.statusLabel.Refresh()
	p.setupButton.Hide()

	// Load events asynchronously
	util.RunAsync(
		func() ([]models.CalendarEvent, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return p.calendarService.GetEventsForDay(ctx, date)
		},
		func(events []models.CalendarEvent, err error) {
			p.isLoading = false
			p.loadingBar.Hide()

			if err != nil {
				p.err = err
				p.statusLabel.Text = "Error loading events"
				p.statusLabel.Refresh()
				p.showError(err)
				return
			}

			p.events = events
			p.err = nil
			p.renderEvents()
		},
	)
}

func (p *CalendarPanel) showNotConnected() {
	p.eventsList.Objects = nil

	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}

	msgLabel := canvas.NewText("Google Calendar not connected", grayColor)
	msgLabel.TextSize = 11
	msgLabel.Alignment = fyne.TextAlignCenter

	p.eventsList.Add(container.NewCenter(msgLabel))
	p.eventsList.Add(layout.NewSpacer())

	p.setupButton.Show()
	p.refreshBtn.Hide()
	p.statusLabel.Text = ""
	p.statusLabel.Refresh()
	p.eventsList.Refresh()
}

func (p *CalendarPanel) showError(err error) {
	p.eventsList.Objects = nil

	redColor := color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}

	errLabel := canvas.NewText("Failed to load events", redColor)
	errLabel.TextSize = 11
	errLabel.Alignment = fyne.TextAlignCenter

	detailLabel := canvas.NewText(err.Error(), color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	detailLabel.TextSize = 9
	detailLabel.Alignment = fyne.TextAlignCenter

	p.eventsList.Add(container.NewCenter(errLabel))
	p.eventsList.Add(container.NewCenter(detailLabel))
	p.eventsList.Add(layout.NewSpacer())

	p.refreshBtn.Show()
	p.setupButton.Hide()
	p.eventsList.Refresh()
}

func (p *CalendarPanel) renderEvents() {
	p.eventsList.Objects = nil

	if len(p.events) == 0 {
		grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
		noEventsLabel := canvas.NewText("No events for this day", grayColor)
		noEventsLabel.TextSize = 11
		noEventsLabel.Alignment = fyne.TextAlignCenter
		p.eventsList.Add(container.NewCenter(noEventsLabel))
		p.statusLabel.Text = ""
	} else {
		for _, event := range p.events {
			eventWidget := p.createEventWidget(event)
			p.eventsList.Add(eventWidget)
		}
		p.statusLabel.Text = fmt.Sprintf("%d events", len(p.events))
	}

	p.refreshBtn.Show()
	p.setupButton.Hide()
	p.statusLabel.Refresh()
	p.eventsList.Refresh()
}

func (p *CalendarPanel) createEventWidget(event models.CalendarEvent) fyne.CanvasObject {
	whiteColor := color.White
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	hoverColor := color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}

	// Time range
	timeLabel := canvas.NewText(event.FormatTimeRange(), whiteColor)
	timeLabel.TextSize = 11
	timeLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Title (truncate if too long)
	title := event.Title
	if len(title) > 25 {
		title = title[:22] + "..."
	}
	titleLabel := canvas.NewText(title, grayColor)
	titleLabel.TextSize = 10

	// Duration
	durationLabel := canvas.NewText(fmt.Sprintf("%.1fh", event.DurationHours()), grayColor)
	durationLabel.TextSize = 9

	// Layout
	content := container.NewVBox(
		timeLabel,
		titleLabel,
	)

	// Background for hover effect
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4

	// Create clickable button
	btn := widget.NewButton("", func() {
		if p.OnEventSelected != nil {
			p.OnEventSelected(event)
		}
	})
	btn.Importance = widget.LowImportance

	// Stack the content on top of the button
	stack := container.NewStack(
		bg,
		container.NewPadded(content),
		btn,
	)

	// Color indicator on left (if event has color)
	var colorIndicator fyne.CanvasObject
	if event.ColorHex != "" {
		colorRect := canvas.NewRectangle(parseHexColor(event.ColorHex))
		colorRect.SetMinSize(fyne.NewSize(4, 0))
		colorIndicator = colorRect
	} else {
		colorIndicator = layout.NewSpacer()
	}

	// Final container with color indicator
	_ = hoverColor // Will be used in future hover implementation

	return container.NewBorder(nil, nil, colorIndicator, durationLabel, stack)
}

// parseHexColor converts hex color string to color.Color
func parseHexColor(hex string) color.Color {
	if len(hex) < 7 {
		return color.NRGBA{R: 0x42, G: 0x85, B: 0xF4, A: 0xFF} // Default blue
	}

	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.NRGBA{R: r, G: g, B: b, A: 0xFF}
}

// SetCalendarService updates the calendar service
func (p *CalendarPanel) SetCalendarService(service *service.CalendarService) {
	p.calendarService = service
	if p.date.IsZero() {
		return
	}
	p.LoadEvents(p.date)
}
