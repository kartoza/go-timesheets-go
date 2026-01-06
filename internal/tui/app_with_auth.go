package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

// AppState represents the application state
type AppState int

const (
	StateLogin AppState = iota
	StateMainMenu
	StateTimerCreation
	StateHistoryView
	StateWorkspaceAssociations
)

// AppWithAuth is a wrapper app that handles authentication
type AppWithAuth struct {
	state              AppState
	loginModel         *LoginModel
	mainMenu           *MainMenuModel
	timerCreationView  *TimesheetApp
	historyView        *TimesheetApp
	workspaceView      *WorkspaceAssociationsModel
	width              int
	height             int
	tokenLoaded        bool
	apiClient          *api.Client
	username           string
	spinner            spinner.Model
}

// NewAppWithAuth creates a new app with authentication
func NewAppWithAuth() (*AppWithAuth, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#E95420"))

	app := &AppWithAuth{
		state:   StateLogin,
		spinner: s,
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

		// Create API client with the stored token
		apiClient, err := createAPIClient(token.Token, token.Username, token.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create API client: %w", err)
		}

		app.apiClient = apiClient
		app.username = token.Username

		// Create main menu
		mainMenu := NewMainMenu(apiClient, token.Username)
		app.mainMenu = mainMenu
		app.state = StateMainMenu
		return app, nil
	}

	// No token, show login
	return app, nil
}

// Init initializes the app
func (a *AppWithAuth) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, a.spinner.Tick)

	switch a.state {
	case StateLogin:
		// Initialize login model if not already done
		if a.loginModel == nil {
			defaultURL := "https://timesheets.kartoza.com"
			loginModel := NewLoginModel(defaultURL)
			a.loginModel = &loginModel
		}
		cmds = append(cmds, a.loginModel.Init())

	case StateMainMenu:
		if a.mainMenu != nil {
			cmds = append(cmds, a.mainMenu.Init())
		}

	case StateTimerCreation:
		if a.timerCreationView != nil {
			cmds = append(cmds, a.timerCreationView.Init())
		}

	case StateHistoryView:
		if a.historyView != nil {
			cmds = append(cmds, a.historyView.Init())
		}

	case StateWorkspaceAssociations:
		if a.workspaceView != nil {
			cmds = append(cmds, a.workspaceView.Init())
		}
	}

	return tea.Batch(cmds...)
}

