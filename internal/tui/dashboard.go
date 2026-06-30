package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LCD digit patterns (5 rows high, 3 chars wide)
var lcdDigits = map[rune][]string{
	'0': {
		"┏━┓",
		"┃ ┃",
		"┃ ┃",
		"┃ ┃",
		"┗━┛",
	},
	'1': {
		"  ┓",
		"  ┃",
		"  ┃",
		"  ┃",
		"  ╹",
	},
	'2': {
		"╺━┓",
		"  ┃",
		"┏━┛",
		"┃  ",
		"┗━╸",
	},
	'3': {
		"╺━┓",
		"  ┃",
		"╺━┫",
		"  ┃",
		"╺━┛",
	},
	'4': {
		"╻ ╻",
		"┃ ┃",
		"┗━┫",
		"  ┃",
		"  ╹",
	},
	'5': {
		"┏━╸",
		"┃  ",
		"┗━┓",
		"  ┃",
		"╺━┛",
	},
	'6': {
		"┏━╸",
		"┃  ",
		"┣━┓",
		"┃ ┃",
		"┗━┛",
	},
	'7': {
		"╺━┓",
		"  ┃",
		"  ┃",
		"  ┃",
		"  ╹",
	},
	'8': {
		"┏━┓",
		"┃ ┃",
		"┣━┫",
		"┃ ┃",
		"┗━┛",
	},
	'9': {
		"┏━┓",
		"┃ ┃",
		"┗━┫",
		"  ┃",
		"╺━┛",
	},
	':': {
		"   ",
		" ● ",
		"   ",
		" ● ",
		"   ",
	},
	'h': {
		"   ",
		"   ",
		"   ",
		"   ",
		" h ",
	},
	'm': {
		"   ",
		"   ",
		"   ",
		"   ",
		" m ",
	},
	' ': {
		"   ",
		"   ",
		"   ",
		"   ",
		"   ",
	},
}

// Blinking colon pattern (empty/hidden)
var lcdColonOff = []string{
	"   ",
	"   ",
	"   ",
	"   ",
	"   ",
}

// RenderLCDTime renders time in large LCD-style digits
func RenderLCDTime(hours, minutes int, color lipgloss.Color, blinkOn bool) string {
	// Format as HH:MM
	timeStr := fmt.Sprintf("%02d:%02d", hours, minutes)

	// Build each row
	rows := make([]string, 5)
	for _, char := range timeStr {
		var pattern []string
		if char == ':' && !blinkOn {
			pattern = lcdColonOff
		} else {
			var ok bool
			pattern, ok = lcdDigits[char]
			if !ok {
				pattern = lcdDigits[' ']
			}
		}
		for i := 0; i < 5; i++ {
			rows[i] += pattern[i] + " "
		}
	}

	// Style and join
	style := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	var styledRows []string
	for _, row := range rows {
		styledRows = append(styledRows, style.Render(row))
	}

	return strings.Join(styledRows, "\n")
}

// RenderLCDNumber renders a number with optional suffix in LCD style
func RenderLCDNumber(value float64, suffix string, color lipgloss.Color) string {
	// Format as X.X or XX.X
	var numStr string
	if value < 10 {
		numStr = fmt.Sprintf("%.1f", value)
	} else {
		numStr = fmt.Sprintf("%.1f", value)
	}

	// Replace . with custom pattern
	numStr = strings.Replace(numStr, ".", ":", 1) // Use colon as decimal point visual

	// Build each row
	rows := make([]string, 5)
	for _, char := range numStr {
		pattern, ok := lcdDigits[char]
		if !ok {
			pattern = lcdDigits[' ']
		}
		for i := 0; i < 5; i++ {
			rows[i] += pattern[i] + " "
		}
	}

	// Add suffix on the last row
	if suffix != "" {
		suffixStyle := lipgloss.NewStyle().
			Foreground(color).
			Bold(true)
		rows[4] += suffixStyle.Render(suffix)
	}

	// Style and join
	style := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	var styledRows []string
	for _, row := range rows {
		styledRows = append(styledRows, style.Render(row))
	}

	return strings.Join(styledRows, "\n")
}

