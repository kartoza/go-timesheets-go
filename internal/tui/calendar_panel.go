package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// CalendarPanel displays calendar events for a specific date
type CalendarPanel struct {
	calendarService *service.CalendarService
	date            time.Time
	events          []models.CalendarEvent
	selectedIndex   int
	isLoading       bool
	err             error
	width           int
	height          int

	// Callback when event is selected
	OnEventSelected func(event models.CalendarEvent)
}

// NewCalendarPanel creates a new calendar panel
func NewCalendarPanel(calendarService *service.CalendarService) *CalendarPanel {
	return &CalendarPanel{
		calendarService: calendarService,
		selectedIndex:   0,
	}
}

// SetSize sets the panel dimensions
func (p *CalendarPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SetDate sets the date and triggers event loading
func (p *CalendarPanel) SetDate(date time.Time) tea.Cmd {
	p.date = date
	p.isLoading = true
	p.events = nil
	p.err = nil
	p.selectedIndex = 0
	return p.loadEvents()
}

// Update handles messages
func (p *CalendarPanel) Update(msg tea.Msg) (*CalendarPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.selectedIndex > 0 {
				p.selectedIndex--
			}
			return p, nil

		case "down", "j":
			if p.selectedIndex < len(p.events)-1 {
				p.selectedIndex++
			}
			return p, nil

		case "enter", " ":
			if len(p.events) > 0 && p.selectedIndex < len(p.events) {
				event := p.events[p.selectedIndex]
				if p.OnEventSelected != nil {
					p.OnEventSelected(event)
				}
				return p, func() tea.Msg {
					return calendarEventSelectedMsg{event: event}
				}
			}
			return p, nil
		}

	case calendarEventsLoadedMsg:
		p.isLoading = false
		if msg.err != nil {
			p.err = msg.err
		} else {
			p.events = msg.events
			p.err = nil
		}
		return p, nil
	}

	return p, nil
}

// View renders the calendar panel
func (p *CalendarPanel) View() string {
	width := p.width
	if width == 0 {
		width = 35
	}

	// Panel styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Align(lipgloss.Center).
		Width(width - 4)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(0, 1).
		Width(width)

	grayStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))

	// Build content
	var lines []string

	// Title
	dateStr := "Calendar Events"
	if !p.date.IsZero() {
		dateStr = fmt.Sprintf("Calendar - %s", p.date.Format("Mon Jan 2"))
	}
	lines = append(lines, titleStyle.Render(dateStr))
	lines = append(lines, "")

	// Content based on state
	if p.calendarService == nil || !p.calendarService.IsConnected() {
		lines = append(lines, grayStyle.Render("Google Calendar not connected"))
		lines = append(lines, "")
		lines = append(lines, grayStyle.Italic(true).Render("Go to Settings → Google Calendar"))
		lines = append(lines, grayStyle.Italic(true).Render("to connect your calendar"))
	} else if p.isLoading {
		lines = append(lines, grayStyle.Render("Loading events..."))
	} else if p.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C"))
		lines = append(lines, errorStyle.Render("Error loading events"))
		errMsg := p.err.Error()
		if len(errMsg) > width-6 {
			errMsg = errMsg[:width-9] + "..."
		}
		lines = append(lines, grayStyle.Render(errMsg))
	} else if len(p.events) == 0 {
		lines = append(lines, grayStyle.Render("No events for this day"))
	} else {
		// Render events
		for i, event := range p.events {
			eventLine := p.renderEventLine(event, i == p.selectedIndex, width-4)
			lines = append(lines, eventLine)
		}
	}

	// Hint
	lines = append(lines, "")
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center).
		Width(width - 4)
	if len(p.events) > 0 {
		lines = append(lines, hintStyle.Render("↑/↓: Select • Enter: Use times"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return borderStyle.Render(content)
}

// renderEventLine renders a single event line
func (p *CalendarPanel) renderEventLine(event models.CalendarEvent, selected bool, maxWidth int) string {
	// Time range
	timeStr := event.FormatTimeRange()

	// Title (truncate if needed)
	title := event.Title
	titleMaxLen := maxWidth - len(timeStr) - 5
	if titleMaxLen < 10 {
		titleMaxLen = 10
	}
	if len(title) > titleMaxLen {
		title = title[:titleMaxLen-3] + "..."
	}

	// Duration
	durationStr := fmt.Sprintf("%.1fh", event.DurationHours())

	// Styles
	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))

	durationStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6"))

	selectedBg := lipgloss.NewStyle().
		Background(lipgloss.Color("#3D3E40"))

	// Build the line
	prefix := "  "
	if selected {
		prefix = "▶ "
		timeStyle = timeStyle.Foreground(lipgloss.Color("#DDA036"))
		titleStyle = titleStyle.Foreground(lipgloss.Color("#FFFFFF"))
	}

	line := fmt.Sprintf("%s%s %s (%s)",
		prefix,
		timeStyle.Render(timeStr),
		titleStyle.Render(title),
		durationStyle.Render(durationStr),
	)

	if selected {
		line = selectedBg.Render(line)
	}

	return line
}

