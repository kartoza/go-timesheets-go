package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/ai"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

// AIAssistantModel provides a TUI for the AI assistant
type AIAssistantModel struct {
	apiClient   *api.Client
	assistant   *ai.TimesheetAssistant
	width       int
	height      int
	headerState *HeaderState

	// Input
	queryInput textinput.Model

	// State
	loading       bool
	statusText    string
	contextLoaded bool
	err           error

	// Results
	result         *ai.QueryResult
	selectedMatch  int // Index of selected match in results
	resultScroll   int // Scroll offset for results
	maxVisibleRows int // How many result rows fit on screen

	// Example queries
	examples     []string
	showExamples bool
}

// NewAIAssistantModel creates a new AI assistant TUI model
func NewAIAssistantModel(apiClient *api.Client) *AIAssistantModel {
	ti := textinput.New()
	ti.Placeholder = "Ask about projects, tasks, or timesheets..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60

	return &AIAssistantModel{
		apiClient:  apiClient,
		assistant:  ai.NewTimesheetAssistant(),
		queryInput: ti,
		statusText: "Loading AI context...",
		examples: []string{
			"Which project for GeoServer Docker?",
			"Which projects did I work on last week?",
			"How consistent was my timekeeping?",
			"Which project did I spend the most time on?",
			"What's my utilization rate?",
			"Am I on track this week?",
			"Do I have any gaps in my timesheet?",
			"How much context switching am I doing?",
			"Compare my hours across projects this month",
			"What's my overtime this week?",
		},
		showExamples:   true,
		maxVisibleRows: 10,
	}
}

// SetHeaderState sets the shared header state
func (m *AIAssistantModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// Init initializes the AI assistant view
func (m *AIAssistantModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.loadContext(),
	)
}

// Update handles messages for the AI assistant
func (m *AIAssistantModel) Update(msg tea.Msg) (*AIAssistantModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.queryInput.Width = min(60, m.width-20)
		m.updateMaxVisibleRows()
		return m, nil

	case tea.KeyMsg:
		// Handle result selection when we have matches
		if m.result != nil && m.result.Type == ai.ResultMatches && len(m.result.Matches) > 0 && !m.queryInput.Focused() {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
				if m.selectedMatch > 0 {
					m.selectedMatch--
					m.ensureMatchVisible()
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
				if m.selectedMatch < len(m.result.Matches)-1 {
					m.selectedMatch++
					m.ensureMatchVisible()
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				match := m.result.Matches[m.selectedMatch]
				return m, m.startTimerFromMatch(match)
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
				m.queryInput.Focus()
				return m, textinput.Blink
			}
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.result != nil && !m.queryInput.Focused() {
				// If results showing and input not focused, focus input
				m.queryInput.Focus()
				return m, textinput.Blink
			}
			return m, func() tea.Msg { return backToMenuMsg{} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.queryInput.Focused() && m.queryInput.Value() != "" {
				return m, m.executeQuery(m.queryInput.Value())
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if m.queryInput.Focused() && m.result != nil && m.result.Type == ai.ResultMatches && len(m.result.Matches) > 0 {
				m.queryInput.Blur()
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("1", "2", "3", "4"))):
			if m.showExamples && !m.queryInput.Focused() {
				idx := int(msg.String()[0] - '1')
				if idx >= 0 && idx < len(m.examples) {
					m.queryInput.SetValue(m.examples[idx])
					return m, m.executeQuery(m.examples[idx])
				}
			}
		}

		// Pass to text input
		if m.queryInput.Focused() {
			var cmd tea.Cmd
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}

	case aiContextLoadedMsg:
		if msg.err != nil {
			m.statusText = "Failed to load context: " + msg.err.Error()
			m.err = msg.err
		} else {
			m.assistant.SetContext(msg.ctx)
			m.contextLoaded = true
			m.updateStatus()

			// Train NN in background if we have entries
			if len(msg.ctx.Entries) > 0 {
				m.assistant.TrainFromHistoryAsync(msg.ctx.Entries, nil)
			}
		}
		return m, nil

	case aiQueryResultMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusText = "Error: " + msg.err.Error()
		} else {
			m.err = nil
			m.result = msg.result
			m.selectedMatch = 0
			m.resultScroll = 0
			m.showExamples = false
			m.updateStatus()

			// If matches found, blur input so arrow keys navigate results
			if m.result.Type == ai.ResultMatches && len(m.result.Matches) > 0 {
				m.queryInput.Blur()
			}
		}
		return m, nil

	case aiTimerStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusText = "Failed to start timer: " + msg.err.Error()
			return m, nil
		}
		// Return to main menu after starting timer
		return m, func() tea.Msg { return backToMenuMsg{} }
	}

	return m, nil
}

