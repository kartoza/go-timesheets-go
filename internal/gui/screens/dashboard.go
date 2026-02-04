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
	gapsButton      *widget.Button
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
	weeklyHours    float64
	monthlyHours   float64
	weeklyLabel    *canvas.Text
	timerStartTime time.Time
	ticker         *time.Ticker
	stopTicker     chan struct{}

	// Callbacks
	OnSettings          func()
	OnHistory           func()
	OnGaps              func()
	OnOffice            func()
	OnAIAssistant       func()
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
	s.restorePowState()
	s.build()
	return s
}

// restorePowState restores the POW enabled state from config
func (s *DashboardScreen) restorePowState() {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	if cfg.UI.PowEnabled {
		// Initialize POW capturer if not already provided
		if s.powCapture == nil {
			powCfg := pow.DefaultConfig()
			powCfg.Enabled = true
			capturer, err := pow.New(powCfg)
			if err != nil {
				return
			}
			s.powCapture = capturer
		} else if !s.powCapture.IsEnabled() {
			// Enable existing capturer
			s.powCapture.SetEnabled(true)
		}
	}
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

	s.weeklyLabel = canvas.NewText(fmt.Sprintf("Weekly: %.1fh", s.weeklyHours), color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF})
	s.weeklyLabel.TextSize = 14

	header := container.NewHBox(
		s.headerLabel,
		layout.NewSpacer(),
		s.weeklyLabel,
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

	// Gaps button
	s.gapsButton = widget.NewButton("📊 Gaps", func() {
		if s.OnGaps != nil {
			s.OnGaps()
		}
	})

	progressBox := widgets.ProgressBox(s.progressGauge, s.historyButton, s.gapsButton)

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
		colorHex := ""
		if fav != nil {
			colorHex = fav.Color
		}
		idx := i
		s.favButtons[i] = widgets.NewFavouriteButton(slotNum, name, projectName, false, isEmpty, colorHex, func() {
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

	// AI Assistant button
	aiButton := widget.NewButton("🤖 AI", func() {
		if s.OnAIAssistant != nil {
			s.OnAIAssistant()
		}
	})

	// POW status label
	s.powStatusLabel = canvas.NewText("", color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF})
	s.powStatusLabel.TextSize = 10
	s.powStatusLabel.Alignment = fyne.TextAlignCenter

	// Button row with minimum height to prevent overlap with favourites grid
	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		aiButton,
		s.powButton,
		s.officeButton,
		s.settingsButton,
		layout.NewSpacer(),
	)
	// Wrap in a container with minimum height
	buttonRowContainer := container.NewVBox(
		widget.NewSeparator(),
		buttonRow,
	)

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Main layout - use Border layout to ensure buttons stay at bottom
	mainContent := container.NewVBox(
		header,
		widget.NewSeparator(),
		dashboardRow,
		layout.NewSpacer(),
		favSection,
	)

	bottomSection := container.NewVBox(
		buttonRowContainer,
		container.NewCenter(s.powStatusLabel),
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
	)

	s.Container = container.NewBorder(
		nil,           // top
		bottomSection, // bottom - always visible
		nil,           // left
		nil,           // right
		mainContent,   // center - takes remaining space
	)

	// Update POW button state
	s.updatePowButton()
}

func (s *DashboardScreen) onStartStopClicked() {
	if s.activeTimer != nil {
		s.showStopTimerDialog()
	} else {
		s.ShowNewEntryDialog()
	}
}

func (s *DashboardScreen) showStopTimerDialog() {
	startTime, _ := s.activeTimer.GetFromTimeAsTime()

	desc := ""
	if s.activeTimer.Description != nil {
		desc = *s.activeTimer.Description
	}

	preFill := &widgets.EntryFormData{
		ProjectID:    s.activeTimer.ProjectID,
		ProjectName:  s.activeTimer.ProjectName,
		TaskID:       s.activeTimer.TaskID.Value,
		TaskName:     s.activeTimer.TaskName,
		ActivityID:   s.activeTimer.ActivityID,
		ActivityName: s.activeTimer.ActivityType,
		Description:  desc,
		StartDate:    startTime.Local().Format("2006-01-02"),
		StartTime:    startTime.Local().Format("15:04"),
		EndDate:      time.Now().Format("2006-01-02"),
		EndTime:      time.Now().Format("15:04"),
	}

	widgets.ShowEntryFormDialog(&widgets.EntryFormConfig{
		Title:           "Stop Timer",
		Window:          s.window,
		APIClient:       s.apiClient,
		ShowDescription: true,
		ShowDateTime:    true,
		ShowTask:        true,
		ShowActivity:    true,
		PreFill:         preFill,
		GitFetcher:      func(desc *widget.Entry) { s.fetchGitCommitsForDescription(desc) },
		Buttons: []widgets.EntryFormButton{
			{
				Label:       "Delete",
				Importance:  widget.DangerImportance,
				LeftAligned: true,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					close()
					s.confirmDeleteTimer()
				},
			},
			{
				Label: "Cancel",
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					close()
				},
			},
			{
				Label:      "Stop Timer",
				Importance: widget.HighImportance,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					s.doStopTimer(data, setError, close)
				},
			},
		},
	})
}

