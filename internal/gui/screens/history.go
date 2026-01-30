package screens

import (
	"fmt"
	"image/color"
	"os/exec"
	"runtime"
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
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// HistoryScreen represents the history screen
type HistoryScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	store     *storage.Storage

	// Widgets
	entryList    *widget.List
	backButton   *widget.Button
	submitButton *widget.Button
	statusLabel  *canvas.Text

	// Data
	entries      []api.TimelogEntry
	powVideoMap  map[int]string // entry ID -> video path

	// Callbacks
	OnBack func()
}

// NewHistoryScreen creates a new history screen
func NewHistoryScreen(apiClient *api.Client, window fyne.Window) *HistoryScreen {
	s := &HistoryScreen{
		apiClient:   apiClient,
		window:      window,
		entries:     []api.TimelogEntry{},
		powVideoMap: make(map[int]string),
	}

	// Initialize storage for POW video lookup
	if cfg, err := config.LoadConfig(); err == nil {
		if store, err := storage.New(cfg.GetStorageDir()); err == nil {
			s.store = store
		}
	}

	s.build()
	return s
}

func (s *HistoryScreen) build() {
	// Title
	title := canvas.NewText("Timesheet History", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Entry list
	s.entryList = widget.NewList(
		func() int { return len(s.entries) },
		func() fyne.CanvasObject {
			return s.createEntryCard()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			s.updateEntryCard(id, obj)
		},
	)
	s.entryList.OnSelected = func(id widget.ListItemID) {
		if int(id) < len(s.entries) {
			s.showEntryDetail(int(id))
		}
		s.entryList.UnselectAll()
	}

	// Buttons
	s.backButton = widget.NewButton("← Back", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	s.submitButton = widget.NewButton("📤 Submit All Pending", s.submitPendingEntries)
	s.submitButton.Importance = widget.HighImportance

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	buttonRow := container.NewHBox(
		s.backButton,
		layout.NewSpacer(),
		s.submitButton,
	)

	// Main layout
	s.Container = container.NewBorder(
		container.NewVBox(
			container.NewCenter(title),
			widget.NewSeparator(),
			buttonRow,
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewCenter(s.statusLabel),
		),
		nil,
		nil,
		s.entryList,
	)
}

func (s *HistoryScreen) createEntryCard() fyne.CanvasObject {
	projectLabel := widget.NewLabel("Project")
	projectLabel.TextStyle = fyne.TextStyle{Bold: true}

	taskLabel := widget.NewLabel("Task")
	dateLabel := widget.NewLabel("Date")
	hoursLabel := widget.NewLabel("Hours")
	hoursLabel.TextStyle = fyne.TextStyle{Bold: true}
	statusLabel := widget.NewLabel("Status")

	descLabel := widget.NewLabel("Description")
	descLabel.TextStyle = fyne.TextStyle{Italic: true}
	descLabel.Wrapping = fyne.TextWrapWord

	return container.NewVBox(
		container.NewHBox(
			projectLabel,
			layout.NewSpacer(),
			dateLabel,
			hoursLabel,
		),
		container.NewHBox(
			taskLabel,
			layout.NewSpacer(),
			statusLabel,
		),
		descLabel,
		widget.NewSeparator(),
	)
}

func (s *HistoryScreen) updateEntryCard(id widget.ListItemID, obj fyne.CanvasObject) {
	if int(id) >= len(s.entries) {
		return
	}

	entry := s.entries[id]
	vbox := obj.(*fyne.Container)

	row1 := vbox.Objects[0].(*fyne.Container)
	row2 := vbox.Objects[1].(*fyne.Container)
	descLabel := vbox.Objects[2].(*widget.Label)

	// Row 1: Project, Date, Hours
	row1.Objects[0].(*widget.Label).SetText(entry.ProjectName)

	fromTime, err := entry.GetFromTimeAsTime()
	if err == nil {
		row1.Objects[2].(*widget.Label).SetText(fromTime.Local().Format("2006-01-02"))
	} else {
		row1.Objects[2].(*widget.Label).SetText(entry.FromTime)
	}
	row1.Objects[3].(*widget.Label).SetText(fmt.Sprintf("%.1fh", entry.Hours))

	// Row 2: Task, Status (with icons)
	taskText := entry.TaskName
	if taskText == "" {
		taskText = "(No task)"
	}
	row2.Objects[0].(*widget.Label).SetText(taskText)

	// Build status text with icons
	statusText := ""

	// Add POW icon if has video
	videoPath, hasPOW := s.powVideoMap[entry.ID]
	if hasPOW && videoPath != "" {
		statusText = "🎬 "
	}

	// Add status
	if entry.ToTime == "" {
		statusText += "🟢 Running"
	} else if entry.Submitted {
		statusText += "✓ Submitted"
	} else {
		statusText += "○ Pending"
	}
	row2.Objects[2].(*widget.Label).SetText(statusText)

	// Row 3: Description
	desc := "(No description)"
	if entry.Description != nil && *entry.Description != "" {
		desc = *entry.Description
	}
	descLabel.SetText(desc)
}

func (s *HistoryScreen) showEntryDetail(index int) {
	if index >= len(s.entries) {
		return
	}

	entry := s.entries[index]
	isEditable := !entry.Submitted

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

	// Pre-select from entry
	selectedProject = &api.ProjectListItem{
		ID:    entry.ProjectID,
		Label: entry.ProjectName,
	}
	if entry.TaskID.Value > 0 {
		selectedTask = &api.TaskListItem{
			ID:    entry.TaskID.Value,
			Label: entry.TaskName,
		}
	}
	selectedActivity = &api.ActivityListItem{
		ID:    entry.ActivityID,
		Label: entry.ActivityType,
	}

	// Title
	titleText := "Entry Details"
	if entry.Submitted {
		titleText = "Entry Details (Submitted)"
	}
	title := canvas.NewText(titleText, color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Project select
	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	projectSelect.SetSelected(&widgets.SelectItem{
		ID:    entry.ProjectID,
		Label: entry.ProjectName,
	})

	// Task select
	taskSelect := widgets.NewBoundedSelect("Select task...", []string{}, s.window)

	// Activity select
	activitySelect := widgets.NewBoundedSelect("Select activity...", []string{}, s.window)

	// Wire up project select
	projectSelect.OnChanged = func(item *widgets.SelectItem) {
		if item != nil {
			selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
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

	// Description entry
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description (optional)")
	if entry.Description != nil && *entry.Description != "" {
		descEntry.SetText(*entry.Description)
	}
	descEntry.SetMinRowsVisible(3)

	// Fetch from Git button
	hasRepoAssoc := s.hasRepoAssociationForProject(entry.ProjectID)
	fetchButton := widget.NewButton("Fetch from Git", func() {
		s.fetchGitCommitsForEntry(&entry, descEntry)
	})
	if !hasRepoAssoc || !isEditable {
		fetchButton.Disable()
	}

	// Date/time fields
	fromTime, _ := entry.GetFromTimeAsTime()

	startDateEntry := widget.NewEntry()
	startDateEntry.SetPlaceHolder("YYYY-MM-DD")
	startDateEntry.SetText(fromTime.Local().Format("2006-01-02"))

	startTimeEntry := widget.NewEntry()
	startTimeEntry.SetPlaceHolder("HH:MM")
	startTimeEntry.SetText(fromTime.Local().Format("15:04"))

	endDateEntry := widget.NewEntry()
	endDateEntry.SetPlaceHolder("YYYY-MM-DD")

	endTimeEntry := widget.NewEntry()
	endTimeEntry.SetPlaceHolder("HH:MM")

	if entry.ToTime != "" {
		toTime, _ := entry.GetToTimeAsTime()
		endDateEntry.SetText(toTime.Local().Format("2006-01-02"))
		endTimeEntry.SetText(toTime.Local().Format("15:04"))
	}

	// Disable fields if submitted
	if !isEditable {
		descEntry.Disable()
		startDateEntry.Disable()
		startTimeEntry.Disable()
		endDateEntry.Disable()
		endTimeEntry.Disable()
	}

	// Error label
	errorLabel = canvas.NewText("", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF})
	errorLabel.TextSize = 12
	errorLabel.Alignment = fyne.TextAlignCenter

	// Status info row
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	statusText := "Pending"
	statusColor := goldColor
	if entry.ToTime == "" {
		statusText = "Running"
		statusColor = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
	} else if entry.Submitted {
		statusText = "Submitted"
		statusColor = color.NRGBA{R: 0x3A, G: 0x9A, B: 0xD9, A: 0xFF}
	}
	statusDisplay := canvas.NewText(fmt.Sprintf("Status: %s  |  %.2f hours", statusText, entry.Hours), statusColor)
	statusDisplay.TextSize = 12
	statusDisplay.Alignment = fyne.TextAlignCenter

	// POW video info
	var powSection fyne.CanvasObject
	powVideoPath, hasPOW := s.powVideoMap[entry.ID]
	if hasPOW && powVideoPath != "" {
		playBtn := widget.NewButton("Play POW Video", func() {
			s.playPOWVideo(powVideoPath)
		})
		playBtn.Importance = widget.HighImportance
		powSection = container.NewHBox(layout.NewSpacer(), playBtn)
	}

	// Date/time row: Start Date + Start Time | End Date + End Time
	dateTimeRow := container.NewGridWithColumns(4,
		container.NewVBox(widget.NewLabel("Start Date"), startDateEntry),
		container.NewVBox(widget.NewLabel("Start Time"), startTimeEntry),
		container.NewVBox(widget.NewLabel("End Date"), endDateEntry),
		container.NewVBox(widget.NewLabel("End Time"), endTimeEntry),
	)

	// Buttons
	var d *widget.PopUp

	closeButton := widget.NewButton("Close", func() {
		if d != nil {
			d.Hide()
		}
	})

	buttonRow := container.NewHBox(layout.NewSpacer(), closeButton)

	if isEditable {
		saveButton := widget.NewButton("Save Changes", func() {
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

			// Parse end date/time (optional for running entries)
			var endTimePtr *time.Time
			var duration float64
			if endDateEntry.Text != "" && endTimeEntry.Text != "" {
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
				endTimePtr = &endParsed
				duration = endParsed.Sub(startParsed).Hours()
			}

			errorLabel.Text = ""
			errorLabel.Refresh()

			// Build TimeEntry for UpdateTimesheet
			updatedEntry := models.TimeEntry{
				ProjectID:   fmt.Sprintf("%d", selectedProject.ID),
				ActivityID:  fmt.Sprintf("%d", selectedActivity.ID),
				Description: descEntry.Text,
				StartTime:   startParsed,
				EndTime:     endTimePtr,
				Duration:    duration,
			}
			if selectedTask != nil {
				taskID := fmt.Sprintf("%d", selectedTask.ID)
				updatedEntry.TaskID = &taskID
			}

			if d != nil {
				d.Hide()
			}
			s.setStatus("Saving changes...")

			util.RunAsync(
				func() (bool, error) {
					return true, s.apiClient.UpdateTimesheet(
						fmt.Sprintf("%d", entry.ID), updatedEntry)
				},
				func(_ bool, err error) {
					if err != nil {
						s.setStatus("Error: " + err.Error())
						return
					}
					s.setStatus("Entry updated")
					s.Refresh()
				},
			)
		})
		saveButton.Importance = widget.HighImportance

		deleteBtn := widget.NewButton("Delete", func() {
			if d != nil {
				d.Hide()
			}
			s.confirmDelete(index)
		})
		deleteBtn.Importance = widget.DangerImportance

		buttonRow = container.NewHBox(
			layout.NewSpacer(),
			deleteBtn,
			closeButton,
			saveButton,
		)
	}

	// Description container with fetch button
	descContainer := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), fetchButton),
		nil,
		nil,
		descEntry,
	)

	// Form layout matching create/stop dialog style
	form := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		widget.NewLabel("Project *"),
		projectSelect.Entry,
		widget.NewLabel("Task"),
		taskSelect.Entry,
		widget.NewLabel("Activity *"),
		activitySelect.Entry,
		widget.NewLabel("Description"),
		descContainer,
		dateTimeRow,
		container.NewCenter(statusDisplay),
	)

	// Add POW section if present
	if powSection != nil {
		form.Add(powSection)
	}

	form.Add(layout.NewSpacer())
	form.Add(errorLabel)
	form.Add(widget.NewSeparator())
	form.Add(buttonRow)

	// Gold-bordered dark background (identical to create/stop dialogs)
	formBorder := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	formBorder.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	formBorder.StrokeWidth = 2
	formBorder.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(form)),
	)

	// Modal popup overlay (same as create/stop dialogs)
	d = widget.NewModalPopUp(formContainer, s.window.Canvas())
	d.Resize(fyne.NewSize(550, 650))
	d.Show()

	// Load tasks for current project
	util.RunAsync(
		func() ([]api.TaskListItem, error) {
			return s.apiClient.GetTasks(fmt.Sprintf("%d", entry.ProjectID))
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
			if !isEditable {
				taskSelect.Disable()
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
			if !isEditable {
				activitySelect.Disable()
			}
		},
	)

	// Disable project select for submitted entries (after showing dialog)
	if !isEditable {
		projectSelect.Disable()
	}
}