// PieChart renders a simple text-based pie chart showing progress
type PieChart struct {
	Width      int
	Height     int
	Worked     float64 // hours worked
	Target     float64 // target hours (e.g., 8)
	WorkColor  lipgloss.Color
	EmptyColor lipgloss.Color
}

// NewPieChart creates a new pie chart
func NewPieChart(worked, target float64) *PieChart {
	return &PieChart{
		Width:      20,
		Height:     10,
		Worked:     worked,
		Target:     target,
		WorkColor:  lipgloss.Color("#DDA036"),
		EmptyColor: lipgloss.Color("#9A9EA0"),
	}
}

// Render renders the pie chart as a string
func (p *PieChart) Render() string {
	percentage := p.Worked / p.Target
	if percentage > 1.0 {
		percentage = 1.0
	}
	if percentage < 0 {
		percentage = 0
	}

	// Use a simple horizontal bar chart that looks like a gauge
	return p.renderGauge(percentage)
}

// renderGauge renders a gauge-style progress indicator
func (p *PieChart) renderGauge(percentage float64) string {
	// Create a semi-circular gauge using Unicode characters
	gaugeWidth := 30
	filledWidth := int(float64(gaugeWidth) * percentage)

	// Build the gauge
	var gauge strings.Builder

	// Top arc
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(gaugeWidth + 4)

	// Progress bar
	filledStyle := lipgloss.NewStyle().
		Foreground(p.WorkColor).
		Bold(true)

	emptyStyle := lipgloss.NewStyle().
		Foreground(p.EmptyColor)

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", gaugeWidth-filledWidth)

	progressBar := filledStyle.Render(filled) + emptyStyle.Render(empty)

	// Percentage and hours text
	pctText := fmt.Sprintf("%.0f%%", percentage*100)
	hoursText := fmt.Sprintf("%.1fh / %.1fh", p.Worked, p.Target)

	pctStyle := lipgloss.NewStyle().
		Foreground(p.WorkColor).
		Bold(true).
		Align(lipgloss.Center).
		Width(gaugeWidth + 4)

	hoursStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center).
		Width(gaugeWidth + 4)

	gauge.WriteString(titleStyle.Render("Daily Progress"))
	gauge.WriteString("\n\n")
	gauge.WriteString("\n\n")
	gauge.WriteString("  " + progressBar + "  ")
	gauge.WriteString("\n\n")
	gauge.WriteString(pctStyle.Render(pctText))
	gauge.WriteString("\n")
	gauge.WriteString(hoursStyle.Render(hoursText))

	return gauge.String()
}

// DonutChart renders a donut/ring chart
type DonutChart struct {
	Worked     float64
	Target     float64
	WorkColor  lipgloss.Color
	EmptyColor lipgloss.Color
}

// NewDonutChart creates a new donut chart
func NewDonutChart(worked, target float64) *DonutChart {
	return &DonutChart{
		Worked:     worked,
		Target:     target,
		WorkColor:  lipgloss.Color("#DDA036"),
		EmptyColor: lipgloss.Color("#333333"),
	}
}

