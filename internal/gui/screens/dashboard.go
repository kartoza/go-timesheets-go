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
	powButton       *widget.Button
	officeButton    *widget.Button
	headerLabel     *canvas.Text
	statusLabel     *canvas.Text
	powStatusLabel  *canvas.Text

	// State
	activeTimer    *api.TimelogEntry
	favourites     *models.FavouriteAssociations
	todayHours     float64
	monthlyHours   float64
	timerStartTime time.Time
	ticker         *time.Ticker
	stopTicker     chan struct{}

	// Callbacks
	OnSettings          func()
	OnHistory           func()
	OnNewTimesheet      func()
	OnOffice            func()
	OnTimerStatusChange func(running bool, projectName, taskName, activityName string, startTime time.Time)
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

	// POW mode button
	s.powButton = widget.NewButton("📸 POW: OFF", s.togglePowMode)

	// Office mode button
	s.officeButton = widget.NewButton("🏢 Office", s.showOffice)

	// POW status label
	s.powStatusLabel = canvas.NewText("", color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF})
	s.powStatusLabel.TextSize = 10
	s.powStatusLabel.Alignment = fyne.TextAlignCenter

	// Button row
	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		s.powButton,
		s.officeButton,
		s.settingsButton,
		layout.NewSpacer(),
	)

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
		buttonRow,
		container.NewCenter(s.powStatusLabel),
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
	)

	// Update POW button state
	s.updatePowButton()
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
	descEntry.SetMinRowsVisible(5)

	// Check if there's a repo association for this project
	hasRepoAssoc := s.hasRepoAssociation(s.activeTimer.ProjectID)

	// Create "Fetch from Git" button
	fetchButton := widget.NewButton("📥 Fetch from Git", func() {
		s.fetchGitCommitsForDescription(descEntry)
	})
	if !hasRepoAssoc {
		fetchButton.Disable()
	}

	// Wrap the description entry with the fetch button
	descContainer := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), fetchButton),
		nil,
		nil,
		descEntry,
	)

	form := dialog.NewForm(
		"Stop Timer",
		"Stop",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Description", descContainer),
		},
		func(confirmed bool) {
			if confirmed {
				s.stopTimer(descEntry.Text)
			}
		},
		s.window,
	)
	form.Resize(fyne.NewSize(500, 300))
	form.Show()
}

// hasRepoAssociation checks if there's a code repo association for the given project
func (s *DashboardScreen) hasRepoAssociation(projectID int) bool {
	cfg, err := config.LoadConfig()
	if err != nil {
		return false
	}

	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		return false
	}

	associations, err := store.LoadCodeRepoAssociations()
	if err != nil || associations == nil {
		return false
	}

	assoc := associations.GetAssociationByProject(projectID)
	return assoc != nil && assoc.HasAssociation()
}

// fetchGitCommitsForDescription fetches git commits and populates the description field
func (s *DashboardScreen) fetchGitCommitsForDescription(descEntry *widget.Entry) {
	if s.activeTimer == nil {
		return
	}

	// Load code repo associations
	cfg, err := config.LoadConfig()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	associations, err := store.LoadCodeRepoAssociations()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	assoc := associations.GetAssociationByProject(s.activeTimer.ProjectID)
	if assoc == nil || !assoc.HasAssociation() {
		dialog.ShowInformation("No Repository", "No repository is linked to this project.\n\nGo to Settings > Code Repositories to link a repository.", s.window)
		return
	}

	// Get timer time range
	startTime := s.timerStartTime
	endTime := time.Now()

	// Determine if local or GitHub repo
	if api.IsLocalPath(assoc.RepoURL) {
		// Local repository
		s.fetchLocalGitCommits(assoc.RepoURL, startTime, endTime, descEntry)
	} else {
		// GitHub repository
		s.fetchGitHubCommits(assoc.RepoOwner, assoc.RepoName, startTime, endTime, descEntry)
	}
}