// hasRepoAssociationForProject checks if there's a code repo association for the given project
func (s *HistoryScreen) hasRepoAssociationForProject(projectID int) bool {
	if s.store == nil {
		return false
	}

	associations, err := s.store.LoadCodeRepoAssociations()
	if err != nil || associations == nil {
		return false
	}

	assoc := associations.GetAssociationByProject(projectID)
	return assoc != nil && assoc.HasAssociation()
}

// fetchGitCommitsForEntry fetches git commits for the entry's time range
func (s *HistoryScreen) fetchGitCommitsForEntry(entry *api.TimelogEntry, descEntry *widget.Entry) {
	if s.store == nil {
		return
	}

	associations, err := s.store.LoadCodeRepoAssociations()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	assoc := associations.GetAssociationByProject(entry.ProjectID)
	if assoc == nil || !assoc.HasAssociation() {
		dialog.ShowInformation("No Repository",
			"No repository is linked to this project.\n\nGo to Settings > Code Repositories to link a repository.",
			s.window)
		return
	}

	// Use entry's time range
	startTime, _ := entry.GetFromTimeAsTime()
	endTime := time.Now().UTC()
	if entry.ToTime != "" {
		endTime, _ = entry.GetToTimeAsTime()
	}

	if api.IsLocalPath(assoc.RepoURL) {
		s.fetchLocalGitCommitsForEntry(assoc.RepoURL, startTime, endTime, descEntry)
	} else {
		s.fetchGitHubCommitsForEntry(assoc.RepoOwner, assoc.RepoName, startTime, endTime, descEntry)
	}
}