// Render renders the donut chart
func (d *DonutChart) Render() string {
	percentage := d.Worked / d.Target
	if percentage > 1.0 {
		percentage = 1.0
	}
	if percentage < 0 {
		percentage = 0
	}

	// Create a text-based donut using Unicode block characters
	// The donut is represented as a ring of characters
	radius := 5
	innerRadius := 2

	lines := make([]string, radius*2+1)

	filledStyle := lipgloss.NewStyle().Foreground(d.WorkColor)
	emptyStyle := lipgloss.NewStyle().Foreground(d.EmptyColor)

	// Calculate angle for filled portion (starting from top, going clockwise)
	filledAngle := percentage * 2 * math.Pi

	for y := -radius; y <= radius; y++ {
		var line strings.Builder
		for x := -radius; x <= radius; x++ {
			dist := math.Sqrt(float64(x*x + y*y))

			if dist >= float64(innerRadius) && dist <= float64(radius) {
				// Calculate angle from top (negative y-axis)
				angle := math.Atan2(float64(x), float64(-y))
				if angle < 0 {
					angle += 2 * math.Pi
				}

				if angle <= filledAngle {
					line.WriteString(filledStyle.Render("█"))
				} else {
					line.WriteString(emptyStyle.Render("░"))
				}
			} else if dist < float64(innerRadius) {
				line.WriteString(" ")
			} else {
				line.WriteString(" ")
			}
		}
		lines[y+radius] = line.String()
	}

	// Add percentage in center
	pctText := fmt.Sprintf("%.0f%%", percentage*100)
	centerLine := radius
	lineLen := len(lines[centerLine])
	centerPos := lineLen/2 - len(pctText)/2

	// Replace center with percentage
	if centerPos > 0 && centerPos+len(pctText) < lineLen {
		pctStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		lines[centerLine] = lines[centerLine][:centerPos] + pctStyle.Render(pctText) + lines[centerLine][centerPos+len(pctText):]
	}

	// Title and footer
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(radius*2 + 1)

	hoursStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center).
		Width(radius*2 + 1)

	result := titleStyle.Render("Shift Progress") + "\n\n"
	result += strings.Join(lines, "\n") + "\n\n"
	result += hoursStyle.Render(fmt.Sprintf("%.1fh / %.1fh", d.Worked, d.Target))

	return result
}

// TimerDashboard combines the LCD timer and progress chart.
//
// Three visual states:
//   - IsActive=true: timer is running (green/orange, blinking, "Stop" button)
//   - IsPaused=true (IsActive=false): a paused entry waiting to be resumed
//     (amber, no blink, "PAUSED" status, "Resume <Project>" button)
//   - both false: idle ("No Active Timer", "Start Timer" button)
type TimerDashboard struct {
	Hours                 int
	Minutes               int
	TodayWorked           float64
	ShiftTarget           float64
	IsActive              bool
	IsPaused              bool
	ProjectName           string
	TaskName              string
	BlinkOn               bool // For blinking colon and status dot
	ButtonSelected        bool // Whether the Start/Stop button is selected
	HistoryButtonSelected bool // Whether the History button is selected
}

// NewTimerDashboard creates a new timer dashboard
func NewTimerDashboard() *TimerDashboard {
	return &TimerDashboard{
		ShiftTarget: 8.0,
	}
}

