package gui

import (
	_ "embed"

	"fyne.io/systray"
)

//go:embed icon.png
var iconData []byte

// SystrayManager handles the system tray icon and menu
type SystrayManager struct {
	app *App

	// Menu items
	mShow   *systray.MenuItem
	mStatus *systray.MenuItem
	mQuit   *systray.MenuItem

	// Channels for communication
	showChan chan struct{}
	quitChan chan struct{}
}

// NewSystrayManager creates a new system tray manager
func NewSystrayManager(app *App) *SystrayManager {
	return &SystrayManager{
		app:      app,
		showChan: make(chan struct{}, 1),
		quitChan: make(chan struct{}, 1),
	}
}

// ShowChan returns the channel that signals when to show the window
func (s *SystrayManager) ShowChan() <-chan struct{} {
	return s.showChan
}

// QuitChan returns the channel that signals when to quit
func (s *SystrayManager) QuitChan() <-chan struct{} {
	return s.quitChan
}

// OnReady is called when the systray is ready
func (s *SystrayManager) OnReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Kartoza Timesheets")
	systray.SetTooltip("Kartoza Timesheets - Time tracking made simple")

	// Set up left-click handler to show window
	systray.SetOnTapped(func() {
		select {
		case s.showChan <- struct{}{}:
		default:
		}
	})

	// Add menu items (shown on right-click)
	s.mShow = systray.AddMenuItem("Show Window", "Show the main window")
	systray.AddSeparator()
	s.mStatus = systray.AddMenuItem("No active timer", "Current timer status")
	s.mStatus.Disable()
	systray.AddSeparator()
	s.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks in a goroutine
	go s.handleClicks()
}

// OnExit is called when the systray is exiting
func (s *SystrayManager) OnExit() {
	// Cleanup if needed
}

// handleClicks handles menu item clicks
func (s *SystrayManager) handleClicks() {
	for {
		select {
		case <-s.mShow.ClickedCh:
			select {
			case s.showChan <- struct{}{}:
			default:
			}
		case <-s.mQuit.ClickedCh:
			select {
			case s.quitChan <- struct{}{}:
			default:
			}
			return
		}
	}
}

// UpdateStatus updates the status shown in the tray menu
func (s *SystrayManager) UpdateStatus(status string) {
	if s.mStatus != nil {
		s.mStatus.SetTitle(status)
	}
}

// SetTimerRunning updates the tray to indicate a timer is running
func (s *SystrayManager) SetTimerRunning(projectName string, taskName string) {
	if taskName != "" {
		s.UpdateStatus("⏱ " + projectName + " - " + taskName)
	} else {
		s.UpdateStatus("⏱ " + projectName)
	}
	systray.SetTooltip("Timer running: " + projectName)
}

// SetTimerStopped updates the tray to indicate no timer is running
func (s *SystrayManager) SetTimerStopped() {
	s.UpdateStatus("No active timer")
	systray.SetTooltip("Kartoza Timesheets - Time tracking made simple")
}