// View renders the AI assistant screen
func (m *AIAssistantModel) View() string {
	// Header
	header := RenderHeader("AI Assistant", m.headerState)

	// Subtitle
	subtitleStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width)
	subtitle := subtitleStyle.Render("Ask questions about your timesheets")

	// Query input box
	queryBox := m.renderQueryBox()

	// Status line
	statusStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Align(lipgloss.Center).
		Width(m.width)
	status := statusStyle.Render(m.statusText)

	// Results or examples
	var content string
	if m.loading {
		content = m.renderLoading()
	} else if m.result != nil {
		content = m.renderResults()
	} else if m.showExamples {
		content = m.renderExamples()
	}

	// Help
	help := m.renderHelp()

	// Combine
	parts := []string{header, "", subtitle, "", queryBox, status}
	if content != "" {
		parts = append(parts, "", content)
	}

	mainSection := lipgloss.JoinVertical(lipgloss.Center, parts...)

	// Center content at top
	centeredMain := lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	// Help at bottom
	helpStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpStyle.Render(help),
	)
}

// renderQueryBox renders the query input area
func (m *AIAssistantModel) renderQueryBox() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorOrange).
		Padding(0, 1).
		Width(min(70, m.width-4))

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true)

	label := labelStyle.Render("Ask:")
	input := m.queryInput.View()

	return lipgloss.Place(
		m.width,
		0,
		lipgloss.Center,
		lipgloss.Top,
		boxStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, label+" ", input)),
	)
}

// renderLoading renders the loading state
func (m *AIAssistantModel) renderLoading() string {
	style := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width)
	return style.Render("Thinking...")
}

// renderResults renders the query results
func (m *AIAssistantModel) renderResults() string {
	if m.result == nil {
		return ""
	}

	switch m.result.Type {
	case ai.ResultMatches:
		return m.renderMatches()
	case ai.ResultAnalysis:
		return m.renderAnalysis()
	case ai.ResultText:
		return m.renderText()
	case ai.ResultError:
		return m.renderError(m.result.Text)
	}
	return ""
}

// renderMatches renders match results with selection
func (m *AIAssistantModel) renderMatches() string {
	if m.result == nil || len(m.result.Matches) == 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true).
		Align(lipgloss.Center).
		Width(min(70, m.width-4))

	containerWidth := min(70, m.width-4)

	selectedStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorOrange).
		Foreground(ColorWhite).
		Width(containerWidth - 4).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDarkGray).
		Foreground(ColorGray).
		Width(containerWidth - 4).
		Padding(0, 1)

	header := headerStyle.Render(fmt.Sprintf("Found %d match(es) - use arrows to select, Enter to start timer:", len(m.result.Matches)))

	var rows []string
	rows = append(rows, header)

	// Apply scroll offset
	visibleEnd := m.resultScroll + m.maxVisibleRows
	if visibleEnd > len(m.result.Matches) {
		visibleEnd = len(m.result.Matches)
	}

	if m.resultScroll > 0 {
		scrollIndicator := lipgloss.NewStyle().Foreground(ColorGray).Render("  ... more above ...")
		rows = append(rows, scrollIndicator)
	}

	for i := m.resultScroll; i < visibleEnd; i++ {
		match := m.result.Matches[i]

		title := match.ProjectName
		if match.TaskName != "" {
			title += " / " + match.TaskName
		}
		if match.ActivityName != "" {
			title += " / " + match.ActivityName
		}

		confidence := fmt.Sprintf("%.0f%% confidence", match.Confidence*100)
		detail := confidence
		if match.Reason != "" {
			detail += " - " + match.Reason
		}

		nameStyle := lipgloss.NewStyle().Bold(true)
		detailStyle := lipgloss.NewStyle().Italic(true).Foreground(ColorGray)

		content := lipgloss.JoinVertical(lipgloss.Left,
			nameStyle.Render(title),
			detailStyle.Render(detail),
		)

		if i == m.selectedMatch {
			indicator := lipgloss.NewStyle().
				Foreground(ColorOrange).
				Bold(true).
				Render("▶ ")
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, indicator, selectedStyle.Render(content)))
		} else {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, "  ", normalStyle.Render(content)))
		}
	}

	if visibleEnd < len(m.result.Matches) {
		scrollIndicator := lipgloss.NewStyle().Foreground(ColorGray).Render("  ... more below ...")
		rows = append(rows, scrollIndicator)
	}

	return lipgloss.JoinVertical(lipgloss.Center, rows...)
}

