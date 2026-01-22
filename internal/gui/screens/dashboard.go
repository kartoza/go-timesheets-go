package screens

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/gui/widgets"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// DashboardScreen represents the main dashboard screen
type DashboardScreen struct {
	Container fyne.CanvasObject

	apiClient  *api.Client
	username   string
	window     fyne.Window
	powCapture *pow.Capturer

	// Widgets
	timerDisplay    *widgets.TimerDisplay
	progressGauge   *widgets.ProgressGauge
	favButtons      []*widgets.FavouriteButton
	startStopButton *widget.Button
	historyButton   *widget.Button
	settingsButton  *widget.Button
	headerLabel     *canvas.Text
	statusLabel     *canvas.Text

	// State
	activeTimer    *api.TimelogEntry
	favourites     *models.FavouriteAssociations
	todayHours     float64
	monthlyHours   float64
	timerStartTime time.Time
	ticker         *time.Ticker
	stopTicker     chan struct{}

	// Callbacks
	OnSettings        func()
	OnHistory         func()
	OnNewTimesheet    func()
	OnTimerStatusChange func(running bool, projectName, taskName string)
}

// NewDashboardScreen creates a new dashboard screen
func NewDashboardScreen(apiClient *api.Client, window fyne.Window, powCapture *pow.Capturer) *DashboardScreen {
	s := &DashboardScreen{
		apiClient:  apiClient,
		username:   apiClient.GetUsername(),
		window:     window,
		powCapture: powCapture,
		stopTicker: make(chan struct{}),
	}
	s.loadFavourites()
	s.build()
	return s
}

func (s *DashboardScreen) loadFavourites() {
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)
	s.favourites, _ = store.LoadFavouriteAssociations()
	if s.favourites == nil {
		s.favourites = models.NewFavouriteAssociations()
	}
}

func (s *DashboardScreen) build() {
	// Header
	s.headerLabel = canvas.NewText(fmt.Sprintf("Welcome, %s", s.username), color.White)
	s.headerLabel.TextSize = 18
	s.headerLabel.TextStyle = fyne.TextStyle{Bold: true}

	monthlyLabel := canvas.NewText(fmt.Sprintf("Monthly: %.1fh", s.monthlyHours), color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF})
	monthlyLabel.TextSize = 14

	header := container.NewHBox(
		s.headerLabel,
		layout.NewSpacer(),
		monthlyLabel,
	)

	// Timer display
	s.timerDisplay = widgets.NewTimerDisplay()

	// Start/Stop button
	s.startStopButton = widget.NewButton("▶ Start Timer", s.onStartStopClicked)
	s.startStopButton.Importance = widget.HighImportance

	timerBox := widgets.TimerBox(s.timerDisplay, s.startStopButton)

	// Progress gauge
	s.progressGauge = widgets.NewProgressGauge(s.todayHours, 8.0)

	// History button
	s.historyButton = widget.NewButton("📋 History", func() {
		if s.OnHistory != nil {
			s.OnHistory()
		}
	})

	progressBox := widgets.ProgressBox(s.progressGauge, s.historyButton)

	// Dashboard row (timer left, progress right)
	dashboardRow := container.NewGridWithColumns(2, timerBox, progressBox)

	// Favourites grid
	s.favButtons = make([]*widgets.FavouriteButton, 9)
	for i := 0; i < 9; i++ {
		slotNum := i + 1
		fav := s.favourites.GetAssociation(slotNum)
		name := ""
		projectName := ""
		isEmpty := true
		if fav != nil && fav.ProjectID > 0 {
			name = fav.Name
			if name == "" {
				name = fav.ProjectName
			}
			projectName = fav.ProjectName
			isEmpty = false
		}
		idx := i
		s.favButtons[i] = widgets.NewFavouriteButton(slotNum, name, projectName, false, isEmpty, func() {
			s.onFavouriteClicked(idx)
		})
	}

	favGrid := widgets.FavouritesGrid(s.favButtons)

	// Favourites title
	favTitle := canvas.NewText("Quick Start Favourites (1-9)", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	favTitle.TextSize = 16
	favTitle.TextStyle = fyne.TextStyle{Bold: true}
	favTitle.Alignment = fyne.TextAlignCenter

	favSection := container.NewVBox(
		container.NewCenter(favTitle),
		favGrid,
	)

	// Settings button
	s.settingsButton = widget.NewButton("⚙ Settings", func() {
		if s.OnSettings != nil {
			s.OnSettings()
		}
	})

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Main layout
	s.Container = container.NewVBox(
		header,
		widget.NewSeparator(),
		dashboardRow,
		layout.NewSpacer(),
		favSection,
		layout.NewSpacer(),
		container.NewCenter(s.settingsButton),
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
	)
}

