package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/screens"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/gui/widgets"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// App represents the main GUI application
type App struct {
	fyneApp    fyne.App
	window     fyne.Window
	apiClient  *api.Client
	powCapture *pow.Capturer

	// Services
	calendarService *service.CalendarService

	// Screens
	loginScreen            *screens.LoginScreen
	dashboardScreen        *screens.DashboardScreen
	historyScreen          *screens.HistoryScreen
	gapsScreen             *screens.GapsScreen
	settingsScreen         *screens.SettingsScreen
	favouritesScreen       *screens.FavouritesScreen
	codeReposScreen        *screens.CodeReposScreen
	officeScreen           *screens.OfficeScreen
	aiAssistantScreen      *screens.AIAssistantScreen
	calendarSettingsScreen *screens.CalendarSettingsScreen

	// Navigation
	currentScreen string // Track current screen for ESC navigation

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

	// Global ESC key handler - close popup if open, otherwise navigate back
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			// Check if there's a modal popup overlay to close first
			overlays := a.window.Canvas().Overlays()
			if overlays != nil && overlays.Top() != nil {
				overlays.Top().Hide()
				return
			}
			a.navigateBack()
		}
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
	fyne.Do(func() {
		if a.window != nil {
			a.window.Show()
			a.window.RequestFocus()
			a.windowVisible = true
		}
	})
}

// hideWindow hides the main window but keeps the app running
func (a *App) hideWindow() {
	fyne.Do(func() {
		if a.window != nil {
			a.window.Hide()
			a.windowVisible = false
		}
	})
}

// quit cleanly exits the application
func (a *App) quit() {
	systray.Quit()
	fyne.Do(func() {
		if a.fyneApp != nil {
			a.fyneApp.Quit()
		}
	})
}

// navigateBack handles ESC key to go to the previous screen
func (a *App) navigateBack() {
	switch a.currentScreen {
	case "history":
		a.showDashboard()
	case "gaps":
		a.showDashboard()
	case "settings":
		a.showDashboard()
	case "favourites":
		a.showSettings()
	case "coderepos":
		a.showSettings()
	case "calendarsettings":
		a.showSettings()
	case "office":
		if a.officeScreen != nil {
			a.officeScreen.StopAnimation()
		}
		a.showDashboard()
	case "aiassistant":
		a.showDashboard()
	// login and dashboard have no back navigation
	}
}

func (a *App) showLogin() {
	a.currentScreen = "login"

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
	a.currentScreen = "dashboard"

	if a.dashboardScreen == nil {
		a.dashboardScreen = screens.NewDashboardScreen(a.apiClient, a.window, a.powCapture)
		a.dashboardScreen.OnSettings = a.showSettings
		a.dashboardScreen.OnHistory = a.showHistory
		a.dashboardScreen.OnGaps = a.showGaps
		a.dashboardScreen.OnOffice = a.showOffice
		a.dashboardScreen.OnAIAssistant = a.showAIAssistant
		a.dashboardScreen.OnTimerStatusChange = a.onTimerStatusChange
	}

	a.setContent(a.dashboardScreen.Container)
	a.dashboardScreen.Refresh()
	a.dashboardScreen.StartTicker() // Start timer updates
}

// onTimerStatusChange updates the systray when timer status changes
func (a *App) onTimerStatusChange(running bool, projectName, taskName, activityName string, startTime time.Time) {
	if a.systrayManager == nil {
		return
	}
	if running {
		a.systrayManager.SetTimerRunning(projectName, taskName, activityName, startTime)
	} else {
		a.systrayManager.SetTimerStopped()
	}
}


func (a *App) showHistory() {
	a.currentScreen = "history"

	if a.historyScreen == nil {
		a.historyScreen = screens.NewHistoryScreen(a.apiClient, a.window)
		a.historyScreen.OnBack = a.showDashboard
	}

	a.setContent(a.historyScreen.Container)
	a.historyScreen.Refresh()
}

func (a *App) showGaps() {
	a.currentScreen = "gaps"

	// Initialize calendar service lazily
	if a.calendarService == nil {
		a.calendarService, _ = service.NewCalendarService()
	}

	if a.gapsScreen == nil {
		a.gapsScreen = screens.NewGapsScreen(a.apiClient, a.window)
		a.gapsScreen.OnBack = a.showDashboard
		a.gapsScreen.OnStartTimesheetFor = func(date time.Time, startTime time.Time, endTime time.Time) {
			// Show timesheet entry dialog with calendar panel
			a.showTimesheetDialogWithCalendar(date, startTime, endTime)
		}
		a.gapsScreen.OnEntryChanged = func() {
			// Refresh gaps view when an entry is changed
			if a.gapsScreen != nil {
				a.gapsScreen.Refresh()
			}
		}
	}

	a.setContent(a.gapsScreen.Container)
	a.gapsScreen.Refresh()
}