// renderAnalysis renders analysis results
func (m *AIAssistantModel) renderAnalysis() string {
	if m.result == nil || m.result.Analysis == nil {
		return ""
	}

	analysis := m.result.Analysis
	containerWidth := min(70, m.width-4)

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true).
		Align(lipgloss.Center).
		Width(containerWidth)

	summaryStyle := lipgloss.NewStyle().
		Foreground(ColorWhite).
		Align(lipgloss.Center).
		Width(containerWidth)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorOrange).
		Width(containerWidth).
		Padding(1, 2)

	var parts []string
	parts = append(parts, titleStyle.Render(analysis.Title))

	if analysis.Summary != "" {
		parts = append(parts, summaryStyle.Render(analysis.Summary))
		parts = append(parts, "")
	}

	// Detail rows
	labelStyle := lipgloss.NewStyle().Foreground(ColorBlue)
	valueStyle := lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)

	var detailRows []string
	for _, d := range analysis.Details {
		// Pad label to align values
		label := labelStyle.Render(d.Label)
		value := valueStyle.Render(d.Value)
		row := lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value)
		detailRows = append(detailRows, row)
	}

	if len(detailRows) > 0 {
		detailContent := lipgloss.JoinVertical(lipgloss.Left, detailRows...)
		parts = append(parts, boxStyle.Render(detailContent))
	}

	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// renderText renders a plain text result
func (m *AIAssistantModel) renderText() string {
	if m.result == nil {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(ColorWhite).
		Width(min(70, m.width-4)).
		Align(lipgloss.Left).
		Padding(1, 2)

	return lipgloss.Place(
		m.width,
		0,
		lipgloss.Center,
		lipgloss.Top,
		style.Render(m.result.Text),
	)
}

// renderError renders an error message
func (m *AIAssistantModel) renderError(msg string) string {
	style := lipgloss.NewStyle().
		Foreground(ColorRed).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width)
	return style.Render("Error: " + msg)
}

// renderExamples renders the example query buttons
func (m *AIAssistantModel) renderExamples() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width)

	buttonStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Padding(0, 2)

	numStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true)

	var buttons []string
	for i, ex := range m.examples {
		num := numStyle.Render(fmt.Sprintf("[%d]", i+1))
		btn := buttonStyle.Render(num + " " + ex)
		buttons = append(buttons, btn)
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Example queries (press number to try):"),
		"",
		lipgloss.JoinVertical(lipgloss.Left, buttons...),
	)
}

// renderHelp renders the help text
func (m *AIAssistantModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Italic(true)

	if m.result != nil && m.result.Type == ai.ResultMatches && len(m.result.Matches) > 0 && !m.queryInput.Focused() {
		return helpStyle.Render("Up/Down: Select match • Enter: Start timer • Tab: Edit query • Esc: Back")
	}
	return helpStyle.Render("Enter: Ask • 1-4: Example queries • Esc: Back")
}

// loadContext loads the AI context asynchronously
func (m *AIAssistantModel) loadContext() tea.Cmd {
	return func() tea.Msg {
		ctx, err := m.fetchContext()
		return aiContextLoadedMsg{ctx: ctx, err: err}
	}
}