// Render renders the complete dashboard
func (td *TimerDashboard) Render() string {
	// Consistent dimensions for both boxes
	boxWidth := 40
	boxHeight := 14              // Increased to accommodate button
	contentWidth := boxWidth - 6 // Account for border and padding

	// Container style with fixed dimensions
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(boxWidth).
		Height(boxHeight)

	// Determine color and status text based on three-way state.
	timerColor := lipgloss.Color("#9A9EA0")
	statusText := "No Active Timer"
	switch {
	case td.IsActive:
		timerColor = lipgloss.Color("#DDA036")
		statusText = "Timer Running"
	case td.IsPaused:
		timerColor = lipgloss.Color("#DDA036")
		statusText = "PAUSED"
	}

	// Status indicator
	statusStyle := lipgloss.NewStyle().
		Foreground(timerColor).
		Bold(true).
		Align(lipgloss.Center).
		Width(contentWidth)

	var statusIndicator string
	switch {
	case td.IsActive:
		// Blink the dot when timer is running
		if td.BlinkOn {
			statusIndicator = statusStyle.Render("● " + statusText)
		} else {
			statusIndicator = statusStyle.Render("○ " + statusText)
		}
	case td.IsPaused:
		// Steady amber dot — no blink because nothing is actively ticking.
		statusIndicator = statusStyle.Render("|| " + statusText)
	default:
		statusIndicator = statusStyle.Render("○ " + statusText)
	}

	// LCD Time display (blink colon only when actively running, not paused)
	blinkColon := !td.IsActive || td.BlinkOn
	lcdTime := RenderLCDTime(td.Hours, td.Minutes, timerColor, blinkColon)

	// Center the LCD time
	lcdStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(contentWidth)

	centeredLCD := lcdStyle.Render(lcdTime)

	// Project/Task info if active
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center).
		Width(contentWidth)

	var projectInfo string
	if (td.IsActive || td.IsPaused) && (td.ProjectName != "" || td.TaskName != "") {
		info := td.ProjectName
		if td.TaskName != "" {
			info += " / " + td.TaskName
		}
		// Truncate if too long
		if len(info) > contentWidth-4 {
			info = info[:contentWidth-7] + "..."
		}
		projectInfo = infoStyle.Render(info)
	} else {
		// Empty placeholder to maintain consistent height
		projectInfo = infoStyle.Render(" ")
	}

	// Start/Stop/Resume button. ASCII text only — the glyphs ▶ ■ render as
	// "?" on systems whose terminal font lacks them; colour conveys the role.
	buttonText := "Start Timer"
	buttonColor := lipgloss.Color("#2ECC71") // Green for start
	switch {
	case td.IsActive:
		buttonText = "Stop Timer"
		buttonColor = lipgloss.Color("#E74C3C") // Red for stop
	case td.IsPaused:
		buttonText = "Resume"
		if td.ProjectName != "" {
			buttonText = "Resume " + td.ProjectName
		}
		buttonColor = lipgloss.Color("#DDA036") // Amber for resume
	}

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(buttonColor).
		Padding(0, 2).
		Bold(true)

	if td.ButtonSelected {
		buttonStyle = buttonStyle.
			Background(lipgloss.Color("#DDA036")).
			Underline(true)
	}

	buttonCentered := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(contentWidth).
		Render(buttonStyle.Render(buttonText))

	// Combine timer section with button
	timerSection := lipgloss.JoinVertical(
		lipgloss.Center,
		statusIndicator,
		"",
		centeredLCD,
		"",
		projectInfo,
		"",
		buttonCentered,
	)

	// Progress gauge section
	gaugeSection := td.renderGaugeSection(contentWidth)

	// Put timer and gauge side by side with same container style
	timerBox := containerStyle.Render(timerSection)
	gaugeBox := containerStyle.Render(gaugeSection)

	dashboard := lipgloss.JoinHorizontal(
		lipgloss.Top,
		timerBox,
		"  ",
		gaugeBox,
	)

	return dashboard
}

// renderGaugeSection renders the progress gauge section
func (td *TimerDashboard) renderGaugeSection(contentWidth int) string {
	percentage := td.TodayWorked / td.ShiftTarget
	if percentage > 1.0 {
		percentage = 1.0
	}
	if percentage < 0 {
		percentage = 0
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(contentWidth)

	// Progress bar
	gaugeWidth := contentWidth - 4
	filledWidth := int(float64(gaugeWidth) * percentage)

	filledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true)

	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", gaugeWidth-filledWidth)
	progressBar := filledStyle.Render(filled) + emptyStyle.Render(empty)

	barStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(contentWidth)

	// Percentage
	pctStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Align(lipgloss.Center).
		Width(contentWidth)

	// Hours info
	hoursStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center).
		Width(contentWidth)

	// History button
	historyButtonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#569FC6")).
		Padding(0, 2).
		Bold(true)

	if td.HistoryButtonSelected {
		historyButtonStyle = historyButtonStyle.
			Background(lipgloss.Color("#DDA036")).
			Underline(true)
	}

	historyButtonCentered := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(contentWidth).
		Render(historyButtonStyle.Render("📋 History"))

	// Structure to match timer section for alignment:
	// Timer: status, "", LCD (5 lines), "", projectInfo, "", button
	// Gauge: title, "", bar, "", pct+hours (combined), "", button
	return lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render("Daily Progress"),
		"",
		barStyle.Render(progressBar),
		"",
		pctStyle.Render(fmt.Sprintf("%.0f%%", percentage*100)),
		hoursStyle.Render(fmt.Sprintf("%.1fh / %.1fh", td.TodayWorked, td.ShiftTarget)),
		"",
		"", // Extra empty lines to align with timer box
		"",
		"",
		historyButtonCentered,
	)
}