// fetchLocalGitCommitsForEntry fetches commits from a local git repository
func (s *HistoryScreen) fetchLocalGitCommitsForEntry(repoPath string, startTime, endTime time.Time, descEntry *widget.Entry) {
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
				dialog.ShowInformation("No Commits", "No commits found in the entry time range.", s.window)
				return
			}
			existing := descEntry.Text
			if existing != "" {
				descEntry.SetText(existing + "\n\n" + description)
			} else {
				descEntry.SetText(description)
			}
		},
	)
}

// fetchGitHubCommitsForEntry fetches commits from a GitHub repository
func (s *HistoryScreen) fetchGitHubCommitsForEntry(owner, repo string, startTime, endTime time.Time, descEntry *widget.Entry) {
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
				dialog.ShowInformation("No Commits", "No commits found in the entry time range.", s.window)
				return
			}
			existing := descEntry.Text
			if existing != "" {
				descEntry.SetText(existing + "\n\n" + description)
			} else {
				descEntry.SetText(description)
			}
		},
	)
}

// playPOWVideo opens the POW video in the system's default video player
func (s *HistoryScreen) playPOWVideo(videoPath string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", videoPath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", videoPath)
	default: // Linux and others
		cmd = exec.Command("xdg-open", videoPath)
	}

	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("failed to open video: %w", err), s.window)
	}
}

