package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartoza/go-timesheets-go/internal/config"
)

// AppState represents the application state
type AppState int

const (
	StateLogin AppState = iota
	StateMain
)

// AppWithAuth is a wrapper app that handles authentication
type AppWithAuth struct {
	state       AppState
	loginModel  *LoginModel
	mainApp     tea.Model
	width       int
	height      int
	tokenLoaded bool
}

// NewAppWithAuth creates a new app with authentication
func NewAppWithAuth() (*AppWithAuth, error) {
	app := &AppWithAuth{
		state: StateLogin,
	}

	// Check if token exists
	token, err := config.LoadToken()
	if err != nil {
		// Token file is corrupted or unreadable, start with login
		return app, nil
	}

	if token != nil {
		// Token exists, try to use it
		app.tokenLoaded = true

		// Try to create the main app - it will load the token and handle API errors gracefully
		// We don't do a health check here because network issues or API downtime
		// shouldn't delete a valid token
		mainApp, err := NewSimpleApp()
		if err != nil {
			return nil, fmt.Errorf("failed to create main app: %w", err)
		}

		app.mainApp = mainApp
		app.state = StateMain
		return app, nil
	}

	// No token, show login
	return app, nil
}

// Init initializes the app
func (a *AppWithAuth) Init() tea.Cmd {
	switch a.state {
	case StateLogin:
		// Initialize login model if not already done
		if a.loginModel == nil {
			defaultURL := "https://timesheets.kartoza.com"
			loginModel := NewLoginModel(defaultURL)
			a.loginModel = &loginModel
		}
		return a.loginModel.Init()

	case StateMain:
		if a.mainApp != nil {
			return a.mainApp.Init()
		}
	}

	return nil
}

// Update handles messages
func (a *AppWithAuth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		// Pass to active model
		switch a.state {
		case StateLogin:
			if a.loginModel != nil {
				model, c := a.loginModel.Update(msg)
				if m, ok := model.(LoginModel); ok {
					a.loginModel = &m
				}
				cmds = append(cmds, c)
			}
		case StateMain:
			if a.mainApp != nil {
				a.mainApp, cmd = a.mainApp.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.KeyMsg:
		// Handle quit globally
		if msg.String() == "ctrl+c" && a.state != StateLogin {
			return a, tea.Quit
		}

		// Pass to active model
		switch a.state {
		case StateLogin:
			if a.loginModel != nil {
				model, c := a.loginModel.Update(msg)
				if m, ok := model.(LoginModel); ok {
					a.loginModel = &m
				}
				cmds = append(cmds, c)
			}
		case StateMain:
			if a.mainApp != nil {
				a.mainApp, cmd = a.mainApp.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case LoginSuccessMsg:
		// Login succeeded, transition to main app
		mainApp, err := NewSimpleApp()
		if err != nil {
			// Handle error - stay on login screen with error
			if a.loginModel != nil {
				errMsg := LoginErrorMsg{Err: fmt.Errorf("failed to initialize app: %w", err)}
				model, c := a.loginModel.Update(errMsg)
				if m, ok := model.(LoginModel); ok {
					a.loginModel = &m
				}
				return a, c
			}
			return a, nil
		}

		a.mainApp = mainApp
		a.state = StateMain
		return a, a.mainApp.Init()

	case LoginErrorMsg:
		// Pass to login model
		if a.loginModel != nil {
			model, c := a.loginModel.Update(msg)
			if m, ok := model.(LoginModel); ok {
				a.loginModel = &m
			}
			cmds = append(cmds, c)
		}

	default:
		// Pass to active model
		switch a.state {
		case StateLogin:
			if a.loginModel != nil {
				model, c := a.loginModel.Update(msg)
				if m, ok := model.(LoginModel); ok {
					a.loginModel = &m
				}
				cmds = append(cmds, c)
			}
		case StateMain:
			if a.mainApp != nil {
				a.mainApp, cmd = a.mainApp.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return a, tea.Batch(cmds...)
}

// View renders the app
func (a *AppWithAuth) View() string {
	switch a.state {
	case StateLogin:
		if a.loginModel != nil {
			return a.loginModel.View()
		}
		return "Initializing..."

	case StateMain:
		if a.mainApp != nil {
			return a.mainApp.View()
		}
		return "Loading application..."
	}

	return "Unknown state"
}

// Run starts the application
func (a *AppWithAuth) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