// showTimesheetDialogWithCalendar shows a timesheet entry dialog with calendar events panel
func (a *App) showTimesheetDialogWithCalendar(date time.Time, startTime time.Time, endTime time.Time) {
	// Create calendar panel
	calendarPanel := widgets.NewCalendarPanel(a.calendarService)
	calendarPanel.SetDate(date)

	// Variables to hold entry widget references
	var startDateEntry, startTimeEntry, endDateEntry, endTimeEntry *widget.Entry

	// Set up callback for when calendar event is selected
	calendarPanel.OnEventSelected = func(event models.CalendarEvent) {
		if startDateEntry != nil {
			startDateEntry.SetText(event.Start.Local().Format("2006-01-02"))
		}
		if startTimeEntry != nil {
			startTimeEntry.SetText(event.Start.Local().Format("15:04"))
		}
		if endDateEntry != nil {
			endDateEntry.SetText(event.End.Local().Format("2006-01-02"))
		}
		if endTimeEntry != nil {
			endTimeEntry.SetText(event.End.Local().Format("15:04"))
		}
	}

	// Set up callback to navigate to calendar settings
	calendarPanel.OnSetupClicked = func() {
		a.showCalendarSettings()
	}

	preFill := &widgets.EntryFormData{
		StartDate: date.Format("2006-01-02"),
		StartTime: startTime.Format("15:04"),
		EndDate:   date.Format("2006-01-02"),
		EndTime:   endTime.Format("15:04"),
	}

	widgets.ShowEntryFormDialog(&widgets.EntryFormConfig{
		Title:           "New Timesheet Entry",
		Window:          a.window,
		APIClient:       a.apiClient,
		ShowDescription: true,
		ShowDateTime:    true,
		ShowTask:        true,
		ShowActivity:    true,
		PreFill:         preFill,
		CalendarPanel:   calendarPanel.Container,
		OnCalendarTimeUpdate: func(sd, st, ed, et *widget.Entry) {
			startDateEntry = sd
			startTimeEntry = st
			endDateEntry = ed
			endTimeEntry = et
		},
		Buttons: []widgets.EntryFormButton{
			{
				Label: "Cancel",
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					close()
				},
			},
			{
				Label:      "Create Entry",
				Importance: widget.HighImportance,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					a.createTimesheetEntry(data, setError, close)
				},
			},
		},
	})
}

// createTimesheetEntry handles creating a new timesheet entry
func (a *App) createTimesheetEntry(data *widgets.EntryFormData, setError func(string), close func()) {
	if data.ProjectID == 0 {
		setError("Please select a project")
		return
	}
	if data.ActivityID == 0 {
		setError("Please select an activity")
		return
	}

	startParsed, err := time.ParseInLocation("2006-01-02 15:04",
		fmt.Sprintf("%s %s", data.StartDate, data.StartTime), time.Local)
	if err != nil {
		setError("Invalid start date/time")
		return
	}
	startParsed = startParsed.UTC()

	var endTimePtr *time.Time
	var duration float64
	if data.EndDate != "" && data.EndTime != "" {
		endParsed, err := time.ParseInLocation("2006-01-02 15:04",
			fmt.Sprintf("%s %s", data.EndDate, data.EndTime), time.Local)
		if err != nil {
			setError("Invalid end date/time")
			return
		}
		endParsed = endParsed.UTC()
		if endParsed.Before(startParsed) || endParsed.Equal(startParsed) {
			setError("End time must be after start time")
			return
		}
		endTimePtr = &endParsed
		duration = endParsed.Sub(startParsed).Hours()
	}

	entry := models.TimeEntry{
		ProjectID:   fmt.Sprintf("%d", data.ProjectID),
		ActivityID:  fmt.Sprintf("%d", data.ActivityID),
		Description: data.Description,
		StartTime:   startParsed,
		EndTime:     endTimePtr,
		Duration:    duration,
	}
	if data.TaskID > 0 {
		taskID := fmt.Sprintf("%d", data.TaskID)
		entry.TaskID = &taskID
	}

	close()

	// Create the entry asynchronously
	util.RunAsync(
		func() (bool, error) {
			return true, a.apiClient.CreateTimesheet(entry)
		},
		func(_ bool, err error) {
			if err != nil {
				// Show error in a dialog since the form is closed
				return
			}
			// Refresh gaps view after entry is created
			if a.gapsScreen != nil {
				a.gapsScreen.Refresh()
			}
		},
	)
}