func (s *HistoryScreen) confirmDelete(index int) {
	// Custom confirmation dialog with dark text for light background
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	redColor := color.NRGBA{R: 0xC0, G: 0x39, B: 0x2B, A: 0xFF}

	titleText := canvas.NewText("Delete Entry", redColor)
	titleText.TextSize = 18
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageText := canvas.NewText("Are you sure you want to delete this entry?", darkGray)
	messageText.TextSize = 14

	content := container.NewVBox(
		container.NewCenter(titleText),
		widget.NewSeparator(),
		container.NewCenter(messageText),
	)

	var d dialog.Dialog
	yesBtn := widget.NewButton("Delete", func() {
		d.Hide()
		s.deleteEntry(index)
	})
	yesBtn.Importance = widget.DangerImportance

	noBtn := widget.NewButton("Cancel", func() {
		d.Hide()
	})

	buttons := container.NewHBox(layout.NewSpacer(), noBtn, yesBtn, layout.NewSpacer())
	fullContent := container.NewVBox(content, widget.NewSeparator(), buttons)

	d = dialog.NewCustomWithoutButtons("", fullContent, s.window)
	d.Resize(fyne.NewSize(350, 150))
	d.Show()
}

func (s *HistoryScreen) deleteEntry(index int) {
	if index >= len(s.entries) {
		return
	}

	entry := s.entries[index]
	s.setStatus("Deleting entry...")

	util.RunAsync(
		func() (bool, error) {
			return true, s.apiClient.DeleteTimeLog(fmt.Sprintf("%d", entry.ID))
		},
		func(_ bool, err error) {
			if err != nil {
				s.setStatus("Error: " + err.Error())
				return
			}
			s.setStatus("Entry deleted")
			s.Refresh()
		},
	)
}