// fetchContext loads project/task/activity/entry data from the API
func (m *AIAssistantModel) fetchContext() (*ai.TimesheetContext, error) {
	ctx := &ai.TimesheetContext{}

	// Load projects
	projects, err := m.apiClient.GetProjects("")
	if err != nil {
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	for _, p := range projects {
		proj := ai.ProjectInfo{
			ID:   p.ID,
			Name: p.Label,
		}

		// Load tasks for this project
		tasks, err := m.apiClient.GetTasks(fmt.Sprintf("%d", p.ID))
		if err == nil {
			for _, t := range tasks {
				proj.Tasks = append(proj.Tasks, ai.TaskInfo{
					ID:   t.ID,
					Name: t.Label,
				})
			}
		}

		ctx.Projects = append(ctx.Projects, proj)
	}

	// Load activities
	activities, err := m.apiClient.GetActivities()
	if err == nil {
		for _, a := range activities {
			ctx.Activities = append(ctx.Activities, ai.ActivityInfo{
				ID:   a.ID,
				Name: a.Label,
			})
		}
	}

	// Load recent entries
	entries, err := m.apiClient.GetTimelogs()
	if err == nil {
		for _, e := range entries {
			startTime, _ := e.GetFromTimeAsTime()
			endTime, _ := e.GetToTimeAsTime()
			ctx.Entries = append(ctx.Entries, ai.EntryInfo{
				ProjectName:  e.ProjectName,
				TaskName:     e.TaskName,
				ActivityName: e.ActivityType,
				StartTime:    startTime,
				EndTime:      endTime,
				Hours:        e.Hours,
				Description:  derefStrPtr(e.Description),
				Submitted:    e.Submitted,
			})
		}
	}

	return ctx, nil
}

// executeQuery runs the AI query asynchronously
func (m *AIAssistantModel) executeQuery(query string) tea.Cmd {
	m.loading = true
	m.result = nil
	m.err = nil
	m.statusText = "Thinking..."

	return func() tea.Msg {
		result, err := m.assistant.Query(query)
		return aiQueryResultMsg{result: result, err: err}
	}
}

// startTimerFromMatch starts a timer using the selected match
func (m *AIAssistantModel) startTimerFromMatch(match ai.TimesheetMatch) tea.Cmd {
	return func() tea.Msg {
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", match.ProjectID),
			ActivityID:  fmt.Sprintf("%d", match.ActivityID),
			Description: fmt.Sprintf("AI suggested: %s", match.ProjectName),
			StartTime:   time.Now().UTC(),
		}

		if match.TaskID > 0 {
			taskID := fmt.Sprintf("%d", match.TaskID)
			entry.TaskID = &taskID
		}

		err := m.apiClient.CreateTimesheet(entry)
		return aiTimerStartedMsg{
			err:         err,
			projectName: match.ProjectName,
		}
	}
}

// updateStatus updates the status text from AI backend state
func (m *AIAssistantModel) updateStatus() {
	status := m.assistant.GetStatus()

	var parts []string
	if v, ok := status["embedded_loaded"].(bool); ok && v {
		parts = append(parts, "LLM: embedded")
	}
	if v, ok := status["ollama_available"].(bool); ok && v {
		model := "llama3.2"
		if ms, ok := status["ollama_model"].(string); ok && ms != "" {
			model = ms
		}
		parts = append(parts, fmt.Sprintf("Ollama: %s", model))
	}
	if v, ok := status["nn_trained"].(bool); ok && v {
		parts = append(parts, "NN: trained")
	}
	parts = append(parts, "Fuzzy: ready")

	m.statusText = "AI backends: " + strings.Join(parts, " | ")
}

// ensureMatchVisible adjusts scroll to keep selected match visible
func (m *AIAssistantModel) ensureMatchVisible() {
	if m.selectedMatch < m.resultScroll {
		m.resultScroll = m.selectedMatch
	}
	if m.selectedMatch >= m.resultScroll+m.maxVisibleRows {
		m.resultScroll = m.selectedMatch - m.maxVisibleRows + 1
	}
}

// updateMaxVisibleRows updates how many result rows fit on screen
func (m *AIAssistantModel) updateMaxVisibleRows() {
	// Header ~3 + subtitle ~2 + query box ~4 + status ~1 + results header ~2 = ~12 lines overhead
	// Each match card is ~4 lines
	available := m.height - 16
	if available < 3 {
		available = 3
	}
	m.maxVisibleRows = available / 4
	if m.maxVisibleRows < 2 {
		m.maxVisibleRows = 2
	}
}

// derefStrPtr safely dereferences a string pointer
func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Message types for the AI assistant

type aiContextLoadedMsg struct {
	ctx *ai.TimesheetContext
	err error
}

type aiQueryResultMsg struct {
	result *ai.QueryResult
	err    error
}

type aiTimerStartedMsg struct {
	err         error
	projectName string
}

// launchAIAssistantMsg triggers navigation to the AI assistant
type launchAIAssistantMsg struct{}
