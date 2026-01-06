package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
)

// WorkspaceAssociationsModel represents the workspace associations screen
type WorkspaceAssociationsModel struct {
	apiClient *api.Client
	width     int
	height    int
}

// NewWorkspaceAssociationsModel creates a new workspace associations model
func NewWorkspaceAssociationsModel(apiClient *api.Client) *WorkspaceAssociationsModel {
	return &WorkspaceAssociationsModel{
		apiClient: apiClient,
		width:     80,
		height:    24,
	}
}

// Init initializes the workspace associations view
func (m *WorkspaceAssociationsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *WorkspaceAssociationsModel) Update(msg tea.Msg) (*WorkspaceAssociationsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q", "ctrl+c"))):
			// Return to main menu
			return m, func() tea.Msg {
				return backToMenuMsg{}
			}
		}
	}

	return m, nil
}

// View renders the workspace associations screen
func (m *WorkspaceAssociationsModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Align(lipgloss.Center).
		Width(60)

	mottoStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#888888")).
		Align(lipgloss.Center).
		Width(60)

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Width(60)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(1, 2).
		Width(60)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true).
		Align(lipgloss.Center).
		Width(60)

	title := titleStyle.Render("Workspace Associations")
	motto := mottoStyle.Render("Automatic Time Tracking")
	divider := dividerStyle.Render("────────────────────────────────────────────────────────────")

	content := contentStyle.Render(`🚧 Coming Soon! 🚧

This feature will allow you to automatically track time based on which
virtual desktop or workspace is active.

Planned Features:
  • Associate projects and tasks with specific workspaces
  • Automatic timer start/stop based on workspace switching
  • Integration with i3, Sway, Hyprland, and other window managers
  • Manual override and editing capabilities

This will help streamline your workflow by reducing manual timer
management when switching between different work contexts.`)

	help := helpStyle.Render("Press Esc or q to return to main menu")

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		motto,
		divider,
		"",
		content,
		"",
		divider,
		help,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Top,
		view,
	)
}
