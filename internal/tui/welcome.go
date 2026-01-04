package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

// LoginModel represents the login screen model
type LoginModel struct {
	inputs       []textinput.Model
	focusIndex   int
	errorMsg     string
	loggingIn    bool
	loginSuccess bool
	baseURL      string
	width        int
	height       int
}

// LoginSuccessMsg is sent when login succeeds
type LoginSuccessMsg struct {
	Token    string
	Username string
	BaseURL  string
}

// LoginErrorMsg is sent when login fails
type LoginErrorMsg struct {
	Err error
}

// NewLoginModel creates a new login model
func NewLoginModel(baseURL string) LoginModel {
	// Create text inputs
	inputs := make([]textinput.Model, 3)

	// Base URL input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "https://timesheets.kartoza.com"
	inputs[0].SetValue(baseURL)
	inputs[0].CharLimit = 256
	inputs[0].Width = 50
	inputs[0].Prompt = "API URL: "

	// Username input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "username"
	inputs[1].CharLimit = 128
	inputs[1].Width = 50
	inputs[1].Prompt = "Username: "

	// Password input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "password"
	inputs[2].CharLimit = 128
	inputs[2].Width = 50
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].EchoCharacter = '•'
	inputs[2].Prompt = "Password: "

	// Focus the first input
	inputs[0].Focus()

	return LoginModel{
		inputs:  inputs,
		baseURL: baseURL,
	}
}

// Init initializes the login model
func (m LoginModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Handle enter key
			if s == "enter" {
				// If on the last field, submit the form
				if m.focusIndex == len(m.inputs)-1 && !m.loggingIn {
					return m, m.performLogin()
				}

				// Move to next field
				m.focusIndex++
			}

			// Cycle through inputs
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else if s == "tab" || s == "down" {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}

			// Update focus
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					m.inputs[i].Focus()
					cmds = append(cmds, m.inputs[i].Cursor.BlinkCmd())
				} else {
					m.inputs[i].Blur()
				}
			}

			return m, tea.Batch(cmds...)
		}

	case LoginSuccessMsg:
		m.loggingIn = false
		m.loginSuccess = true
		// This will be handled by the parent app

	case LoginErrorMsg:
		m.loggingIn = false
		m.errorMsg = msg.Err.Error()
	}

	// Update the focused input
	if !m.loggingIn {
		cmd := m.updateInputs(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateInputs updates the text inputs
func (m *LoginModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

// View renders the login screen
func (m LoginModel) View() string {
	if m.loginSuccess {
		return ""
	}

	var content []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Padding(1, 0).
		Align(lipgloss.Center)

	title := titleStyle.Render("🕐 Kartoza Timesheet")
	content = append(content, title)

	// Subtitle
	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Align(lipgloss.Center)

	subtitle := subtitleStyle.Render("Ubuntu: \"I am because we are\"")
	content = append(content, subtitle)
	content = append(content, "")

	// Login form box
	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E95420")).
		Padding(1, 2).
		Margin(1, 0)

	welcomeText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Render("Welcome! Please sign in to continue")

	var formContent []string
	formContent = append(formContent, welcomeText)
	formContent = append(formContent, "")

	// Render inputs
	for i := range m.inputs {
		formContent = append(formContent, m.inputs[i].View())
	}

	// Error message
	if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)
		formContent = append(formContent, "")
		formContent = append(formContent, errorStyle.Render("Error: "+m.errorMsg))
	}

	// Loading indicator
	if m.loggingIn {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true)
		formContent = append(formContent, "")
		formContent = append(formContent, loadingStyle.Render("Logging in..."))
	}

	formBox := formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formContent...))
	content = append(content, formBox)

	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Align(lipgloss.Center)

	if !m.loggingIn {
		instructions := instructionStyle.Render("Tab: Next field | Enter: Submit | Esc: Quit")
		content = append(content, instructions)
	}

	// Center everything
	finalContent := lipgloss.JoinVertical(lipgloss.Center, content...)

	// Center horizontally and vertically
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		finalContent,
	)
}

// performLogin performs the login action
func (m *LoginModel) performLogin() tea.Cmd {
	return func() tea.Msg {
		m.loggingIn = true
		m.errorMsg = ""

		baseURL := m.inputs[0].Value()
		username := m.inputs[1].Value()
		password := m.inputs[2].Value()

		// Validate inputs
		if baseURL == "" {
			return LoginErrorMsg{Err: fmt.Errorf("API URL is required")}
		}
		if username == "" {
			return LoginErrorMsg{Err: fmt.Errorf("username is required")}
		}
		if password == "" {
			return LoginErrorMsg{Err: fmt.Errorf("password is required")}
		}

		// Create API client and attempt login
		client, err := api.NewClient(api.Config{
			BaseURL: baseURL,
			Timeout: 30,
		})
		if err != nil {
			return LoginErrorMsg{Err: fmt.Errorf("failed to create API client: %w", err)}
		}

		loginResp, err := client.Login(username, password)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}

		// Save token to disk
		if err := config.SaveToken(loginResp.Token, username, baseURL); err != nil {
			return LoginErrorMsg{Err: fmt.Errorf("failed to save token: %w", err)}
		}

		return LoginSuccessMsg{
			Token:    loginResp.Token,
			Username: username,
			BaseURL:  baseURL,
		}
	}
}
