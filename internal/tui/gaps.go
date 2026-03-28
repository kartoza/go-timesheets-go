package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// SegmentType represents the type of segment (gap or project)
type SegmentType int

const (
	SegmentGap SegmentType = iota
	SegmentProject
)

// Segment represents a selectable segment within a day
type Segment struct {
	Type    SegmentType
	Project *service.ProjectHourData // nil for gap segments
}

// GapsView displays a gap analysis with vertical bars and horizontal scrolling
type GapsView struct {
	apiClient   *api.Client
	gapsService *service.GapsService
	width       int
	height      int
	headerState *HeaderState

	// Data - contains all days from start of month to today
	dayData         []service.DayData
	daySegments     [][]Segment // Segments per day for navigation
	selectedDay     int         // Currently selected day index in dayData
	selectedSegment int         // Currently selected segment within day (0 = gap or first project)
	viewportOffset  int         // Offset for viewport scrolling (0 = showing most recent days)
	visibleDays     int         // Number of days visible in viewport (default 14)
	isLoading       bool
	err             error
	statusMessage   string
}

// NewGapsView creates a new gaps view
func NewGapsView(apiClient *api.Client) *GapsView {
	return &GapsView{
		apiClient:      apiClient,
		gapsService:    service.NewGapsService(8.0), // 8-hour target
		visibleDays:    14,                          // Show 14 days at a time
		viewportOffset: 0,                           // Start at the end (most recent days)
	}
}

// SetHeaderState sets the shared header state
func (g *GapsView) SetHeaderState(state *HeaderState) {
	g.headerState = state
}

// Init initializes the gaps view
func (g *GapsView) Init() tea.Cmd {
	g.isLoading = true
	return g.loadData()
}

// buildDaySegments builds the segment list for each day
func (g *GapsView) buildDaySegments() {
	g.daySegments = make([][]Segment, len(g.dayData))

	for i, day := range g.dayData {
		var segments []Segment

		// Add gap segment if there's a gap
		if day.GapHours > 0 {
			segments = append(segments, Segment{Type: SegmentGap})
		}

		// Add project segments
		projectList := g.gapsService.GetSortedProjects(&g.dayData[i])
		for _, proj := range projectList {
			segments = append(segments, Segment{
				Type:    SegmentProject,
				Project: proj,
			})
		}

		g.daySegments[i] = segments
	}
}

// Update handles messages
func (g *GapsView) Update(msg tea.Msg) (*GapsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.width = msg.Width
		g.height = msg.Height
		return g, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return g, tea.Quit

		case "esc":
			// Go back to main menu
			return g, func() tea.Msg { return backToMenuMsg{} }

		case "left", "h":
			if g.selectedDay > 0 {
				g.selectedDay--
				g.selectedSegment = 0 // Reset to first segment
				// Scroll viewport if needed
				viewStart := len(g.dayData) - g.visibleDays - g.viewportOffset
				if viewStart < 0 {
					viewStart = 0
				}
				if g.selectedDay < viewStart {
					g.viewportOffset++
				}
			}
			return g, nil

		case "right", "l":
			if g.selectedDay < len(g.dayData)-1 {
				g.selectedDay++
				g.selectedSegment = 0 // Reset to first segment
				// Scroll viewport if needed
				viewEnd := len(g.dayData) - g.viewportOffset
				if g.selectedDay >= viewEnd {
					g.viewportOffset--
					if g.viewportOffset < 0 {
						g.viewportOffset = 0
					}
				}
			}
			return g, nil

		case "home":
			// Jump to start of data (beginning of month)
			g.selectedDay = 0
			g.selectedSegment = 0
			// Scroll viewport to show start
			maxOffset := len(g.dayData) - g.visibleDays
			if maxOffset < 0 {
				maxOffset = 0
			}
			g.viewportOffset = maxOffset
			return g, nil

		case "end":
			// Jump to end of data (today)
			g.selectedDay = len(g.dayData) - 1
			g.selectedSegment = 0
			g.viewportOffset = 0
			return g, nil

		case "up", "k":
			// Move up within segments
			if g.selectedDay >= 0 && g.selectedDay < len(g.daySegments) {
				if g.selectedSegment > 0 {
					g.selectedSegment--
				}
			}
			return g, nil

		case "down", "j":
			// Move down within segments
			if g.selectedDay >= 0 && g.selectedDay < len(g.daySegments) {
				if g.selectedSegment < len(g.daySegments[g.selectedDay])-1 {
					g.selectedSegment++
				}
			}
			return g, nil

		case "enter", " ":
			// If selected segment is a gap, launch timesheet creator
			if g.selectedDay >= 0 && g.selectedDay < len(g.daySegments) {
				segments := g.daySegments[g.selectedDay]
				if g.selectedSegment >= 0 && g.selectedSegment < len(segments) {
					seg := segments[g.selectedSegment]
					if seg.Type == SegmentGap {
						day := &g.dayData[g.selectedDay]
						startTime := g.gapsService.CalculateGapStartTime(day, 9)
						endTime := g.gapsService.CalculateGapEndTime(day, startTime)
						return g, func() tea.Msg {
							return launchTimesheetForDateMsg{
								date:      day.Date,
								startTime: startTime,
								endTime:   endTime,
							}
						}
					}
				}
			}
			return g, nil

		case "r":
			// Refresh data
			g.isLoading = true
			return g, g.loadData()
		}

	case gapsDataLoadedMsg:
		g.isLoading = false
		if msg.err != nil {
			g.err = msg.err
		} else {
			g.dayData = msg.dayData
			g.buildDaySegments()
			g.err = nil
			// Select today (rightmost visible day) by default
			g.selectedDay = len(g.dayData) - 1
			g.viewportOffset = 0
		}
		return g, nil
	}

	return g, nil
}

