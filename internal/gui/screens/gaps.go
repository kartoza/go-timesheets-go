package screens

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// GapsScreen shows a 14-day view with vertical bars representing logged hours
type GapsScreen struct {
	Container fyne.CanvasObject

	apiClient   *api.Client
	window      fyne.Window
	gapsService *service.GapsService

	// Widgets
	barContainer  *fyne.Container
	detailPanel   *fyne.Container
	statusLabel   *canvas.Text
	loadingBar    *widget.ProgressBarInfinite

	// Detail panel labels
	detailProject     *canvas.Text
	detailTask        *canvas.Text
	detailActivity    *canvas.Text
	detailHours       *canvas.Text
	detailDescription *canvas.Text

	// Data
	dayData []service.DayData

	// Callbacks
	OnBack              func()
	OnStartTimesheetFor func(date time.Time, startTime time.Time) // Called when clicking a gap
}

// NewGapsScreen creates a new gaps screen
func NewGapsScreen(apiClient *api.Client, window fyne.Window) *GapsScreen {
	s := &GapsScreen{
		apiClient:   apiClient,
		window:      window,
		gapsService: service.NewGapsService(8.0), // 8-hour target
	}
	s.build()
	return s
}

func (s *GapsScreen) build() {
	// Title
	title := canvas.NewText("Time Gaps - Last 14 Days", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Back button
	backButton := widget.NewButton("< Back", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Header row
	header := container.NewBorder(nil, nil, backButton, nil, container.NewCenter(title))

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Loading bar
	s.loadingBar = widget.NewProgressBarInfinite()
	s.loadingBar.Hide()

	// Bar container - will hold the 14 vertical bars
	s.barContainer = container.NewHBox()

	// Detail panel - shows info about hovered segment
	s.detailProject = canvas.NewText("", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	s.detailProject.TextSize = 14
	s.detailProject.TextStyle = fyne.TextStyle{Bold: true}

	s.detailTask = canvas.NewText("", color.White)
	s.detailTask.TextSize = 12

	s.detailActivity = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.detailActivity.TextSize = 11

	s.detailHours = canvas.NewText("", color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF})
	s.detailHours.TextSize = 12
	s.detailHours.TextStyle = fyne.TextStyle{Bold: true}

	s.detailDescription = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.detailDescription.TextSize = 11

	detailBorder := canvas.NewRectangle(color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF})
	detailBorder.CornerRadius = 4

	s.detailPanel = container.NewVBox(
		s.detailProject,
		s.detailTask,
		s.detailActivity,
		s.detailHours,
		s.detailDescription,
	)

	detailBox := container.NewStack(
		detailBorder,
		container.NewPadded(s.detailPanel),
	)
	detailBox.Resize(fyne.NewSize(400, 100))

	// Set initial placeholder
	s.showDetailPlaceholder()

	// Legend
	legendTitle := canvas.NewText("Legend:", color.White)
	legendTitle.TextSize = 12

	gapLegend := container.NewHBox(
		canvas.NewRectangle(color.White),
		canvas.NewText("Gap (click to fill)", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}),
	)
	gapLegend.Objects[0].(*canvas.Rectangle).SetMinSize(fyne.NewSize(16, 16))

	legend := container.NewHBox(
		layout.NewSpacer(),
		legendTitle,
		gapLegend,
		layout.NewSpacer(),
	)

	// Instructions
	instructions := canvas.NewText("Hover over segments to see details. Click red gaps to fill them.", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	instructions.TextSize = 11
	instructions.Alignment = fyne.TextAlignCenter

	// Main layout
	s.Container = container.NewBorder(
		container.NewVBox(
			header,
			widget.NewSeparator(),
			s.loadingBar,
			container.NewCenter(s.statusLabel),
		),
		container.NewVBox(
			container.NewCenter(detailBox),
			widget.NewSeparator(),
			legend,
			container.NewCenter(instructions),
		),
		nil,
		nil,
		container.NewCenter(container.NewPadded(s.barContainer)),
	)
}

func (s *GapsScreen) showDetailPlaceholder() {
	s.detailProject.Text = "Hover over a segment to see details"
	s.detailProject.Refresh()
	s.detailTask.Text = ""
	s.detailTask.Refresh()
	s.detailActivity.Text = ""
	s.detailActivity.Refresh()
	s.detailHours.Text = ""
	s.detailHours.Refresh()
	s.detailDescription.Text = ""
	s.detailDescription.Refresh()
}

func (s *GapsScreen) showProjectDetail(proj *service.ProjectHourData) {
	s.detailProject.Text = proj.ProjectName
	s.detailProject.Refresh()

	// Collect unique tasks and activities
	tasks := make(map[string]bool)
	activities := make(map[string]bool)
	var descriptions []string

	for _, entry := range proj.Entries {
		if entry.TaskName != "" {
			tasks[entry.TaskName] = true
		}
		if entry.ActivityName != "" {
			activities[entry.ActivityName] = true
		}
		if entry.Description != "" {
			descriptions = append(descriptions, entry.Description)
		}
	}

	// Format tasks
	var taskList []string
	for t := range tasks {
		taskList = append(taskList, t)
	}
	if len(taskList) > 0 {
		s.detailTask.Text = "Task: " + strings.Join(taskList, ", ")
	} else {
		s.detailTask.Text = "Task: -"
	}
	s.detailTask.Refresh()

	// Format activities
	var actList []string
	for a := range activities {
		actList = append(actList, a)
	}
	if len(actList) > 0 {
		s.detailActivity.Text = "Activity: " + strings.Join(actList, ", ")
	} else {
		s.detailActivity.Text = "Activity: -"
	}
	s.detailActivity.Refresh()

	// Hours
	s.detailHours.Text = fmt.Sprintf("Hours: %.2f", proj.Hours)
	s.detailHours.Refresh()

	// Description (truncate if too long)
	if len(descriptions) > 0 {
		desc := strings.Join(descriptions, " | ")
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}
		s.detailDescription.Text = desc
	} else {
		s.detailDescription.Text = "(no description)"
	}
	s.detailDescription.Refresh()
}