// loadEvents loads calendar events for the current date
func (p *CalendarPanel) loadEvents() tea.Cmd {
	return func() tea.Msg {
		if p.calendarService == nil {
			return calendarEventsLoadedMsg{err: fmt.Errorf("calendar not configured")}
		}

		if !p.calendarService.IsConnected() {
			return calendarEventsLoadedMsg{err: fmt.Errorf("not connected to Google Calendar")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		events, err := p.calendarService.GetEventsForDay(ctx, p.date)
		if err != nil {
			return calendarEventsLoadedMsg{err: err}
		}

		return calendarEventsLoadedMsg{events: events}
	}
}

// IsConnected returns whether the calendar service is connected
func (p *CalendarPanel) IsConnected() bool {
	return p.calendarService != nil && p.calendarService.IsConnected()
}

// GetSelectedEvent returns the currently selected event
func (p *CalendarPanel) GetSelectedEvent() *models.CalendarEvent {
	if len(p.events) == 0 || p.selectedIndex >= len(p.events) {
		return nil
	}
	return &p.events[p.selectedIndex]
}

// SetCalendarService updates the calendar service
func (p *CalendarPanel) SetCalendarService(svc *service.CalendarService) {
	p.calendarService = svc
}

// Message types
type calendarEventsLoadedMsg struct {
	events []models.CalendarEvent
	err    error
}

type calendarEventSelectedMsg struct {
	event models.CalendarEvent
}

// CalendarSettingsModel represents the calendar settings screen for TUI
type CalendarSettingsModel struct {
	calendarService *service.CalendarService
	width           int
	height          int
	headerState     *HeaderState

	// Input fields
	focusedField    int // 0=clientID, 1=clientSecret, 2=connect, 3=disconnect, 4=back
	clientIDInput   string
	clientIDCursor  int
	secretInput     string
	secretCursor    int
	showSecret      bool

	// State
	isConnecting bool
	statusMsg    string
	err          error
}

// NewCalendarSettingsModel creates a new calendar settings model
func NewCalendarSettingsModel() *CalendarSettingsModel {
	m := &CalendarSettingsModel{
		focusedField: 0,
	}

	// Load existing config
	m.calendarService, _ = service.NewCalendarService()

	return m
}

// Init initializes the model
func (m *CalendarSettingsModel) Init() tea.Cmd {
	return nil
}

// SetHeaderState sets the shared header state
func (m *CalendarSettingsModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// Update handles messages
func (m *CalendarSettingsModel) Update(msg tea.Msg) (*CalendarSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc", "q":
			if m.focusedField == 4 { // Back button
				return m, func() tea.Msg { return backToSettingsMsg{} }
			}
			return m, func() tea.Msg { return backToSettingsMsg{} }

		case "tab", "down":
			m.focusedField = (m.focusedField + 1) % 5
			return m, nil

		case "shift+tab", "up":
			m.focusedField--
			if m.focusedField < 0 {
				m.focusedField = 4
			}
			return m, nil

		case "enter":
			return m.handleEnter()

		case "backspace":
			switch m.focusedField {
			case 0: // Client ID
				if len(m.clientIDInput) > 0 {
					m.clientIDInput = m.clientIDInput[:len(m.clientIDInput)-1]
				}
			case 1: // Secret
				if len(m.secretInput) > 0 {
					m.secretInput = m.secretInput[:len(m.secretInput)-1]
				}
			}
			return m, nil

		default:
			// Handle text input
			if len(msg.String()) == 1 {
				switch m.focusedField {
				case 0: // Client ID
					m.clientIDInput += msg.String()
				case 1: // Secret
					m.secretInput += msg.String()
				}
			}
			return m, nil
		}

	case calendarAuthCompleteMsg:
		m.isConnecting = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Authentication failed"
		} else {
			m.statusMsg = "Connected to Google Calendar"
			m.err = nil
			// Reload service
			m.calendarService, _ = service.NewCalendarService()
		}
		return m, nil
	}

	return m, nil
}

// handleEnter handles the enter key
func (m *CalendarSettingsModel) handleEnter() (*CalendarSettingsModel, tea.Cmd) {
	switch m.focusedField {
	case 2: // Connect button
		if m.clientIDInput == "" || m.secretInput == "" {
			m.err = fmt.Errorf("please enter both Client ID and Secret")
			return m, nil
		}
		m.isConnecting = true
		m.statusMsg = "Starting authentication..."
		return m, m.startAuth()

	case 3: // Disconnect button
		if m.calendarService != nil {
			_ = m.calendarService.Disconnect()
			m.calendarService = nil
		}
		m.statusMsg = "Disconnected"
		return m, nil

	case 4: // Back button
		return m, func() tea.Msg { return backToSettingsMsg{} }
	}

	// For input fields, move to next field
	m.focusedField = (m.focusedField + 1) % 5
	return m, nil
}

