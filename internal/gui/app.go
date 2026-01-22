package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/systray"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/screens"
	"github.com/kartoza/go-timesheets-go/internal/pow"
)

// App represents the main GUI application
type App struct {
	fyneApp    fyne.App
	window     fyne.Window
	apiClient  *api.Client
	powCapture *pow.Capturer

	// Screens
	loginScreen      *screens.LoginScreen
	dashboardScreen  *screens.DashboardScreen
	timesheetScreen  *screens.TimesheetScreen
	historyScreen    *screens.HistoryScreen
	settingsScreen   *screens.SettingsScreen
	favouritesScreen *screens.FavouritesScreen
	workspacesScreen *screens.WorkspacesScreen
	codeReposScreen  *screens.CodeReposScreen

	// Main container
	content *fyne.Container

	// System tray
	systrayManager *SystrayManager
	windowVisible  bool
}

// NewApp creates a new GUI application
func NewApp(powCapture *pow.Capturer) *App {
	return &App{
		powCapture: powCapture,
	}
}

// Run starts the GUI application
func (a *App) Run() error {
	// Create Fyne app
	a.fyneApp = app.NewWithID("com.kartoza.timesheets")
	a.fyneApp.Settings().SetTheme(NewUbuntuTheme())

	// Create main window
	a.window = a.fyneApp.NewWindow("Kartoza Timesheets")
	a.window.Resize(fyne.NewSize(480, 640))
	a.window.CenterOnScreen()

	// Create content container
	a.content = container.NewStack()
	a.window.SetContent(a.content)

	// Set up window close handler - hide instead of quit
	a.window.SetCloseIntercept(func() {
		a.hideWindow()
	})

	// Create system tray manager
	a.systrayManager = NewSystrayManager(a)

	// Start listening for systray events
	go a.handleSystrayEvents()

	// Start with login screen
	a.showLogin()

	// Mark window as visible
	a.windowVisible = true

	// Run systray in a goroutine (it blocks)
	go systray.Run(a.systrayManager.OnReady, a.systrayManager.OnExit)

	// Run the Fyne app
	a.window.ShowAndRun()
	return nil
}

// handleSystrayEvents listens for systray events
func (a *App) handleSystrayEvents() {
	for {
		select {
		case <-a.systrayManager.ShowChan():
			a.showWindow()
		case <-a.systrayManager.QuitChan():
			a.quit()
			return
		}
	}
}

// showWindow shows and focuses the main window
func (a *App) showWindow() {
	if a.window != nil {
		a.window.Show()
		a.window.RequestFocus()
		a.windowVisible = true
	}
}

// hideWindow hides the main window but keeps the app running
func (a *App) hideWindow() {
	if a.window != nil {
		a.window.Hide()
		a.windowVisible = false
	}
}

// quit cleanly exits the application
func (a *App) quit() {
	systray.Quit()
	if a.fyneApp != nil {
		a.fyneApp.Quit()
	}
}

func (a *App) showLogin() {
	if a.loginScreen == nil {
		a.loginScreen = screens.NewLoginScreen(a.window)
		a.loginScreen.OnLogin = func(client *api.Client, username string) {
			a.apiClient = client
			a.showDashboard()
		}
	}

	a.setContent(a.loginScreen.Container)

	// Try auto-login
	a.loginScreen.TryAutoLogin()
}

func (a *App) showDashboard() {
	if a.dashboardScreen == nil {
		a.dashboardScreen = screens.NewDashboardScreen(a.apiClient, a.window, a.powCapture)
		a.dashboardScreen.OnSettings = a.showSettings
		a.dashboardScreen.OnHistory = a.showHistory
		a.dashboardScreen.OnNewTimesheet = a.showTimesheet
		a.dashboardScreen.OnTimerStatusChange = a.onTimerStatusChange
	}

	a.setContent(a.dashboardScreen.Container)
	a.dashboardScreen.Refresh()
}

// onTimerStatusChange updates the systray when timer status changes
func (a *App) onTimerStatusChange(running bool, projectName, taskName string) {
	if a.systrayManager == nil {
		return
	}
	if running {
		a.systrayManager.SetTimerRunning(projectName, taskName)
	} else {
		a.systrayManager.SetTimerStopped()
	}
}

func (a *App) showTimesheet() {
	if a.timesheetScreen == nil {
		a.timesheetScreen = screens.NewTimesheetScreen(a.apiClient, a.window)
		a.timesheetScreen.OnBack = a.showDashboard
		a.timesheetScreen.OnTimerStarted = func() {
			a.showDashboard()
		}
	}

	a.setContent(a.timesheetScreen.Container)
	a.timesheetScreen.Refresh()
}

func (a *App) showHistory() {
	if a.historyScreen == nil {
		a.historyScreen = screens.NewHistoryScreen(a.apiClient, a.window)
		a.historyScreen.OnBack = a.showDashboard
	}

	a.setContent(a.historyScreen.Container)
	a.historyScreen.Refresh()
}

func (a *App) showSettings() {
	if a.settingsScreen == nil {
		a.settingsScreen = screens.NewSettingsScreen(a.window)
		a.settingsScreen.OnBack = a.showDashboard
		a.settingsScreen.OnEditFavourites = a.showFavourites
		a.settingsScreen.OnWorkspaces = a.showWorkspaces
		a.settingsScreen.OnCodeRepos = a.showCodeRepos
		a.settingsScreen.OnLogout = a.handleLogout
	}

	a.setContent(a.settingsScreen.Container)
}

func (a *App) showFavourites() {
	if a.favouritesScreen == nil {
		a.favouritesScreen = screens.NewFavouritesScreen(a.apiClient, a.window)
		a.favouritesScreen.OnBack = a.showSettings
	}

	a.setContent(a.favouritesScreen.Container)
	a.favouritesScreen.Refresh()
}

func (a *App) showWorkspaces() {
	if a.workspacesScreen == nil {
		a.workspacesScreen = screens.NewWorkspacesScreen(a.apiClient, a.window)
		a.workspacesScreen.OnBack = a.showSettings
	}

	a.setContent(a.workspacesScreen.Container)
	a.workspacesScreen.Refresh()
}

func (a *App) showCodeRepos() {
	if a.codeReposScreen == nil {
		a.codeReposScreen = screens.NewCodeReposScreen(a.apiClient, a.window)
		a.codeReposScreen.OnBack = a.showSettings
	}

	a.setContent(a.codeReposScreen.Container)
	a.codeReposScreen.Refresh()
}

func (a *App) handleLogout() {
	// Clear API client
	a.apiClient = nil

	// Reset screens
	a.dashboardScreen = nil
	a.timesheetScreen = nil
	a.historyScreen = nil
	a.settingsScreen = nil
	a.favouritesScreen = nil
	a.workspacesScreen = nil
	a.codeReposScreen = nil
	a.loginScreen = nil

	// Show login
	a.showLogin()
}

func (a *App) setContent(content fyne.CanvasObject) {
	a.content.Objects = []fyne.CanvasObject{content}
	a.content.Refresh()
}