// Update handles messages
func (a *AppWithAuth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update spinner
	var cmd tea.Cmd
	a.spinner, cmd = a.spinner.Update(msg)
	cmds = append(cmds, cmd)

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
		case StateMainMenu:
			if a.mainMenu != nil {
				model, c := a.mainMenu.Update(msg)
				if m, ok := model.(*MainMenuModel); ok {
					a.mainMenu = m
				}
				cmds = append(cmds, c)
			}
		case StateTimerCreation:
			if a.timerCreationView != nil {
				model, c := a.timerCreationView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.timerCreationView = m
				}
				cmds = append(cmds, c)
			}
		case StateHistoryView:
			if a.historyView != nil {
				model, c := a.historyView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.historyView = m
				}
				cmds = append(cmds, c)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
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
		case StateMainMenu:
			if a.mainMenu != nil {
				model, c := a.mainMenu.Update(msg)
				if m, ok := model.(*MainMenuModel); ok {
					a.mainMenu = m
				}
				cmds = append(cmds, c)
			}
		case StateTimerCreation:
			if a.timerCreationView != nil {
				model, c := a.timerCreationView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.timerCreationView = m
				}
				cmds = append(cmds, c)
			}
		case StateHistoryView:
			if a.historyView != nil {
				model, c := a.historyView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.historyView = m
				}
				cmds = append(cmds, c)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
			}
		}

	case LoginSuccessMsg:
		// Login succeeded, create API client and transition to main menu
		apiClient, err := createAPIClient(msg.Token, msg.Username, msg.BaseURL)
		if err != nil {
			// Handle error - stay on login screen with error
			if a.loginModel != nil {
				errMsg := LoginErrorMsg{Err: fmt.Errorf("failed to create API client: %w", err)}
				model, c := a.loginModel.Update(errMsg)
				if m, ok := model.(LoginModel); ok {
					a.loginModel = &m
				}
				return a, c
			}
			return a, nil
		}

		a.apiClient = apiClient
		a.username = msg.Username

		// Create main menu
		mainMenu := NewMainMenu(apiClient, msg.Username)
		a.mainMenu = mainMenu
		a.state = StateMainMenu
		return a, a.mainMenu.Init()

	case LoginErrorMsg:
		// Pass to login model
		if a.loginModel != nil {
			model, c := a.loginModel.Update(msg)
			if m, ok := model.(LoginModel); ok {
				a.loginModel = &m
			}
			cmds = append(cmds, c)
		}

	case launchTimerCreationMsg:
		// Transition to timer creation with authenticated API client
		timerApp, err := NewTimesheetAppWithClient(a.apiClient, a.username)
		if err != nil {
			// Handle error
			return a, func() tea.Msg {
				return errorMsg(fmt.Errorf("failed to create timer app: %w", err))
			}
		}
		a.timerCreationView = timerApp
		a.state = StateTimerCreation
		return a, a.timerCreationView.Init()

	case backToMenuMsg:
		// Return to main menu from timer creation
		a.state = StateMainMenu
		a.timerCreationView = nil
		a.historyView = nil
		a.workspaceView = nil
		// Reload main menu to refresh state
		if a.mainMenu != nil {
			return a, a.mainMenu.Init()
		}
		return a, nil

	case launchHistoryViewMsg:
		// Transition to history view with authenticated API client
		historyApp, err := NewTimesheetAppWithClientAndTab(a.apiClient, a.username, TabHistory)
		if err != nil {
			// Handle error
			return a, func() tea.Msg {
				return errorMsg(fmt.Errorf("failed to create history view: %w", err))
			}
		}
		a.historyView = historyApp
		a.state = StateHistoryView
		return a, a.historyView.Init()

	case launchWorkspaceAssociationsMsg:
		// Transition to workspace associations view
		workspaceView := NewWorkspaceAssociationsModel(a.apiClient)
		a.workspaceView = workspaceView
		a.state = StateWorkspaceAssociations
		return a, a.workspaceView.Init()

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
		case StateMainMenu:
			if a.mainMenu != nil {
				model, c := a.mainMenu.Update(msg)
				if m, ok := model.(*MainMenuModel); ok {
					a.mainMenu = m
				}
				cmds = append(cmds, c)
			}
		case StateTimerCreation:
			if a.timerCreationView != nil {
				model, c := a.timerCreationView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.timerCreationView = m
				}
				cmds = append(cmds, c)
			}
		case StateHistoryView:
			if a.historyView != nil {
				model, c := a.historyView.Update(msg)
				if m, ok := model.(*TimesheetApp); ok {
					a.historyView = m
				}
				cmds = append(cmds, c)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
			}
		}
	}

	return a, tea.Batch(cmds...)
}

// Message types for state transitions
type launchTimerCreationMsg struct{}
type launchHistoryViewMsg struct{}
type launchWorkspaceAssociationsMsg struct{}
type backToMenuMsg struct{}

// View renders the app
func (a *AppWithAuth) View() string {
	switch a.state {
	case StateLogin:
		if a.loginModel != nil {
			return a.loginModel.View()
		}
		return "Initializing..."

	case StateMainMenu:
		if a.mainMenu != nil {
			return a.mainMenu.View()
		}
		return "Loading main menu..."

	case StateTimerCreation:
		if a.timerCreationView != nil {
			return a.timerCreationView.View()
		}
		return a.renderLoadingScreen("Loading timer creation")

	case StateHistoryView:
		if a.historyView != nil {
			return a.historyView.View()
		}
		return a.renderLoadingScreen("Loading history")

	case StateWorkspaceAssociations:
		if a.workspaceView != nil {
			return a.workspaceView.View()
		}
		return a.renderLoadingScreen("Loading workspace associations")
	}

	return "Unknown state"
}

// renderLoadingScreen renders a centered loading screen with spinner
func (a *AppWithAuth) renderLoadingScreen(message string) string {
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E95420"))
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		MarginTop(1)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		spinnerStyle.Render(a.spinner.View()),
		textStyle.Render(message+"..."),
	)

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// Run starts the application
func (a *AppWithAuth) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// createAPIClient creates an API client with the given token, username and base URL
func createAPIClient(token, username, baseURL string) (*api.Client, error) {
	client, err := api.NewClient(api.Config{
		BaseURL: baseURL,
		Timeout: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	client.SetAuthToken(token)
	client.SetUsername(username)
	return client, nil
}