// startAuth starts the OAuth flow
func (m *CalendarSettingsModel) startAuth() tea.Cmd {
	return func() tea.Msg {
		if m.calendarService == nil {
			m.calendarService = service.NewCalendarServiceWithCredentials(m.clientIDInput, m.secretInput)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		authURL, codeChan, errChan, err := m.calendarService.StartAuthFlow(ctx, m.clientIDInput, m.secretInput)
		if err != nil {
			return calendarAuthCompleteMsg{err: err}
		}

		// Open browser
		_ = service.OpenBrowser(authURL)

		// Wait for code or error
		select {
		case code := <-codeChan:
			err = m.calendarService.CompleteAuthFlow(context.Background(), code, m.clientIDInput, m.secretInput)
			return calendarAuthCompleteMsg{err: err}
		case err := <-errChan:
			return calendarAuthCompleteMsg{err: err}
		case <-ctx.Done():
			return calendarAuthCompleteMsg{err: ctx.Err()}
		}
	}
}

// View renders the calendar settings screen
func (m *CalendarSettingsModel) View() string {
	// Render header
	header := m.renderHeader()

	// Build content
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Align(lipgloss.Center).
		Width(54)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(15)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Width(15)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#3A3A3A")).
		Padding(0, 1).
		Width(35)

	focusedInputStyle := inputStyle.
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#DDA036"))

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#569FC6")).
		Padding(0, 2)

	focusedButtonStyle := buttonStyle.
		Background(lipgloss.Color("#DDA036")).
		Bold(true)

	dangerButtonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#E74C3C")).
		Padding(0, 2)

	focusedDangerButtonStyle := dangerButtonStyle.
		Background(lipgloss.Color("#C0392B")).
		Bold(true)

	var lines []string

	// Title
	lines = append(lines, titleStyle.Render("Google Calendar Settings"))
	lines = append(lines, "")

	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true)
	lines = append(lines, instructionStyle.Render("1. Go to Google Cloud Console"))
	lines = append(lines, instructionStyle.Render("2. Create OAuth 2.0 credentials"))
	lines = append(lines, instructionStyle.Render("3. Enter Client ID and Secret below"))
	lines = append(lines, "")

	// Client ID
	clientLabel := labelStyle.Render("Client ID:")
	if m.focusedField == 0 {
		clientLabel = focusedLabelStyle.Render("Client ID:")
	}
	clientValue := m.clientIDInput
	if clientValue == "" {
		clientValue = "Enter client ID..."
	}
	if len(clientValue) > 30 {
		clientValue = clientValue[:27] + "..."
	}
	clientInput := inputStyle.Render(clientValue)
	if m.focusedField == 0 {
		clientInput = focusedInputStyle.Render(clientValue)
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, clientLabel, clientInput))

	// Client Secret
	secretLabel := labelStyle.Render("Client Secret:")
	if m.focusedField == 1 {
		secretLabel = focusedLabelStyle.Render("Client Secret:")
	}
	secretValue := strings.Repeat("*", len(m.secretInput))
	if secretValue == "" {
		secretValue = "Enter secret..."
	}
	if len(secretValue) > 30 {
		secretValue = secretValue[:27] + "..."
	}
	secretInput := inputStyle.Render(secretValue)
	if m.focusedField == 1 {
		secretInput = focusedInputStyle.Render(secretValue)
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, secretLabel, secretInput))
	lines = append(lines, "")

	// Status
	if m.calendarService != nil && m.calendarService.IsConnected() {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#27AE60")).
			Bold(true)
		lines = append(lines, statusStyle.Render("✓ Connected to Google Calendar"))
	} else if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F39C12"))
		lines = append(lines, statusStyle.Render(m.statusMsg))
	}

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C"))
		lines = append(lines, errorStyle.Render("Error: "+m.err.Error()))
	}
	lines = append(lines, "")

	// Buttons
	connectBtn := buttonStyle.Render("Connect")
	if m.focusedField == 2 {
		connectBtn = focusedButtonStyle.Render("▶ Connect")
	}

	disconnectBtn := dangerButtonStyle.Render("Disconnect")
	if m.focusedField == 3 {
		disconnectBtn = focusedDangerButtonStyle.Render("▶ Disconnect")
	}

	backBtn := buttonStyle.Render("← Back")
	if m.focusedField == 4 {
		backBtn = focusedButtonStyle.Render("▶ Back")
	}

	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, connectBtn, "  ", disconnectBtn, "  ", backBtn)
	buttonRowCentered := lipgloss.NewStyle().
		Width(54).
		Align(lipgloss.Center).
		Render(buttonRow)
	lines = append(lines, buttonRowCentered)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	mainContent := containerStyle.Render(content)

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)
	help := helpStyle.Render("Tab: Next field • Enter: Select • Esc: Back")

	// Combine
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		mainContent,
	)

	centered := lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	helpCentered := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(help)

	return lipgloss.JoinVertical(lipgloss.Left, centered, helpCentered)
}

// renderHeader renders the header
func (m *CalendarSettingsModel) renderHeader() string {
	if m.headerState != nil {
		return RenderHeader("Calendar Settings", m.headerState)
	}
	return RenderHeader("Calendar Settings", &HeaderState{})
}

// Message types
type calendarAuthCompleteMsg struct {
	err error
}

type backToSettingsMsg struct{}
