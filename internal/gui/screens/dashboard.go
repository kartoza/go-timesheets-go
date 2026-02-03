package screens

import (
	"fmt"
	"image/color"
	"sort"
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
	monthlyHours   float64
	timerStartTime time.Time
	ticker         *time.Ticker
	stopTicker     chan struct{}

	// Callbacks
	OnSettings          func()
	OnHistory           func()
	OnGaps              func()
	OnNewTimesheet      func()
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

	// Button row
	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		aiButton,
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
	// Local state for the dialog
	var (
		selectedProject  *api.ProjectListItem
		selectedTask     *api.TaskListItem
		selectedActivity *api.ActivityListItem
		projects         []api.ProjectListItem
		tasks            []api.TaskListItem
		activities       []api.ActivityListItem
		errorLabel       *canvas.Text
	)

	// Pre-select from active timer
	selectedProject = &api.ProjectListItem{
		ID:    s.activeTimer.ProjectID,
		Label: s.activeTimer.ProjectName,
	}
	if s.activeTimer.TaskID.Value > 0 {
		selectedTask = &api.TaskListItem{
			ID:    s.activeTimer.TaskID.Value,
			Label: s.activeTimer.TaskName,
		}
	}
	selectedActivity = &api.ActivityListItem{
		ID:    s.activeTimer.ActivityID,
		Label: s.activeTimer.ActivityType,
	}

	// Title
	title := canvas.NewText("Stop Timer", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Project select
	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	projectSelect.SetSelected(&widgets.SelectItem{
		ID:    s.activeTimer.ProjectID,
		Label: s.activeTimer.ProjectName,
	})

	// Task select
	taskSelect := widgets.NewBoundedSelect("Select task...", []string{}, s.window)

	// Activity select
	activitySelect := widgets.NewBoundedSelect("Select activity...", []string{}, s.window)

	// Wire up project select
	projectSelect.OnChanged = func(item *widgets.SelectItem) {
		if item != nil {
			selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
			// Load tasks for the new project
			util.RunAsync(
				func() ([]api.TaskListItem, error) {
					return s.apiClient.GetTasks(fmt.Sprintf("%d", item.ID))
				},
				func(result []api.TaskListItem, err error) {
					if err != nil {
						return
					}
					tasks = result
					options := make([]string, len(tasks))
					for i, t := range tasks {
						options[i] = t.Label
					}
					taskSelect.SetOptions(options)
				},
			)
		} else {
			selectedProject = nil
			taskSelect.SetOptions([]string{})
		}
	}
	projectSelect.OnSearch = func(query string) {
		util.RunAsync(
			func() ([]api.ProjectListItem, error) {
				return s.apiClient.GetProjects(query)
			},
			func(result []api.ProjectListItem, err error) {
				if err != nil {
					return
				}
				projects = result
				items := make([]widgets.SelectItem, len(projects))
				for i, p := range projects {
					items[i] = widgets.SelectItem{ID: p.ID, Label: p.Label}
				}
				projectSelect.SetItems(items)
			},
		)
	}

	// Wire up task select
	taskSelect.OnChanged = func(selected string) {
		for _, t := range tasks {
			if t.Label == selected {
				selectedTask = &t
				break
			}
		}
	}

	// Wire up activity select
	activitySelect.OnChanged = func(selected string) {
		for _, a := range activities {
			if a.Label == selected {
				selectedActivity = &a
				break
			}
		}
	}

	// Description entry (full width)
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description (optional)")
	if s.activeTimer.Description != nil {
		descEntry.SetText(*s.activeTimer.Description)
	}
	descEntry.SetMinRowsVisible(3)

	// Fetch from Git button
	hasRepoAssoc := s.hasRepoAssociation(s.activeTimer.ProjectID)
	fetchButton := widget.NewButton("Fetch from Git", func() {
		s.fetchGitCommitsForDescription(descEntry)
	})
	if !hasRepoAssoc {
		fetchButton.Disable()
	}

	// Date/time fields - parse start time from active timer
	startTime, _ := s.activeTimer.GetFromTimeAsTime()

	startDateEntry := widget.NewEntry()
	startDateEntry.SetPlaceHolder("YYYY-MM-DD")
	startDateEntry.SetText(startTime.Local().Format("2006-01-02"))

	startTimeEntry := widget.NewEntry()
	startTimeEntry.SetPlaceHolder("HH:MM")
	startTimeEntry.SetText(startTime.Local().Format("15:04"))

	endDateEntry := widget.NewEntry()
	endDateEntry.SetPlaceHolder("YYYY-MM-DD")
	endDateEntry.SetText(time.Now().Format("2006-01-02"))

	endTimeEntry := widget.NewEntry()
	endTimeEntry.SetPlaceHolder("HH:MM")
	endTimeEntry.SetText(time.Now().Format("15:04"))

	// Error label
	errorLabel = canvas.NewText("", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF})
	errorLabel.TextSize = 12
	errorLabel.Alignment = fyne.TextAlignCenter

	// Date/time row: Start Date + Start Time | End Date + End Time
	dateTimeRow := container.NewGridWithColumns(4,
		container.NewVBox(widget.NewLabel("Start Date"), startDateEntry),
		container.NewVBox(widget.NewLabel("Start Time"), startTimeEntry),
		container.NewVBox(widget.NewLabel("End Date"), endDateEntry),
		container.NewVBox(widget.NewLabel("End Time"), endTimeEntry),
	)

	// Buttons - declare var so we can reference in closures
	var d *widget.PopUp

	stopButton := widget.NewButton("Stop Timer", func() {
		// Validation
		if selectedProject == nil {
			errorLabel.Text = "Please select a project"
			errorLabel.Refresh()
			return
		}
		if selectedActivity == nil {
			errorLabel.Text = "Please select an activity"
			errorLabel.Refresh()
			return
		}

		// Parse start date/time
		startParsed, err := time.ParseInLocation("2006-01-02 15:04",
			fmt.Sprintf("%s %s", startDateEntry.Text, startTimeEntry.Text), time.Local)
		if err != nil {
			errorLabel.Text = "Invalid start date/time"
			errorLabel.Refresh()
			return
		}
		startParsed = startParsed.UTC()

		// Parse end date/time
		endParsed, err := time.ParseInLocation("2006-01-02 15:04",
			fmt.Sprintf("%s %s", endDateEntry.Text, endTimeEntry.Text), time.Local)
		if err != nil {
			errorLabel.Text = "Invalid end date/time"
			errorLabel.Refresh()
			return
		}
		endParsed = endParsed.UTC()

		if endParsed.Before(startParsed) || endParsed.Equal(startParsed) {
			errorLabel.Text = "End time must be after start time"
			errorLabel.Refresh()
			return
		}

		errorLabel.Text = ""
		errorLabel.Refresh()

		// Build the TimeEntry for UpdateTimesheet
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", selectedProject.ID),
			ActivityID:  fmt.Sprintf("%d", selectedActivity.ID),
			Description: descEntry.Text,
			StartTime:   startParsed,
			EndTime:     &endParsed,
			Duration:    endParsed.Sub(startParsed).Hours(),
		}
		if selectedTask != nil {
			taskID := fmt.Sprintf("%d", selectedTask.ID)
			entry.TaskID = &taskID
		}

		if d != nil {
			d.Hide()
		}
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
	})
	stopButton.Importance = widget.HighImportance

	cancelButton := widget.NewButton("Cancel", func() {
		if d != nil {
			d.Hide()
		}
	})

	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		cancelButton,
		stopButton,
	)

	// Description container with fetch button
	descContainer := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), fetchButton),
		nil,
		nil,
		descEntry,
	)

	// Form layout matching timesheet creator style
	form := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		widget.NewLabel("Project *"),
		projectSelect,
		widget.NewLabel("Task"),
		taskSelect.Select,
		widget.NewLabel("Activity *"),
		activitySelect.Select,
		widget.NewLabel("Description"),
		descContainer,
		dateTimeRow,
		layout.NewSpacer(),
		errorLabel,
		widget.NewSeparator(),
		buttonRow,
	)

	// Container with border (same style as timesheet creator)
	formBorder := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	formBorder.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	formBorder.StrokeWidth = 2
	formBorder.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(form)),
	)

	// Show as a popup overlay - pass formContainer directly so the dark
	// background fills the entire popup with no white space around it
	d = widget.NewModalPopUp(formContainer, s.window.Canvas())
	d.Resize(fyne.NewSize(550, 600))
	d.Show()

	// Load tasks for current project
	util.RunAsync(
		func() ([]api.TaskListItem, error) {
			return s.apiClient.GetTasks(fmt.Sprintf("%d", s.activeTimer.ProjectID))
		},
		func(result []api.TaskListItem, err error) {
			if err != nil {
				return
			}
			tasks = result
			options := make([]string, len(tasks))
			for i, t := range tasks {
				options[i] = t.Label
			}
			taskSelect.SetOptions(options)
			if selectedTask != nil {
				taskSelect.SetSelected(selectedTask.Label)
			}
		},
	)

	// Load activities
	util.RunAsync(
		func() ([]api.ActivityListItem, error) {
			return s.apiClient.GetActivities()
		},
		func(result []api.ActivityListItem, err error) {
			if err != nil {
				return
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].Label < result[j].Label
			})
			activities = result
			options := make([]string, len(activities))
			for i, a := range activities {
				options[i] = a.Label
			}
			activitySelect.SetOptions(options)
			if selectedActivity != nil {
				activitySelect.SetSelected(selectedActivity.Label)
			}
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