func (s *GapsScreen) showGapDetail(day *service.DayData) {
	s.detailProject.Text = fmt.Sprintf("Gap - %s", day.Date.Format("Mon Jan 02"))
	s.detailProject.Refresh()
	s.detailTask.Text = ""
	s.detailTask.Refresh()
	s.detailActivity.Text = ""
	s.detailActivity.Refresh()
	s.detailHours.Text = fmt.Sprintf("Missing: %.2f hours to reach 8h target", day.GapHours)
	s.detailHours.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
	s.detailHours.Refresh()
	s.detailDescription.Text = "Click to create a timesheet entry for this gap"
	s.detailDescription.Refresh()
}

// Refresh loads data and rebuilds the bars
func (s *GapsScreen) Refresh() {
	s.loadingBar.Show()
	s.setStatus("Loading timesheet data...")

	util.RunAsync(
		func() ([]api.TimelogEntry, error) {
			return s.apiClient.GetTimelogs()
		},
		func(entries []api.TimelogEntry, err error) {
			s.loadingBar.Hide()
			if err != nil {
				s.setStatus("Error loading data: " + err.Error())
				return
			}
			s.dayData = s.gapsService.CalculateGaps(entries, 14)
			s.buildBars()
			s.setStatus(fmt.Sprintf("Showing %d days of data", len(s.dayData)))
		},
	)
}

// getProjectColor converts a hex color string to color.Color
func (s *GapsScreen) getProjectColor(projectID int) color.Color {
	colors := []color.NRGBA{
		{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}, // Green
		{R: 0x34, G: 0x98, B: 0xDB, A: 0xFF}, // Blue
		{R: 0x9B, G: 0x59, B: 0xB6, A: 0xFF}, // Purple
		{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF}, // Orange
		{R: 0x1A, G: 0xBC, B: 0x9C, A: 0xFF}, // Teal
		{R: 0xE9, G: 0x1E, B: 0x63, A: 0xFF}, // Pink
		{R: 0x00, G: 0xBC, B: 0xD4, A: 0xFF}, // Cyan
		{R: 0xFF, G: 0xC1, B: 0x07, A: 0xFF}, // Amber
	}
	return colors[service.GetProjectColorIndex(projectID)]
}

// hoverableSegment is a custom widget that detects mouse hover
type hoverableSegment struct {
	widget.BaseWidget
	rect      *canvas.Rectangle
	onHover   func()
	onUnhover func()
	onClick   func()
}

func newHoverableSegment(rect *canvas.Rectangle, onHover, onUnhover, onClick func()) *hoverableSegment {
	h := &hoverableSegment{
		rect:      rect,
		onHover:   onHover,
		onUnhover: onUnhover,
		onClick:   onClick,
	}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hoverableSegment) CreateRenderer() fyne.WidgetRenderer {
	return &hoverableSegmentRenderer{segment: h}
}

func (h *hoverableSegment) MouseIn(*desktop.MouseEvent) {
	if h.onHover != nil {
		h.onHover()
	}
}

