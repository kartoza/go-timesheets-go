package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
)

// HistoryView displays timesheet history
type HistoryView struct {
	apiClient *api.Client
	username  string
	width     int
	height    int

	// Data
	allHistory []api.TimelogEntry

	// Pagination
	page          int
	pageSize      int
	cursor        int

	// State
	err     error
	loading bool
}

// NewHistoryView creates a new history view
func NewHistoryView(apiClient *api.Client, username string) *HistoryView {
	return &HistoryView{
		apiClient: apiClient,
		username:  username,
		pageSize:  20,
		page:      0,
		cursor:    0,
		loading:   true,
	}
}

// Init initializes the history view
func (h *HistoryView) Init() tea.Cmd {
	return h.loadHistory()
}

// Update handles messages
func (h *HistoryView) Update(msg tea.Msg) (*HistoryView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return h, tea.Quit

		case "esc", "q":
			return h, func() tea.Msg { return backToMenuMsg{} }

		case "up", "k":
			if h.cursor > 0 {
				h.cursor--
			}

		case "down", "j":
			pageEntries := h.getPageEntries()
			if h.cursor < len(pageEntries)-1 {
				h.cursor++
			}

		case "left", "pgup", "h":
			if h.page > 0 {
				h.page--
				h.cursor = 0
			}

		case "right", "pgdown", "l":
			totalPages := h.getTotalPages()
			if h.page < totalPages-1 {
				h.page++
				h.cursor = 0
			}

		case "r":
			// Refresh
			h.loading = true
			return h, h.loadHistory()
		}

	case historyLoadedMsg:
		h.loading = false
		h.allHistory = msg.entries
		// Sort by start time descending
		sort.Slice(h.allHistory, func(i, j int) bool {
			return h.allHistory[i].StartTime().After(h.allHistory[j].StartTime())
		})

	case errorMsg:
		h.loading = false
		h.err = error(msg)
	}

	return h, nil
}

// View renders the history view
func (h *HistoryView) View() string {
	if h.width == 0 {
		return "Loading..."
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Align(lipgloss.Center)

	if h.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			loadingStyle.Render("Loading history..."),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	if h.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			errorStyle.Render("Error: "+h.err.Error()),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Press 'r' to retry, Esc to go back"),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	if len(h.allHistory) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			emptyStyle.Render("No timesheet entries found"),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("Press Esc to go back"),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Pagination info
	totalPages := h.getTotalPages()
	title := titleStyle.Render(fmt.Sprintf("Timesheet History - Page %d of %d (%d entries)", h.page+1, totalPages, len(h.allHistory)))

	// Render table
	table := h.renderTable()

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true).
		Align(lipgloss.Center)

	help := helpStyle.Render("↑/↓: Navigate • ←/→: Change page • r: Refresh • Esc/q: Back to menu")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		table,
		"",
		help,
	)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Top, content)
}

// renderTable renders the history table
func (h *HistoryView) renderTable() string {
	pageEntries := h.getPageEntries()

	// Table styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Width(16).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(16).
		Align(lipgloss.Left)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#E95420")).
		Width(16).
		Align(lipgloss.Left)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(90).
		Align(lipgloss.Left)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#E95420")).
		Width(90).
		Align(lipgloss.Left)

	// Header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Project"),
		headerStyle.Render("Task"),
		headerStyle.Render("Activity"),
		headerStyle.Render("Date"),
		headerStyle.Width(10).Render("Hours"),
		headerStyle.Width(12).Render("Status"),
	)

	// Rows
	var rows []string
	for i, entry := range pageEntries {
		isSelected := i == h.cursor

		// Truncate long values
		project := truncate(entry.ProjectName, 14)
		task := truncate(entry.TaskName, 14)
		activity := truncate(entry.Activity(), 14)
		dateStr := entry.StartTime().Format("2006-01-02")
		hours := fmt.Sprintf("%.2fh", entry.Duration())

		status := "Pending"
		if entry.IsSubmitted() {
			status = "✓ Submitted"
		} else if entry.EndTime().IsZero() {
			status = "● Running"
		}

		var row1 string
		if isSelected {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedStyle.Render(project),
				selectedStyle.Render(task),
				selectedStyle.Render(activity),
				selectedStyle.Render(dateStr),
				selectedStyle.Width(10).Render(hours),
				selectedStyle.Width(12).Render(status),
			)
		} else {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(project),
				cellStyle.Render(task),
				cellStyle.Render(activity),
				cellStyle.Render(dateStr),
				cellStyle.Width(10).Render(hours),
				cellStyle.Width(12).Render(status),
			)
		}

		// Description row
		desc := entry.GetDescriptionString()
		if desc == "" {
			desc = "(no description)"
		}
		renderedDesc := renderHistoryDescription(desc, 86)

		var row2 string
		if isSelected {
			row2 = selectedDescStyle.Render("  " + renderedDesc)
		} else {
			row2 = descStyle.Render("  " + renderedDesc)
		}

		rows = append(rows, row1, row2)

		// Separator
		if i < len(pageEntries)-1 {
			sep := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444444")).
				Render(strings.Repeat("─", 90))
			rows = append(rows, sep)
		}
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, rows...)...)

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E95420")).
		Padding(1, 2)

	return tableStyle.Render(tableContent)
}

// Helper functions
func (h *HistoryView) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if h.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		entries, err := h.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		return historyLoadedMsg{entries: entries}
	}
}

func (h *HistoryView) getTotalPages() int {
	if len(h.allHistory) == 0 {
		return 1
	}
	return (len(h.allHistory) + h.pageSize - 1) / h.pageSize
}

func (h *HistoryView) getPageEntries() []api.TimelogEntry {
	if len(h.allHistory) == 0 {
		return nil
	}

	start := h.page * h.pageSize
	end := start + h.pageSize
	if end > len(h.allHistory) {
		end = len(h.allHistory)
	}

	return h.allHistory[start:end]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func renderHistoryDescription(html string, width int) string {
	if html == "" {
		return "(no description)"
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return strings.TrimSpace(html)
	}

	rendered, err := r.Render(html)
	if err != nil {
		return strings.TrimSpace(html)
	}

	rendered = strings.TrimSpace(rendered)
	for strings.Contains(rendered, "\n\n") {
		rendered = strings.ReplaceAll(rendered, "\n\n", "\n")
	}

	// Take only first line for compact display
	if idx := strings.Index(rendered, "\n"); idx != -1 {
		rendered = rendered[:idx]
	}

	return rendered
}

// Message types
type historyLoadedMsg struct {
	entries []api.TimelogEntry
}