func (s *DashboardScreen) onStartStopClicked() {
	if s.activeTimer != nil {
		s.showStopTimerDialog()
	} else {
		if s.OnNewTimesheet != nil {
			s.OnNewTimesheet()
		}
	}
}

func (s *DashboardScreen) showStopTimerDialog() {
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description (optional)")
	if s.activeTimer.Description != nil {
		descEntry.SetText(*s.activeTimer.Description)
	}
	descEntry.SetMinRowsVisible(3)

	form := dialog.NewForm(
		"Stop Timer",
		"Stop",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Description", descEntry),
		},
		func(confirmed bool) {
			if confirmed {
				s.stopTimer(descEntry.Text)
			}
		},
		s.window,
	)
	form.Resize(fyne.NewSize(400, 200))
	form.Show()
}

func (s *DashboardScreen) stopTimer(description string) {
	if s.activeTimer == nil {
		return
	}

	s.setStatus("Stopping timer...")

	util.RunAsync(
		func() (bool, error) {
			entry := *s.activeTimer
			if description != "" {
				entry.Description = &description
			}
			return true, s.apiClient.StopTimesheetWithDescription(&entry, description)
		},
		func(_ bool, err error) {
			if err != nil {
				s.setStatus("Error: " + err.Error())
				return
			}
			s.activeTimer = nil
			s.updateTimerDisplay()
			s.setStatus("Timer stopped")
			s.Refresh()
		},
	)
}

func (s *DashboardScreen) onFavouriteClicked(index int) {
	fav := s.favourites.GetAssociation(index + 1)
	if fav == nil || fav.ProjectID == 0 {
		s.setStatus("Configure this favourite in Settings")
		return
	}

	// Check if this favourite is already running
	if s.activeTimer != nil && s.activeTimer.ProjectID == fav.ProjectID {
		s.setStatus("This timer is already running")
		return
	}

	// If a timer is running, stop it first
	if s.activeTimer != nil {
		s.setStatus("Stopping current timer...")
		util.RunAsync(
			func() (bool, error) {
				return true, s.apiClient.StopTimesheet(s.activeTimer)
			},
			func(_ bool, err error) {
				if err != nil {
					s.setStatus("Error stopping timer: " + err.Error())
					return
				}
				s.startFavouriteTimer(fav)
			},
		)
	} else {
		s.startFavouriteTimer(fav)
	}
}

func (s *DashboardScreen) startFavouriteTimer(fav *models.FavouriteAssociation) {
	s.setStatus("Starting timer...")

	util.RunAsync(
		func() (bool, error) {
			entry := models.TimeEntry{
				ProjectID:  fmt.Sprintf("%d", fav.ProjectID),
				ActivityID: fmt.Sprintf("%d", fav.ActivityID),
				StartTime:  time.Now(),
			}
			if fav.TaskID > 0 {
				taskID := fmt.Sprintf("%d", fav.TaskID)
				entry.TaskID = &taskID
			}
			err := s.apiClient.CreateTimesheet(entry)
			return true, err
		},
		func(_ bool, err error) {
			if err != nil {
				s.setStatus("Error: " + err.Error())
				return
			}
			s.setStatus(fmt.Sprintf("Started: %s", fav.Name))
			s.Refresh()
		},
	)
}