// View renders the gaps view
func (g *GapsView) View() string {
	if g.width == 0 {
		return "Loading..."
	}

	// Render header
	header := g.renderHeader()

	// Container style
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(min(g.width-4, 80))

	var content string
	if g.isLoading {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Align(lipgloss.Center).
			Width(70).
			Render("Loading timesheet data...")
	} else if g.err != nil {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C")).
			Align(lipgloss.Center).
			Width(70).
			Render("Error: " + g.err.Error())
	} else {
		content = g.renderBars()
	}

	// Detail panel
	detailPanel := g.renderDetailPanel()

	// Legend
	legend := g.renderLegend()

	// Help text
	helpText := g.renderHelp()

	mainContent := containerStyle.Render(content)

	// Combine all parts
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		mainContent,
		"",
		detailPanel,
		"",
		legend,
	)

	// Center on screen
	centeredMain := lipgloss.Place(
		g.width,
		g.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	// Place help at bottom
	helpStyle := lipgloss.NewStyle().
		Width(g.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpStyle.Render(helpText),
	)
}

func (g *GapsView) renderHeader() string {
	if g.headerState != nil {
		return RenderHeader("Time Gaps", g.headerState)
	}
	return RenderHeader("Time Gaps", &HeaderState{})
}

// getVisibleDays returns the slice of days currently in the viewport
func (g *GapsView) getVisibleDays() ([]service.DayData, int) {
	if len(g.dayData) == 0 {
		return nil, 0
	}

	// Calculate viewport bounds
	viewEnd := len(g.dayData) - g.viewportOffset
	viewStart := viewEnd - g.visibleDays
	if viewStart < 0 {
		viewStart = 0
	}
	if viewEnd > len(g.dayData) {
		viewEnd = len(g.dayData)
	}

	return g.dayData[viewStart:viewEnd], viewStart
}