func (s *HistoryScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// Refresh reloads data from the API
func (s *HistoryScreen) Refresh() {
	s.setStatus("Loading...")

	util.RunAsync(
		func() ([]api.TimelogEntry, error) {
			return s.apiClient.GetTimelogs()
		},
		func(entries []api.TimelogEntry, err error) {
			if err != nil {
				s.setStatus("Error: " + err.Error())
				return
			}

			s.entries = entries

			// Load POW video mapping for all entries
			s.loadPowVideoMapping()

			// Count pending and POW videos
			pendingCount := 0
			powCount := 0
			for _, e := range entries {
				if !e.Submitted && e.ToTime != "" {
					pendingCount++
				}
				if _, hasPOW := s.powVideoMap[e.ID]; hasPOW {
					powCount++
				}
			}

			statusMsg := fmt.Sprintf("%d entries, %d pending", len(entries), pendingCount)
			if powCount > 0 {
				statusMsg += fmt.Sprintf(", %d with POW", powCount)
			}
			s.setStatus(statusMsg)
			s.entryList.Refresh()

			// Update submit button state
			s.submitButton.Enable()
			if pendingCount == 0 {
				s.submitButton.Disable()
			}
		},
	)
}

// loadPowVideoMapping loads the POW video paths for all entries
func (s *HistoryScreen) loadPowVideoMapping() {
	s.powVideoMap = make(map[int]string)

	if s.store == nil {
		return
	}

	// Load the full mapping
	mapping, err := s.store.LoadPowVideoMapping()
	if err != nil || mapping == nil {
		return
	}

	// Build a map of entry IDs that we have
	entryIDs := make(map[int]bool)
	for _, e := range s.entries {
		entryIDs[e.ID] = true
	}

	// Only include videos for entries we have
	for entryID, videoPath := range mapping.Entries {
		if entryIDs[entryID] {
			s.powVideoMap[entryID] = videoPath
		}
	}
}

// submitPendingEntries submits all pending entries
func (s *HistoryScreen) submitPendingEntries() {
	// Count pending entries
	pendingEntries := make([]api.TimelogEntry, 0)
	for _, e := range s.entries {
		if !e.Submitted && e.ToTime != "" {
			pendingEntries = append(pendingEntries, e)
		}
	}

	if len(pendingEntries) == 0 {
		s.setStatus("No pending entries to submit")
		return
	}

	// Custom confirmation dialog with dark text for light background
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	titleText := canvas.NewText("Submit Timesheets", goldColor)
	titleText.TextSize = 18
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageText := canvas.NewText(fmt.Sprintf("Submit %d pending entries?", len(pendingEntries)), darkGray)
	messageText.TextSize = 14

	content := container.NewVBox(
		container.NewCenter(titleText),
		widget.NewSeparator(),
		container.NewCenter(messageText),
	)

	var d dialog.Dialog
	yesBtn := widget.NewButton("Yes", func() {
		d.Hide()
		s.doSubmit(pendingEntries)
	})
	yesBtn.Importance = widget.HighImportance

	noBtn := widget.NewButton("No", func() {
		d.Hide()
	})

	buttons := container.NewHBox(layout.NewSpacer(), noBtn, yesBtn, layout.NewSpacer())
	fullContent := container.NewVBox(content, widget.NewSeparator(), buttons)

	d = dialog.NewCustomWithoutButtons("", fullContent, s.window)
	d.Resize(fyne.NewSize(350, 150))
	d.Show()
}

// doSubmit performs the actual submission
func (s *HistoryScreen) doSubmit(entries []api.TimelogEntry) {
	s.setStatus(fmt.Sprintf("Submitting %d entries...", len(entries)))
	s.submitButton.Disable()

	util.RunAsync(
		func() (int, error) {
			// Use SubmitAllPending which handles all pending entries
			err := s.apiClient.SubmitAllPending()
			if err != nil {
				return 0, err
			}
			return len(entries), nil
		},
		func(successCount int, err error) {
			if err != nil {
				s.setStatus("Error: " + err.Error())
				s.submitButton.Enable()
				return
			}

			s.setStatus(fmt.Sprintf("Successfully submitted %d entries!", successCount))

			// Show fireworks celebration!
			if successCount > 0 {
				widgets.ShowFireworksCelebration(
					s.window,
					fmt.Sprintf("🎉 Submitted %d Timesheets! 🎉", successCount),
					8*time.Second,
				)
			}

			// Refresh the list
			s.Refresh()
		},
	)
}