func (h *hoverableSegment) MouseMoved(*desktop.MouseEvent) {}

func (h *hoverableSegment) MouseOut() {
	if h.onUnhover != nil {
		h.onUnhover()
	}
}

func (h *hoverableSegment) Tapped(*fyne.PointEvent) {
	if h.onClick != nil {
		h.onClick()
	}
}

type hoverableSegmentRenderer struct {
	segment *hoverableSegment
}

func (r *hoverableSegmentRenderer) Layout(size fyne.Size) {
	r.segment.rect.Resize(size)
}

func (r *hoverableSegmentRenderer) MinSize() fyne.Size {
	return r.segment.rect.MinSize()
}

func (r *hoverableSegmentRenderer) Refresh() {
	r.segment.rect.Refresh()
}

func (r *hoverableSegmentRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.segment.rect}
}

func (r *hoverableSegmentRenderer) Destroy() {}

func (s *GapsScreen) buildBars() {
	s.barContainer.Objects = nil

	barWidth := float32(28)
	barMaxHeight := float32(200)

	for i := range s.dayData {
		day := &s.dayData[i]

		// Create segments container for this day's bar
		segments := container.NewVBox()

		// Get sorted projects for consistent display
		projectList := s.gapsService.GetSortedProjects(day)

		// Add gap segment at top (if there's a gap)
		if day.GapHours > 0 {
			gapHeight := float32(day.GapHours/s.gapsService.TargetHours) * barMaxHeight
			if gapHeight < 8 {
				gapHeight = 8
			}
			gapRect := canvas.NewRectangle(color.White)
			gapRect.SetMinSize(fyne.NewSize(barWidth, gapHeight))

			dayIndex := i
			dayPtr := day
			gapSegment := newHoverableSegment(
				gapRect,
				func() { s.showGapDetail(dayPtr) },
				func() { s.showDetailPlaceholder() },
				func() { s.onGapClicked(dayIndex) },
			)
			segments.Add(gapSegment)
		}

		// Add project segments (from top to bottom)
		for _, proj := range projectList {
			segHeight := float32(proj.Hours/s.gapsService.TargetHours) * barMaxHeight
			if segHeight < 4 {
				segHeight = 4
			}
			projRect := canvas.NewRectangle(s.getProjectColor(proj.ProjectID))
			projRect.SetMinSize(fyne.NewSize(barWidth, segHeight))

			projPtr := proj
			projSegment := newHoverableSegment(
				projRect,
				func() { s.showProjectDetail(projPtr) },
				func() { s.showDetailPlaceholder() },
				nil,
			)
			segments.Add(projSegment)
		}

		// Add empty space if bar is very short
		if day.TotalHours < 0.5 && day.GapHours <= 0 {
			emptyRect := canvas.NewRectangle(color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF})
			emptyRect.SetMinSize(fyne.NewSize(barWidth, barMaxHeight))
			segments.Add(emptyRect)
		}

		// Date label
		dateLabel := canvas.NewText(day.Date.Format("02"), color.White)
		dateLabel.TextSize = 10
		dateLabel.Alignment = fyne.TextAlignCenter

		// Day of week label
		dowLabel := canvas.NewText(day.Date.Format("Mon"), color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
		dowLabel.TextSize = 9
		dowLabel.Alignment = fyne.TextAlignCenter

		// Hours label
		hoursLabel := canvas.NewText(fmt.Sprintf("%.1f", day.TotalHours), color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
		hoursLabel.TextSize = 9
		hoursLabel.Alignment = fyne.TextAlignCenter

		// Container for one day's column
		dayColumn := container.NewVBox(
			hoursLabel,
			segments,
			dateLabel,
			dowLabel,
		)

		// Add border/background
		border := canvas.NewRectangle(color.NRGBA{R: 0x2D, G: 0x2D, B: 0x2D, A: 0xFF})
		border.CornerRadius = 4

		dayBox := container.NewStack(border, container.NewPadded(dayColumn))
		s.barContainer.Add(dayBox)
	}

	s.barContainer.Refresh()
}

func (s *GapsScreen) onGapClicked(dayIndex int) {
	if dayIndex < 0 || dayIndex >= len(s.dayData) {
		return
	}

	day := &s.dayData[dayIndex]
	startTime := s.gapsService.CalculateGapStartTime(day, 9) // 9 AM default start

	if s.OnStartTimesheetFor != nil {
		s.OnStartTimesheetFor(day.Date, startTime)
	}
}

func (s *GapsScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}