func (a *App) showSettings() {
	a.currentScreen = "settings"

	if a.settingsScreen == nil {
		a.settingsScreen = screens.NewSettingsScreen(a.window)
		a.settingsScreen.OnBack = a.showDashboard
		a.settingsScreen.OnEditFavourites = a.showFavourites
		a.settingsScreen.OnCodeRepos = a.showCodeRepos
		a.settingsScreen.OnCalendarSettings = a.showCalendarSettings
		a.settingsScreen.OnLogout = a.handleLogout
	}

	a.setContent(a.settingsScreen.Container)
}

func (a *App) showCalendarSettings() {
	a.currentScreen = "calendarsettings"

	if a.calendarSettingsScreen == nil {
		a.calendarSettingsScreen = screens.NewCalendarSettingsScreen(a.window)
		a.calendarSettingsScreen.OnBack = a.showSettings
	}

	a.setContent(a.calendarSettingsScreen.Container)
	a.calendarSettingsScreen.Refresh()
}

func (a *App) showFavourites() {
	a.currentScreen = "favourites"

	if a.favouritesScreen == nil {
		a.favouritesScreen = screens.NewFavouritesScreen(a.apiClient, a.window)
		a.favouritesScreen.OnBack = a.showSettings
	}

	a.setContent(a.favouritesScreen.Container)
	a.favouritesScreen.Refresh()
}

func (a *App) showCodeRepos() {
	a.currentScreen = "coderepos"

	if a.codeReposScreen == nil {
		a.codeReposScreen = screens.NewCodeReposScreen(a.apiClient, a.window)
		a.codeReposScreen.OnBack = a.showSettings
	}

	a.setContent(a.codeReposScreen.Container)
	a.codeReposScreen.Refresh()
}

func (a *App) showOffice() {
	a.currentScreen = "office"

	if a.officeScreen == nil {
		a.officeScreen = screens.NewOfficeScreen(a.window)
		a.officeScreen.OnBack = func() {
			a.officeScreen.StopAnimation()
			a.showDashboard()
		}
	}

	a.setContent(a.officeScreen.Container)
	a.officeScreen.StartAnimation()
}

func (a *App) showAIAssistant() {
	a.currentScreen = "aiassistant"

	if a.aiAssistantScreen == nil {
		a.aiAssistantScreen = screens.NewAIAssistantScreen(a.apiClient, a.window)
		a.aiAssistantScreen.OnBack = a.showDashboard
		a.aiAssistantScreen.OnStartTimer = func(projectID, taskID, activityID int, projectName, taskName, activityName string) {
			// Start the timer and go back to dashboard
			a.startTimerFromAI(projectID, taskID, activityID, projectName)
		}
	}

	a.setContent(a.aiAssistantScreen.Container)
	a.aiAssistantScreen.Refresh()
}

func (a *App) startTimerFromAI(projectID, taskID, activityID int, projectName string) {
	entry := models.TimeEntry{
		ProjectID:   fmt.Sprintf("%d", projectID),
		ActivityID:  fmt.Sprintf("%d", activityID),
		Description: fmt.Sprintf("AI suggested: %s", projectName),
		StartTime:   time.Now().UTC(),
	}
	if taskID > 0 {
		taskIDStr := fmt.Sprintf("%d", taskID)
		entry.TaskID = &taskIDStr
	}

	util.RunAsync(
		func() (bool, error) {
			return true, a.apiClient.CreateTimesheet(entry)
		},
		func(_ bool, err error) {
			if err != nil {
				// Show error on dashboard
				a.showDashboard()
				return
			}
			a.showDashboard()
		},
	)
}

func (a *App) handleLogout() {
	// Clear API client
	a.apiClient = nil

	// Reset screens
	a.dashboardScreen = nil
	a.historyScreen = nil
	a.gapsScreen = nil
	a.settingsScreen = nil
	a.favouritesScreen = nil
	a.codeReposScreen = nil
	a.officeScreen = nil
	a.aiAssistantScreen = nil
	a.calendarSettingsScreen = nil
	a.loginScreen = nil

	// Show login
	a.showLogin()
}

func (a *App) setContent(content fyne.CanvasObject) {
	a.content.Objects = []fyne.CanvasObject{content}
	a.content.Refresh()
}