// fetchLocalGitCommits fetches commits from a local git repository
func (s *DashboardScreen) fetchLocalGitCommits(repoPath string, startTime, endTime time.Time, descEntry *widget.Entry) {
	repoPath = api.ExpandPath(repoPath)

	if !api.IsGitRepository(repoPath) {
		dialog.ShowError(fmt.Errorf("not a git repository: %s", repoPath), s.window)
		return
	}

	util.RunAsync(
		func() (string, error) {
			localGitClient := api.NewLocalGitClient()
			commits, err := localGitClient.GetCommitsInTimeRange(repoPath, startTime, endTime, "")
			if err != nil {
				return "", err
			}
			if len(commits) == 0 {
				return "", nil
			}
			return api.FormatLocalCommitsAsDescription(commits), nil
		},
		func(description string, err error) {
			if err != nil {
				dialog.ShowError(err, s.window)
				return
			}
			if description == "" {
				dialog.ShowInformation("No Commits", "No commits found in the timer time range.", s.window)
				return
			}
			// Append to existing description or set new
			existing := descEntry.Text
			if existing != "" {
				descEntry.SetText(existing + "\n\n" + description)
			} else {
				descEntry.SetText(description)
			}
		},
	)
}

// fetchGitHubCommits fetches commits from a GitHub repository
func (s *DashboardScreen) fetchGitHubCommits(owner, repo string, startTime, endTime time.Time, descEntry *widget.Entry) {
	cfg, err := config.LoadConfig()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load config: %w", err), s.window)
		return
	}

	githubToken := cfg.GetGitHubToken()
	if githubToken == "" {
		dialog.ShowInformation("GitHub Token Required",
			"A GitHub token is required to fetch commits.\n\nSet GITHUB_TOKEN environment variable or configure it in settings.",
			s.window)
		return
	}

	util.RunAsync(
		func() (string, error) {
			githubClient := api.NewGitHubClient(githubToken)
			commits, err := githubClient.GetCommitsInTimeRange(owner, repo, startTime, endTime, "")
			if err != nil {
				return "", err
			}
			if len(commits) == 0 {
				return "", nil
			}
			return api.FormatCommitsAsDescription(commits), nil
		},
		func(description string, err error) {
			if err != nil {
				dialog.ShowError(err, s.window)
				return
			}
			if description == "" {
				dialog.ShowInformation("No Commits", "No commits found in the timer time range.", s.window)
				return
			}
			// Append to existing description or set new
			existing := descEntry.Text
			if existing != "" {
				descEntry.SetText(existing + "\n\n" + description)
			} else {
				descEntry.SetText(description)
			}
		},
	)
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
			s.OnTimerStatusChange(true, s.activeTimer.ProjectName, s.activeTimer.TaskName, s.activeTimer.ActivityType, s.timerStartTime)
		}
	} else {
		s.timerDisplay.SetTime(0, 0)
		s.timerDisplay.SetActive(false, "", "")
		s.timerDisplay.StopBlinking()
		s.startStopButton.SetText("▶ Start Timer")
		// Notify systray of timer status
		if s.OnTimerStatusChange != nil {
			s.OnTimerStatusChange(false, "", "", "", time.Time{})
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
					// Use fyne.Do for thread-safe UI updates
					fyne.Do(func() {
						s.timerDisplay.SetTime(hours, minutes)
					})
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

// togglePowMode toggles POW (Proof of Work) screenshot capture mode
func (s *DashboardScreen) togglePowMode() {
	if s.powCapture == nil {
		// Initialize POW capturer if not exists
		cfg := pow.DefaultConfig()
		cfg.Enabled = true
		capturer, err := pow.New(cfg)
		if err != nil {
			s.setStatus("POW Error: " + err.Error())
			return
		}
		s.powCapture = capturer
	}

	enabled, ok := s.powCapture.Toggle()
	if ok {
		s.updatePowButton()
		if enabled {
			s.setStatus("POW mode enabled - screenshots will be captured")
		} else {
			s.setStatus("POW mode disabled")
		}
	} else {
		s.setStatus("Failed to toggle POW mode")
	}
}

// updatePowButton updates the POW button text based on current state
func (s *DashboardScreen) updatePowButton() {
	if s.powCapture != nil && s.powCapture.IsEnabled() {
		s.powButton.SetText("📸 POW: ON")
		s.powStatusLabel.Text = "Screenshots will be captured during timer sessions"
		s.powStatusLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // Green
	} else {
		s.powButton.SetText("📸 POW: OFF")
		s.powStatusLabel.Text = ""
	}
	s.powStatusLabel.Refresh()
}

// showOffice launches the office mode visualization
func (s *DashboardScreen) showOffice() {
	if s.OnOffice != nil {
		s.OnOffice()
	}
}