func (g *GapsView) renderBars() string {
	if len(g.dayData) == 0 {
		return "No data available"
	}

	visibleDays, viewStart := g.getVisibleDays()
	if len(visibleDays) == 0 {
		return "No data available"
	}

	barHeight := 12 // Number of rows for the bar
	barWidth := 4   // Width of each bar

	// Build rows from top to bottom
	var rows []string

	// Title row showing date range
	firstDay := visibleDays[0].Date
	lastDay := visibleDays[len(visibleDays)-1].Date
	titleText := fmt.Sprintf("%s - %s", firstDay.Format("Jan 02"), lastDay.Format("Jan 02"))

	// Add scroll indicators
	canScrollLeft := viewStart > 0
	canScrollRight := g.viewportOffset > 0
	if canScrollLeft {
		titleText = "← " + titleText
	}
	if canScrollRight {
		titleText = titleText + " →"
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(len(visibleDays) * (barWidth + 1))
	rows = append(rows, titleStyle.Render(titleText))
	rows = append(rows, "")

	// Hours labels row (top)
	var hoursRow strings.Builder
	for i, day := range visibleDays {
		actualIndex := viewStart + i
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Width(barWidth).
			Align(lipgloss.Center)
		if actualIndex == g.selectedDay {
			style = style.Foreground(lipgloss.Color("#DDA036")).Bold(true)
		}
		hoursRow.WriteString(style.Render(fmt.Sprintf("%.1f", day.TotalHours)))
		hoursRow.WriteString(" ")
	}
	rows = append(rows, hoursRow.String())

	// Build the bars row by row (from top to bottom)
	for row := 0; row < barHeight; row++ {
		var barRow strings.Builder
		for i, day := range visibleDays {
			actualIndex := viewStart + i
			char := g.getBarChar(day, row, barHeight)
			style := g.getBarStyle(day, row, barHeight, actualIndex == g.selectedDay, actualIndex)
			barRow.WriteString(style.Width(barWidth).Align(lipgloss.Center).Render(char))
			barRow.WriteString(" ")
		}
		rows = append(rows, barRow.String())
	}

	// Date labels row
	var dateRow strings.Builder
	for i, day := range visibleDays {
		actualIndex := viewStart + i
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(barWidth).
			Align(lipgloss.Center)
		if actualIndex == g.selectedDay {
			style = style.Foreground(lipgloss.Color("#DDA036")).Bold(true)
		}
		dateRow.WriteString(style.Render(day.Date.Format("02")))
		dateRow.WriteString(" ")
	}
	rows = append(rows, dateRow.String())

	// Day of week labels row
	var dowRow strings.Builder
	for i, day := range visibleDays {
		actualIndex := viewStart + i
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Width(barWidth).
			Align(lipgloss.Center)
		if actualIndex == g.selectedDay {
			style = style.Foreground(lipgloss.Color("#DDA036"))
		}
		dowRow.WriteString(style.Render(day.Date.Format("Mon")[:2]))
		dowRow.WriteString(" ")
	}
	rows = append(rows, dowRow.String())

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (g *GapsView) renderDetailPanel() string {
	if g.selectedDay < 0 || g.selectedDay >= len(g.daySegments) {
		return ""
	}

	segments := g.daySegments[g.selectedDay]
	if len(segments) == 0 {
		return ""
	}

	// Ensure selected segment is within bounds
	if g.selectedSegment >= len(segments) {
		g.selectedSegment = len(segments) - 1
	}
	if g.selectedSegment < 0 {
		g.selectedSegment = 0
	}

	seg := segments[g.selectedSegment]
	day := &g.dayData[g.selectedDay]

	// Detail box style
	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#569FC6")).
		Padding(0, 2).
		Width(min(g.width-4, 70))

	var lines []string

	if seg.Type == SegmentGap {
		// Gap segment details
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C")).
			Bold(true)
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Gap - %s", day.Date.Format("Mon Jan 02"))))
		lines = append(lines, "")

		hoursStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C"))
		lines = append(lines, hoursStyle.Render(fmt.Sprintf("Missing: %.2f hours to reach 8h target", day.GapHours)))

		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Italic(true)
		lines = append(lines, hintStyle.Render("Press Enter to create a timesheet entry for this gap"))
	} else if seg.Project != nil {
		// Project segment details
		proj := seg.Project

		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDA036")).
			Bold(true)
		lines = append(lines, titleStyle.Render(proj.ProjectName))

		// Collect unique tasks and activities
		tasks := make(map[string]bool)
		activities := make(map[string]bool)
		var descriptions []string
		missingDescCount := 0

		for _, entry := range proj.Entries {
			if entry.TaskName != "" {
				tasks[entry.TaskName] = true
			}
			if entry.ActivityName != "" {
				activities[entry.ActivityName] = true
			}
			if entry.Description != "" {
				descriptions = append(descriptions, entry.Description)
			} else if !entry.Submitted {
				missingDescCount++
			}
		}

		// Warning for missing descriptions
		if missingDescCount > 0 {
			warnStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E74C3C")).
				Bold(true)
			lines = append(lines, warnStyle.Render(fmt.Sprintf("⚠ %d entr%s missing description",
				missingDescCount,
				func() string {
					if missingDescCount == 1 {
						return "y"
					}
					return "ies"
				}())))
		}

		// Tasks
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0"))

		var taskList []string
		for t := range tasks {
			taskList = append(taskList, t)
		}
		if len(taskList) > 0 {
			taskStr := strings.Join(taskList, ", ")
			if len(taskStr) > 50 {
				taskStr = taskStr[:47] + "..."
			}
			lines = append(lines, labelStyle.Render("Task: ")+valueStyle.Render(taskStr))
		}

		// Activities
		var actList []string
		for a := range activities {
			actList = append(actList, a)
		}
		if len(actList) > 0 {
			actStr := strings.Join(actList, ", ")
			if len(actStr) > 50 {
				actStr = actStr[:47] + "..."
			}
			lines = append(lines, labelStyle.Render("Activity: ")+valueStyle.Render(actStr))
		}

		// Hours
		hoursStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true)
		lines = append(lines, hoursStyle.Render(fmt.Sprintf("Hours: %.2f", proj.Hours)))

		// Descriptions
		if len(descriptions) > 0 {
			desc := strings.Join(descriptions, " | ")
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			lines = append(lines, valueStyle.Italic(true).Render(desc))
		}
	}

	// Segment navigation indicator
	if len(segments) > 1 {
		navStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true)
		lines = append(lines, "")
		lines = append(lines, navStyle.Render(fmt.Sprintf("Segment %d of %d (↑/↓ to navigate)", g.selectedSegment+1, len(segments))))
	}

	return detailStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (g *GapsView) getBarChar(day service.DayData, row, totalRows int) string {
	// Calculate what portion of the bar this row represents
	// Row 0 is top, row totalRows-1 is bottom
	targetHours := g.gapsService.TargetHours
	hoursPerRow := targetHours / float64(totalRows)

	// Calculate the hour threshold for this row (from top)
	rowStartHours := targetHours - (float64(row) * hoursPerRow)
	rowEndHours := targetHours - (float64(row+1) * hoursPerRow)

	// Check if this row is in the gap zone (above logged hours)
	if day.TotalHours < rowStartHours {
		// This row is in the gap zone
		if day.TotalHours <= rowEndHours {
			// Fully in gap
			return "░"
		}
		// Partially in gap
		return "▒"
	}

	// This row is in the logged zone
	return "█"
}

func (g *GapsView) getBarStyle(day service.DayData, row, totalRows int, selected bool, dayIndex int) lipgloss.Style {
	targetHours := g.gapsService.TargetHours
	hoursPerRow := targetHours / float64(totalRows)
	rowStartHours := targetHours - (float64(row) * hoursPerRow)

	style := lipgloss.NewStyle()

	isGapRow := day.TotalHours < rowStartHours

	if isGapRow {
		// Gap zone - white
		style = style.Foreground(lipgloss.Color("#FFFFFF"))

		// Highlight if this is the selected day and gap is selected
		if selected && dayIndex < len(g.daySegments) {
			segments := g.daySegments[dayIndex]
			if len(segments) > 0 && g.selectedSegment < len(segments) && segments[g.selectedSegment].Type == SegmentGap {
				style = style.Bold(true).Background(lipgloss.Color("#3D3E40"))
			}
		}
	} else {
		// Logged zone - use project color or default green
		if len(day.ProjectHours) > 0 {
			// Get first project color
			for pid := range day.ProjectHours {
				style = style.Foreground(lipgloss.Color(service.GetProjectColorHex(pid)))
				break
			}
		} else {
			style = style.Foreground(lipgloss.Color("#2ECC71"))
		}

		// Highlight if this is the selected day and a project is selected
		if selected && dayIndex < len(g.daySegments) {
			segments := g.daySegments[dayIndex]
			if len(segments) > 0 && g.selectedSegment < len(segments) && segments[g.selectedSegment].Type == SegmentProject {
				style = style.Bold(true).Background(lipgloss.Color("#3D3E40"))
			}
		}
	}

	if selected && !isGapRow {
		style = style.Bold(true)
	}

	return style
}

func (g *GapsView) renderLegend() string {
	legendStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center)

	gapStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	loggedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))

	return legendStyle.Render(
		gapStyle.Render("░ Gap (unlogged)") + "  " +
			loggedStyle.Render("█ Logged hours") +
			"  Target: 8h/day",
	)
}

func (g *GapsView) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	return helpStyle.Render("←/→: Navigate days • Home/End: Jump to start/end • ↑/↓: Select segment • Enter: Fill gap • R: Refresh • Esc: Back")
}

func (g *GapsView) loadData() tea.Cmd {
	return func() tea.Msg {
		entries, err := g.apiClient.GetTimelogs()
		if err != nil {
			return gapsDataLoadedMsg{err: err}
		}

		// Calculate days from start of month to today
		now := time.Now()
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		daysInPeriod := int(now.Sub(startOfMonth).Hours()/24) + 1 // +1 to include today

		// Ensure at least 14 days are shown
		if daysInPeriod < 14 {
			daysInPeriod = 14
		}

		dayData := g.gapsService.CalculateGaps(entries, daysInPeriod)
		return gapsDataLoadedMsg{dayData: dayData}
	}
}

// Message types
type gapsDataLoadedMsg struct {
	dayData []service.DayData
	err     error
}

type launchTimesheetForDateMsg struct {
	date      time.Time
	startTime time.Time
	endTime   time.Time
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
