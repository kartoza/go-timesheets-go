package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/monitoring"
	"github.com/kartoza/go-timesheets-go/internal/pow"
)

// AppState represents the application state
type AppState int

const (
	StateLogin AppState = iota
	StateMainMenu
	StateTimesheetCreation
	StateHistoryView
	StateWorkspaceAssociations
	StateCodeRepos
)

// AppWithAuth is a wrapper app that handles authentication
type AppWithAuth struct {
	state              AppState
	loginModel         *LoginModel
	mainMenu           *MainMenuModel
	timesheetCreator   *TimesheetCreator
	historyView        *HistoryView
	workspaceView      *WorkspaceAssociationsModel
	codeReposView      *CodeReposModel
	width              int
	height             int
	tokenLoaded        bool
	apiClient          *api.Client
	username           string
	spinner            spinner.Model
	monitoringServer   *monitoring.Server
	metrics            *monitoring.Metrics
	powCapturer        *pow.Capturer
}

// NewAppWithAuth creates a new app with authentication
// powCapturer can be nil if POW mode is not enabled
func NewAppWithAuth(powCapturer *pow.Capturer) (*AppWithAuth, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))

	var metrics *monitoring.Metrics
	var monitoringServer *monitoring.Server

	// Only initialize monitoring in debug builds
	if DebugEnabled {
		homeDir, _ := os.UserHomeDir()
		logDir := filepath.Join(homeDir, ".config/.kartoza-timesheets/logs")
		var err error
		metrics, err = monitoring.Initialize(logDir)
		if err != nil {
			// Log error but don't fail app startup
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize monitoring: %v\n", err)
		}

		// Start monitoring server
		monitoringServer = monitoring.NewServer(6060)
		if err := monitoringServer.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to start monitoring server: %v\n", err)
		} else if metrics != nil {
			fmt.Fprintf(os.Stderr, "Monitoring server started on http://localhost:6060\n")
		}
	}

	app := &AppWithAuth{
		state:            StateLogin,
		spinner:          s,
		monitoringServer: monitoringServer,
		metrics:          metrics,
		powCapturer:      powCapturer,
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
		apiClient, err := createAPIClient(token.Token, token.Username, token.BaseURL, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create API client: %w", err)
		}

		app.apiClient = apiClient
		app.username = token.Username

		// Create main menu
		mainMenu := NewMainMenu(apiClient, token.Username, powCapturer)
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

	case StateTimesheetCreation:
		if a.timesheetCreator != nil {
			cmds = append(cmds, a.timesheetCreator.Init())
		}

	case StateHistoryView:
		if a.historyView != nil {
			cmds = append(cmds, a.historyView.Init())
		}

	case StateWorkspaceAssociations:
		if a.workspaceView != nil {
			cmds = append(cmds, a.workspaceView.Init())
		}

	case StateCodeRepos:
		if a.codeReposView != nil {
			cmds = append(cmds, a.codeReposView.Init())
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
		case StateTimesheetCreation:
			if a.timesheetCreator != nil {
				a.timesheetCreator, _ = a.timesheetCreator.Update(msg)
			}
		case StateHistoryView:
			if a.historyView != nil {
				a.historyView, _ = a.historyView.Update(msg)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
			}
		case StateCodeRepos:
			if a.codeReposView != nil {
				model, c := a.codeReposView.Update(msg)
				a.codeReposView = model
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
		case StateTimesheetCreation:
			if a.timesheetCreator != nil {
				var c tea.Cmd
				a.timesheetCreator, c = a.timesheetCreator.Update(msg)
				cmds = append(cmds, c)
			}
		case StateHistoryView:
			if a.historyView != nil {
				var c tea.Cmd
				a.historyView, c = a.historyView.Update(msg)
				cmds = append(cmds, c)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
			}
		case StateCodeRepos:
			if a.codeReposView != nil {
				model, c := a.codeReposView.Update(msg)
				a.codeReposView = model
				cmds = append(cmds, c)
			}
		}

	case LoginSuccessMsg:
		// Login succeeded, create API client and transition to main menu
		apiClient, err := createAPIClient(msg.Token, msg.Username, msg.BaseURL, a.metrics)
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
		mainMenu := NewMainMenu(apiClient, msg.Username, a.powCapturer)
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
		// Transition to new unified timesheet creator
		timesheetCreator := NewTimesheetCreator(a.apiClient, a.username)
		timesheetCreator.width = a.width
		timesheetCreator.height = a.height
		timesheetCreator.SetHeaderState(a.getHeaderState())
		a.timesheetCreator = timesheetCreator
		a.state = StateTimesheetCreation
		return a, a.timesheetCreator.Init()

	case backToMenuMsg:
		// Return to main menu
		a.state = StateMainMenu
		a.timesheetCreator = nil
		a.historyView = nil
		a.workspaceView = nil
		a.codeReposView = nil
		// Reload main menu to refresh state
		if a.mainMenu != nil {
			return a, a.mainMenu.Init()
		}
		return a, nil

	case backToMenuWithHoursMsg:
		// Return to main menu with updated hours from history view (no API call needed)
		a.state = StateMainMenu
		a.historyView = nil
		// Update main menu hours directly from the cached data
		if a.mainMenu != nil {
			a.mainMenu.monthlyHours = msg.monthlyHours
			a.mainMenu.todayHours = msg.todayHours
			a.mainMenu.updateDashboard()
		}
		return a, nil

	case launchHistoryViewMsg:
		// Transition to history view
		historyView := NewHistoryView(a.apiClient, a.username)
		historyView.width = a.width
		historyView.height = a.height
		historyView.SetHeaderState(a.getHeaderState())
		a.historyView = historyView
		a.state = StateHistoryView
		return a, a.historyView.Init()

	case launchWorkspaceAssociationsMsg:
		// Transition to workspace associations view
		workspaceView := NewWorkspaceAssociationsModel(a.apiClient)
		workspaceView.width = a.width
		workspaceView.height = a.height
		workspaceView.SetHeaderState(a.getHeaderState())
		a.workspaceView = workspaceView
		a.state = StateWorkspaceAssociations
		return a, a.workspaceView.Init()

	case launchCodeReposMsg:
		// Transition to code repos view
		codeReposView := NewCodeReposModel(a.apiClient)
		codeReposView.width = a.width
		codeReposView.height = a.height
		codeReposView.SetHeaderState(a.getHeaderState())
		a.codeReposView = codeReposView
		a.state = StateCodeRepos
		return a, a.codeReposView.Init()

	case launchOfficeViewMsg:
		// Launch office view as a subprocess that takes over the terminal
		return a, a.launchOffice()

	case officeFinishedMsg:
		// Office view finished, return to main menu
		return a, nil

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
		case StateTimesheetCreation:
			if a.timesheetCreator != nil {
				var c tea.Cmd
				a.timesheetCreator, c = a.timesheetCreator.Update(msg)
				cmds = append(cmds, c)
			}
		case StateHistoryView:
			if a.historyView != nil {
				var c tea.Cmd
				a.historyView, c = a.historyView.Update(msg)
				cmds = append(cmds, c)
			}
		case StateWorkspaceAssociations:
			if a.workspaceView != nil {
				model, c := a.workspaceView.Update(msg)
				a.workspaceView = model
				cmds = append(cmds, c)
			}
		case StateCodeRepos:
			if a.codeReposView != nil {
				model, c := a.codeReposView.Update(msg)
				a.codeReposView = model
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
type launchCodeReposMsg struct{}
type launchOfficeViewMsg struct{}
type officeFinishedMsg struct{}
type backToMenuMsg struct{}

// getHeaderState returns the current header state from the main menu
func (a *AppWithAuth) getHeaderState() *HeaderState {
	if a.mainMenu != nil {
		return &HeaderState{
			Username:     a.username,
			IsActive:     a.mainMenu.activeTimer != nil,
			MonthlyHours: a.mainMenu.monthlyHours,
		}
	}
	return &HeaderState{Username: a.username}
}

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

	case StateTimesheetCreation:
		if a.timesheetCreator != nil {
			return a.timesheetCreator.View()
		}
		return a.renderLoadingScreen("Loading timesheet creator")

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

	case StateCodeRepos:
		if a.codeReposView != nil {
			return a.codeReposView.View()
		}
		return a.renderLoadingScreen("Loading code repositories")
	}

	return "Unknown state"
}

// renderLoadingScreen renders a centered loading screen with spinner
func (a *AppWithAuth) renderLoadingScreen(message string) string {
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))
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
	// Show splash screen first (3 seconds, skippable with any key)
	// This runs as a separate program before the main app
	if err := ShowSplashScreen(3 * time.Second); err != nil {
		// If splash fails, continue anyway - it's not critical
		_ = err
	}

	// Now run the main application
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()

	// Cleanup on exit
	if a.monitoringServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.monitoringServer.Stop(ctx)
	}

	if a.metrics != nil {
		a.metrics.Close()
	}

	return err
}

// createAPIClient creates an API client with the given token, username and base URL
func createAPIClient(token, username, baseURL string, metrics *monitoring.Metrics) (*api.Client, error) {
	client, err := api.NewClient(api.Config{
		BaseURL: baseURL,
		Timeout: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	client.SetAuthToken(token)
	client.SetUsername(username)

	// Set metrics if available
	if metrics != nil {
		client.SetMetrics(metrics)
	}

	return client, nil
}

// launchOffice launches the office view as a subprocess
func (a *AppWithAuth) launchOffice() tea.Cmd {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return officeFinishedMsg{}
		}
	}

	c := exec.Command(execPath, "office")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return officeFinishedMsg{}
	})
}