// doStopTimer handles the stop timer action from the unified dialog
func (s *DashboardScreen) doStopTimer(data *widgets.EntryFormData, setError func(string), close func()) {
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

	entry := models.TimeEntry{
		ProjectID:   fmt.Sprintf("%d", data.ProjectID),
		ActivityID:  fmt.Sprintf("%d", data.ActivityID),
		Description: data.Description,
		StartTime:   startParsed,
		EndTime:     &endParsed,
		Duration:    endParsed.Sub(startParsed).Hours(),
	}
	if data.TaskID > 0 {
		taskID := fmt.Sprintf("%d", data.TaskID)
		entry.TaskID = &taskID
	}

	close()
	s.setStatus("Stopping timer...")

	util.RunAsync(
		func() (bool, error) {
			return true, s.apiClient.UpdateTimesheet(
				fmt.Sprintf("%d", s.activeTimer.ID), entry)
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

// confirmDeleteTimer shows a delete confirmation for the active timer
func (s *DashboardScreen) confirmDeleteTimer() {
	dialog.ShowConfirm("Delete Timer",
		"Are you sure you want to delete this timer entry?\nThis action cannot be undone.",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			s.setStatus("Deleting timer...")

			util.RunAsync(
				func() (bool, error) {
					return true, s.apiClient.DeleteTimesheet(fmt.Sprintf("%d", s.activeTimer.ID))
				},
				func(_ bool, err error) {
					if err != nil {
						s.setStatus("Error: " + err.Error())
						return
					}
					s.activeTimer = nil
					s.updateTimerDisplay()
					s.setStatus("Timer deleted")
					s.Refresh()
				},
			)
		},
		s.window,
	)
}

// ShowNewEntryDialog shows a new timesheet entry dialog (replaces the separate timesheet screen)
func (s *DashboardScreen) ShowNewEntryDialog() {
	s.showNewEntryDialogWithPrefill(time.Now().Format("2006-01-02"), time.Now().Format("15:04"))
}

// ShowNewEntryDialogForDate shows a new timesheet entry dialog pre-filled for a specific date/time
func (s *DashboardScreen) ShowNewEntryDialogForDate(date time.Time, startTime time.Time) {
	s.showNewEntryDialogWithPrefill(date.Format("2006-01-02"), startTime.Format("15:04"))
}

func (s *DashboardScreen) showNewEntryDialogWithPrefill(dateStr, timeStr string) {
	preFill := &widgets.EntryFormData{
		StartDate: dateStr,
		StartTime: timeStr,
	}

	widgets.ShowEntryFormDialog(&widgets.EntryFormConfig{
		Title:           "New Timesheet Entry",
		Window:          s.window,
		APIClient:       s.apiClient,
		ShowDescription: true,
		ShowDateTime:    true,
		ShowTask:        true,
		ShowActivity:    true,
		PreFill:         preFill,
		Buttons: []widgets.EntryFormButton{
			{
				Label: "Cancel",
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					close()
				},
			},
			{
				Label:      "Start Timer",
				Importance: widget.HighImportance,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					s.doCreateEntry(data, setError, close)
				},
			},
		},
	})
}