func (s *DashboardScreen) updateTimerDisplay() {
	if s.activeTimer != nil {
		elapsed := time.Since(s.timerStartTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		s.timerDisplay.SetTime(hours, minutes)
		s.timerDisplay.SetActive(true, s.activeTimer.ProjectName, s.activeTimer.TaskName)
		s.timerDisplay.StartBlinking()
		s.startStopButton.SetText("■ Stop Timer")
		// Notify systray of timer status
		if s.OnTimerStatusChange != nil {
			s.OnTimerStatusChange(true, s.activeTimer.ProjectName, s.activeTimer.TaskName)
		}
	} else {
		s.timerDisplay.SetTime(0, 0)
		s.timerDisplay.SetActive(false, "", "")
		s.timerDisplay.StopBlinking()
		s.startStopButton.SetText("▶ Start Timer")
		// Notify systray of timer status
		if s.OnTimerStatusChange != nil {
			s.OnTimerStatusChange(false, "", "")
		}
	}

	// Update favourite buttons to show running state
	for i, btn := range s.favButtons {
		fav := s.favourites.GetAssociation(i + 1)
		isRunning := false
		if s.activeTimer != nil && fav != nil && s.activeTimer.ProjectID == fav.ProjectID {
			isRunning = true
		}
		name := ""
		projectName := ""
		isEmpty := true
		if fav != nil && fav.ProjectID > 0 {
			name = fav.Name
			if name == "" {
				name = fav.ProjectName
			}
			projectName = fav.ProjectName
			isEmpty = false
		}
		btn.Update(name, projectName, isRunning, isEmpty)
	}
}

func (s *DashboardScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// Refresh reloads data from the API
func (s *DashboardScreen) Refresh() {
	// Load active timer
	util.RunAsync(
		func() (*api.TimelogEntry, error) {
			return s.apiClient.GetActiveTimesheet()
		},
		func(entry *api.TimelogEntry, err error) {
			if err != nil {
				s.setStatus("Error loading timer: " + err.Error())
				return
			}
			s.activeTimer = entry
			if entry != nil {
				startTime, parseErr := entry.GetFromTimeAsTime()
				if parseErr == nil {
					s.timerStartTime = startTime
				}
			}
			s.updateTimerDisplay()
		},
	)

	// Load timelogs to calculate hours
	util.RunAsync(
		func() ([]api.TimelogEntry, error) {
			return s.apiClient.GetTimelogs()
		},
		func(entries []api.TimelogEntry, err error) {
			if err != nil {
				return
			}
			// Calculate today's and monthly hours
			now := time.Now()
			todayStr := now.Format("2006-01-02")
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

			var todayHours, monthlyHours float64
			for _, e := range entries {
				fromTime, parseErr := e.GetFromTimeAsTime()
				if parseErr != nil {
					continue
				}
				if fromTime.Format("2006-01-02") == todayStr {
					todayHours += e.Hours
				}
				if fromTime.After(monthStart) || fromTime.Equal(monthStart) {
					monthlyHours += e.Hours
				}
			}
			s.todayHours = todayHours
			s.monthlyHours = monthlyHours
			s.progressGauge.SetProgress(todayHours, 8.0)
			s.headerLabel.Text = fmt.Sprintf("Welcome, %s | Monthly: %.1fh", s.username, monthlyHours)
			s.headerLabel.Refresh()
		},
	)

	// Reload favourites
	s.loadFavourites()
	s.updateTimerDisplay()
}

// StartTicker starts the timer update ticker
func (s *DashboardScreen) StartTicker() {
	if s.ticker != nil {
		return
	}
	s.ticker = time.NewTicker(time.Second)
	go func() {
		for {
			select {
			case <-s.ticker.C:
				if s.activeTimer != nil {
					elapsed := time.Since(s.timerStartTime)
					hours := int(elapsed.Hours())
					minutes := int(elapsed.Minutes()) % 60
					s.timerDisplay.SetTime(hours, minutes)
				}
			case <-s.stopTicker:
				return
			}
		}
	}()
}

// StopTicker stops the timer update ticker
func (s *DashboardScreen) StopTicker() {
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}
	select {
	case s.stopTicker <- struct{}{}:
	default:
	}
}
