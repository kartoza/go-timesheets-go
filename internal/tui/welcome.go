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
	Password string
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

	case tea.MouseMsg:
		// Handle mouse clicks on input fields
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			clickedIndex := m.getInputFromMousePosition(msg.X, msg.Y)
			if clickedIndex >= 0 && clickedIndex < len(m.inputs) {
				m.focusIndex = clickedIndex
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
		}
		return m, nil

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
		Foreground(lipgloss.Color("#DDA036")).
		Padding(1, 0).
		Align(lipgloss.Center)

	title := titleStyle.Render("Kartoza Timesheets")
	content = append(content, title)

	// Subtitle/Motto
	mottoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	motto := mottoStyle.Render("Time Flies")
	content = append(content, motto)
	content = append(content, "")

	// Divider
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center)
	divider := dividerStyle.Render("────────────────────────────────────────────────────────────")
	content = append(content, divider)
	content = append(content, "")

	// Login form box
	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Margin(1, 0)

	welcomeText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Render("Please sign in to continue")

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
			Foreground(lipgloss.Color("#CC0403")).
			Bold(true)
		formContent = append(formContent, "")
		formContent = append(formContent, errorStyle.Render("Error: "+m.errorMsg))
	}

	// Loading indicator
	if m.loggingIn {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			Bold(true)
		formContent = append(formContent, "")
		formContent = append(formContent, loadingStyle.Render("Logging in..."))
	}

	formBox := formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formContent...))
	content = append(content, formBox)

	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8A8B8B")).
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

		// Save token and password to disk (password needed for session auth on POST requests)
		if err := config.SaveTokenWithPassword(loginResp.Token, username, password, baseURL); err != nil {
			return LoginErrorMsg{Err: fmt.Errorf("failed to save token: %w", err)}
		}

		return LoginSuccessMsg{
			Token:    loginResp.Token,
			Username: username,
			BaseURL:  baseURL,
			Password: password,
		}
	}
}

// getInputFromMousePosition returns the input index from mouse coordinates
// Returns -1 if the click is not on a valid input field
func (m *LoginModel) getInputFromMousePosition(x, y int) int {
	// The form is centered on screen
	// Calculate the approximate position of the form

	// Form layout (centered):
	// - Title (2 lines with padding)
	// - Motto (1 line)
	// - Empty line
	// - Divider (1 line)
	// - Empty line
	// - Form box with:
	//   - Margin top (1 line)
	//   - Border top (1 line)
	//   - Padding top (1 line)
	//   - "Please sign in to continue" (1 line)
	//   - Empty line
	//   - Input 0 (API URL) - 1 line
	//   - Input 1 (Username) - 1 line
	//   - Input 2 (Password) - 1 line

	// Approximate vertical center
	formHeight := 15 // rough estimate of form height
	formStartY := (m.height - formHeight) / 2

	// Calculate input field positions
	// After title (2), motto (1), empty (1), divider (1), empty (1), margin (1), border (1), padding (1), welcome text (1), empty (1)
	inputStartY := formStartY + 11

	// Each input field is 1 line tall
	for i := 0; i < len(m.inputs); i++ {
		inputY := inputStartY + i
		if y == inputY {
			return i
		}
	}

	return -1
}