// doCreateEntry handles the create entry action from the new entry dialog
func (s *DashboardScreen) doCreateEntry(data *widgets.EntryFormData, setError func(string), close func()) {
	if data.ProjectID == 0 {
		setError("Please select a project")
		return
	}
	if data.ActivityID == 0 {
		setError("Please select an activity")
		return
	}

	// Parse start date/time
	startParsed, err := time.ParseInLocation("2006-01-02 15:04",
		fmt.Sprintf("%s %s", data.StartDate, data.StartTime), time.Local)
	if err != nil {
		setError("Invalid start time (use HH:MM format)")
		return
	}
	startParsed = startParsed.UTC()

	// Parse end time if provided
	var endTimePtr *time.Time
	var duration float64
	if data.EndTime != "" {
		endParsed, err := time.ParseInLocation("2006-01-02 15:04",
			fmt.Sprintf("%s %s", data.EndDate, data.EndTime), time.Local)
		if err != nil {
			setError("Invalid end time (use HH:MM format)")
			return
		}
		endParsed = endParsed.UTC()
		duration = endParsed.Sub(startParsed).Hours()
		if duration <= 0 {
			setError("End time must be after start time")
			return
		}
		endTimePtr = &endParsed
	}

	close()
	s.setStatus("Creating entry...")

	util.RunAsync(
		func() (bool, error) {
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
			return true, s.apiClient.CreateTimesheet(entry)
		},
		func(_ bool, err error) {
			if err != nil {
				s.setStatus("Error creating entry: " + err.Error())
				return
			}
			s.setStatus("Timer started")
			s.Refresh()
		},
	)
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
	endTime := time.Now().UTC()

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

	// Stop POW session and generate video (works even if POW was disabled mid-session)
	entryID := s.activeTimer.ID
	if s.powCapture != nil {
		videoPath, err := s.powCapture.StopSession()
		if err != nil {
			// Log error but don't block timer stop
			fmt.Printf("Failed to stop POW session: %v\n", err)
		} else if videoPath != "" {
			// Save the POW video path mapping to the entry ID
			cfg, cfgErr := config.LoadConfig()
			if cfgErr == nil {
				store, storeErr := storage.New(cfg.GetStorageDir())
				if storeErr == nil {
					_ = store.SavePowVideoPath(entryID, videoPath)
				}
			}
		}
	}

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

		// Stop POW session and generate video (works even if POW was disabled mid-session)
		entryID := s.activeTimer.ID
		if s.powCapture != nil {
			videoPath, err := s.powCapture.StopSession()
			if err != nil {
				fmt.Printf("Failed to stop POW session: %v\n", err)
			} else if videoPath != "" {
				cfg, cfgErr := config.LoadConfig()
				if cfgErr == nil {
					store, storeErr := storage.New(cfg.GetStorageDir())
					if storeErr == nil {
						_ = store.SavePowVideoPath(entryID, videoPath)
					}
				}
			}
		}

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
				StartTime:  time.Now().UTC(),
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

			// Start POW session if enabled
			if s.powCapture != nil && s.powCapture.IsEnabled() {
				// We need to get the entry ID from the API after refresh
				// For now, start with placeholder - actual ID will come from active timer
				if startErr := s.powCapture.StartSession(0, fav.ProjectName, fav.TaskName); startErr != nil {
					fmt.Printf("Failed to start POW session: %v\n", startErr)
				}
			}

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
		colorHex := ""
		if fav != nil {
			colorHex = fav.Color
		}
		btn.Update(name, projectName, isRunning, isEmpty, colorHex)
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
			// Calculate today's, weekly, and monthly hours
			now := time.Now()
			todayStr := now.Format("2006-01-02")
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			// Calculate week start (Monday)
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7 // Sunday = 7
			}
			weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())

			var todayHours, weeklyHours, monthlyHours float64
			for _, e := range entries {
				fromTime, parseErr := e.GetFromTimeAsTime()
				if parseErr != nil {
					continue
				}
				if fromTime.Format("2006-01-02") == todayStr {
					todayHours += e.Hours
				}
				if fromTime.After(weekStart) || fromTime.Equal(weekStart) {
					weeklyHours += e.Hours
				}
				if fromTime.After(monthStart) || fromTime.Equal(monthStart) {
					monthlyHours += e.Hours
				}
			}
			s.todayHours = todayHours
			s.weeklyHours = weeklyHours
			s.monthlyHours = monthlyHours
			s.progressGauge.SetProgress(todayHours, 8.0)
			s.headerLabel.Text = fmt.Sprintf("Welcome, %s | Monthly: %.1fh", s.username, monthlyHours)
			s.headerLabel.Refresh()
			s.weeklyLabel.Text = fmt.Sprintf("Weekly: %.1fh", weeklyHours)
			s.weeklyLabel.Refresh()
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
		// Initialize POW capturer - start disabled, then toggle will enable it
		cfg := pow.DefaultConfig()
		cfg.Enabled = false
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
		// Save POW state to config
		s.savePowState(enabled)

		if enabled {
			s.setStatus("POW mode enabled - screenshots will be captured")
			// If a timer is already running, start POW session immediately
			if s.activeTimer != nil {
				if err := s.powCapture.StartSession(s.activeTimer.ID, s.activeTimer.ProjectName, s.activeTimer.TaskName); err != nil {
					fmt.Printf("Failed to start POW session for running timer: %v\n", err)
				}
			}
		} else {
			s.setStatus("POW mode disabled")
			// If disabling while timer is running, pause the session (keep screenshots for later)
			// Video will be generated when timer stops
			if s.activeTimer != nil {
				s.powCapture.PauseSession()
			}
		}
	} else {
		s.setStatus("Failed to toggle POW mode")
	}
}

// savePowState saves the POW enabled state to config
func (s *DashboardScreen) savePowState(enabled bool) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}
	cfg.UI.PowEnabled = enabled
	_ = config.SaveConfig(cfg)
}

// updatePowButton updates the POW button text based on current state
func (s *DashboardScreen) updatePowButton() {
	if s.powCapture != nil && s.powCapture.IsEnabled() {
		s.powButton.SetText("📸 POW: ON")
		s.powButton.Importance = widget.SuccessImportance // Green button
		s.powStatusLabel.Text = "Screenshots will be captured during timer sessions"
		s.powStatusLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // Green
	} else {
		s.powButton.SetText("📸 POW: OFF")
		s.powButton.Importance = widget.MediumImportance // Default button
		s.powStatusLabel.Text = ""
	}
	s.powButton.Refresh()
	s.powStatusLabel.Refresh()
}

// showOffice launches the office mode visualization
func (s *DashboardScreen) showOffice() {
	if s.OnOffice != nil {
		s.OnOffice()
	}
}
