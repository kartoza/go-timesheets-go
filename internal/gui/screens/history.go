package screens

import (
	"fmt"
	"image/color"
	"os/exec"
	"runtime"
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
	"github.com/kartoza/go-timesheets-go/internal/timefmt"
)

// HistoryScreen represents the history screen with two-column layout
type HistoryScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	store     *storage.Storage

	// Widgets
	entryList     *widget.List
	backButton    *widget.Button
	submitButton  *widget.Button
	statusLabel   *canvas.Text
	detailPanel   *fyne.Container
	selectedIndex int

	// Detail panel components
	detailTitle       *canvas.Text
	detailProject     *canvas.Text
	detailTask        *canvas.Text
	detailActivity    *canvas.Text
	detailDate        *canvas.Text
	detailTime        *canvas.Text
	detailHours       *canvas.Text
	detailStatus      *canvas.Text
	detailDescLabel   *canvas.Text
	detailDescription *widget.Label
	detailEditBtn     *widget.Button
	detailDeleteBtn   *widget.Button
	detailPowBtn      *widget.Button

	// Data
	entries     []api.TimelogEntry
	powVideoMap map[int]string // entry ID -> video path

	// Callbacks
	OnBack func()
}

// NewHistoryScreen creates a new history screen
func NewHistoryScreen(apiClient *api.Client, window fyne.Window) *HistoryScreen {
	s := &HistoryScreen{
		apiClient:     apiClient,
		window:        window,
		entries:       []api.TimelogEntry{},
		powVideoMap:   make(map[int]string),
		selectedIndex: -1,
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
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}

	// Title
	title := canvas.NewText("Timesheet History", goldColor)
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Entry list - compact without descriptions
	s.entryList = widget.NewList(
		func() int { return len(s.entries) },
		func() fyne.CanvasObject {
			return s.createCompactEntryRow()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			s.updateCompactEntryRow(id, obj)
		},
	)
	s.entryList.OnSelected = func(id widget.ListItemID) {
		s.selectedIndex = int(id)
		s.updateDetailPanel()
	}

	// Buttons
	s.backButton = widget.NewButton("← Back", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	s.submitButton = widget.NewButton("↑ Submit All Pending", s.submitPendingEntries)
	s.submitButton.Importance = widget.HighImportance

	// Status label
	s.statusLabel = canvas.NewText("", grayColor)
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	buttonRow := container.NewHBox(
		s.backButton,
		layout.NewSpacer(),
		s.submitButton,
	)

	// Build detail panel
	s.buildDetailPanel()

	// Left column: list
	leftColumn := container.NewBorder(
		nil, nil, nil, nil,
		s.entryList,
	)

	// Right column: detail panel with border
	detailBorder := canvas.NewRectangle(color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF})
	rightColumn := container.NewStack(
		detailBorder,
		container.NewPadded(s.detailPanel),
	)

	// Two-column split (60% list, 40% detail)
	splitContainer := container.NewHSplit(leftColumn, rightColumn)
	splitContainer.SetOffset(0.55)

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
		splitContainer,
	)
}

func (s *HistoryScreen) buildDetailPanel() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	whiteColor := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}

	// Title
	s.detailTitle = canvas.NewText("Select an entry", goldColor)
	s.detailTitle.TextSize = 16
	s.detailTitle.TextStyle = fyne.TextStyle{Bold: true}

	// Labels
	s.detailProject = canvas.NewText("", whiteColor)
	s.detailProject.TextSize = 14
	s.detailProject.TextStyle = fyne.TextStyle{Bold: true}

	s.detailTask = canvas.NewText("", grayColor)
	s.detailTask.TextSize = 12

	s.detailActivity = canvas.NewText("", grayColor)
	s.detailActivity.TextSize = 12

	s.detailDate = canvas.NewText("", whiteColor)
	s.detailDate.TextSize = 12

	s.detailTime = canvas.NewText("", grayColor)
	s.detailTime.TextSize = 12

	s.detailHours = canvas.NewText("", goldColor)
	s.detailHours.TextSize = 14
	s.detailHours.TextStyle = fyne.TextStyle{Bold: true}

	s.detailStatus = canvas.NewText("", grayColor)
	s.detailStatus.TextSize = 12

	s.detailDescLabel = canvas.NewText("Description:", whiteColor)
	s.detailDescLabel.TextSize = 12
	s.detailDescLabel.TextStyle = fyne.TextStyle{Bold: true}

	s.detailDescription = widget.NewLabel("Select an entry from the list to view details")
	s.detailDescription.Wrapping = fyne.TextWrapWord

	// Action buttons
	s.detailEditBtn = widget.NewButton("Edit", func() {
		if s.selectedIndex >= 0 && s.selectedIndex < len(s.entries) {
			s.showEntryDetail(s.selectedIndex)
		}
	})
	s.detailEditBtn.Importance = widget.HighImportance
	s.detailEditBtn.Hide()

	s.detailDeleteBtn = widget.NewButton("Delete", func() {
		if s.selectedIndex >= 0 && s.selectedIndex < len(s.entries) {
			s.confirmDelete(s.selectedIndex)
		}
	})
	s.detailDeleteBtn.Importance = widget.DangerImportance
	s.detailDeleteBtn.Hide()

	s.detailPowBtn = widget.NewButton("▶ Play POW", func() {
		if s.selectedIndex >= 0 && s.selectedIndex < len(s.entries) {
			entry := s.entries[s.selectedIndex]
			if videoPath, ok := s.powVideoMap[entry.ID]; ok {
				s.playPOWVideo(videoPath)
			}
		}
	})
	s.detailPowBtn.Hide()

	buttonBox := container.NewHBox(
		s.detailDeleteBtn,
		layout.NewSpacer(),
		s.detailPowBtn,
		s.detailEditBtn,
	)

	// Top section: metadata (fixed height)
	topSection := container.NewVBox(
		s.detailTitle,
		widget.NewSeparator(),
		s.detailProject,
		s.detailTask,
		s.detailActivity,
		widget.NewSeparator(),
		container.NewHBox(s.detailDate, layout.NewSpacer(), s.detailHours),
		s.detailTime,
		s.detailStatus,
		widget.NewSeparator(),
		s.detailDescLabel,
	)

	// Bottom section: buttons (fixed height)
	bottomSection := container.NewVBox(
		widget.NewSeparator(),
		buttonBox,
	)

	// Description scroll area expands to fill remaining space
	descScroll := container.NewScroll(s.detailDescription)

	// Use Border layout: top fixed, bottom fixed, center (description) expands
	s.detailPanel = container.NewBorder(
		topSection,    // top
		bottomSection, // bottom
		nil,           // left
		nil,           // right
		descScroll,    // center - expands to fill available space
	)
}

func (s *HistoryScreen) updateDetailPanel() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	greenColor := color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
	blueColor := color.NRGBA{R: 0x3A, G: 0x9A, B: 0xD9, A: 0xFF}

	if s.selectedIndex < 0 || s.selectedIndex >= len(s.entries) {
		s.detailTitle.Text = "Select an entry"
		s.detailTitle.Refresh()
		s.detailProject.Text = ""
		s.detailProject.Refresh()
		s.detailTask.Text = ""
		s.detailTask.Refresh()
		s.detailActivity.Text = ""
		s.detailActivity.Refresh()
		s.detailDate.Text = ""
		s.detailDate.Refresh()
		s.detailTime.Text = ""
		s.detailTime.Refresh()
		s.detailHours.Text = ""
		s.detailHours.Refresh()
		s.detailStatus.Text = ""
		s.detailStatus.Refresh()
		s.detailDescription.SetText("Select an entry from the list to view details")
		s.detailEditBtn.Hide()
		s.detailDeleteBtn.Hide()
		s.detailPowBtn.Hide()
		return
	}

	entry := s.entries[s.selectedIndex]

	// Title
	s.detailTitle.Text = "Entry Details"
	s.detailTitle.Refresh()

	// Project
	s.detailProject.Text = entry.ProjectName
	s.detailProject.Refresh()

	// Task
	if entry.TaskName != "" {
		s.detailTask.Text = "Task: " + entry.TaskName
	} else {
		s.detailTask.Text = "Task: (none)"
	}
	s.detailTask.Refresh()

	// Activity
	s.detailActivity.Text = "Activity: " + entry.ActivityType
	s.detailActivity.Refresh()

	// Date and time
	fromTime, err := entry.GetFromTimeAsTime()
	if err == nil {
		s.detailDate.Text = fromTime.Local().Format("Monday, January 02, 2006")
		if entry.ToTime != "" {
			toTime, _ := entry.GetToTimeAsTime()
			s.detailTime.Text = fmt.Sprintf("%s - %s",
				fromTime.Local().Format("15:04"),
				toTime.Local().Format("15:04"))
		} else {
			s.detailTime.Text = fmt.Sprintf("%s - running", fromTime.Local().Format("15:04"))
		}
	} else {
		s.detailDate.Text = entry.FromTime
		s.detailTime.Text = ""
	}
	s.detailDate.Refresh()
	s.detailTime.Refresh()

	// Hours
	s.detailHours.Text = fmt.Sprintf("%.2f hours", entry.Hours)
	s.detailHours.Refresh()

	// Status — BMP Unicode symbols + colour. The bundled Noto Sans font
	// covers these glyphs cleanly (we previously used color emoji which
	// the font lacks; now we use ● for the running indicator, ✓ for
	// submitted, and ○ for pending).
	if entry.ToTime == "" {
		s.detailStatus.Text = "● Running"
		s.detailStatus.Color = greenColor
	} else if entry.Submitted {
		s.detailStatus.Text = "✓ Submitted"
		s.detailStatus.Color = blueColor
	} else {
		s.detailStatus.Text = "○ Pending"
		s.detailStatus.Color = goldColor
	}
	s.detailStatus.Refresh()

	// Description
	if entry.Description != nil && *entry.Description != "" {
		s.detailDescription.SetText(*entry.Description)
	} else {
		s.detailDescription.SetText("(No description)")
	}

	// Show/hide action buttons based on entry state
	isEditable := !entry.Submitted
	if isEditable {
		s.detailEditBtn.Show()
		s.detailDeleteBtn.Show()
	} else {
		s.detailEditBtn.SetText("View")
		s.detailEditBtn.Show()
		s.detailDeleteBtn.Hide()
	}

	// POW button
	if videoPath, hasPOW := s.powVideoMap[entry.ID]; hasPOW && videoPath != "" {
		s.detailPowBtn.Show()
	} else {
		s.detailPowBtn.Hide()
	}
}

func (s *HistoryScreen) createCompactEntryRow() fyne.CanvasObject {
	projectLabel := widget.NewLabel("Project")
	projectLabel.TextStyle = fyne.TextStyle{Bold: true}
	projectLabel.Truncation = fyne.TextTruncateEllipsis

	taskLabel := widget.NewLabel("Task")
	taskLabel.Truncation = fyne.TextTruncateEllipsis

	dateLabel := widget.NewLabel("Date")
	hoursLabel := widget.NewLabel("Hours")
	hoursLabel.TextStyle = fyne.TextStyle{Bold: true}
	statusLabel := widget.NewLabel("Status")

	// Use fixed widths for consistent columns
	dateLabel.Resize(fyne.NewSize(80, dateLabel.MinSize().Height))
	hoursLabel.Resize(fyne.NewSize(50, hoursLabel.MinSize().Height))

	row1 := container.NewBorder(nil, nil, nil,
		container.NewHBox(dateLabel, hoursLabel),
		projectLabel,
	)

	row2 := container.NewBorder(nil, nil, nil,
		statusLabel,
		taskLabel,
	)

	return container.NewVBox(row1, row2)
}

func (s *HistoryScreen) updateCompactEntryRow(id widget.ListItemID, obj fyne.CanvasObject) {
	if int(id) >= len(s.entries) {
		return
	}

	entry := s.entries[id]
	vbox := obj.(*fyne.Container)

	row1 := vbox.Objects[0].(*fyne.Container)
	row2 := vbox.Objects[1].(*fyne.Container)

	// Row 1: Project, Date, Hours
	row1.Objects[0].(*widget.Label).SetText(entry.ProjectName)

	rightBox1 := row1.Objects[1].(*fyne.Container)
	fromTime, err := entry.GetFromTimeAsTime()
	if err == nil {
		rightBox1.Objects[0].(*widget.Label).SetText(fromTime.Local().Format("2006-01-02"))
	} else {
		rightBox1.Objects[0].(*widget.Label).SetText(entry.FromTime)
	}
	rightBox1.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%.1fh", entry.Hours))

	// Row 2: Task, Status
	taskText := entry.TaskName
	if taskText == "" {
		taskText = "(No task)"
	}
	row2.Objects[0].(*widget.Label).SetText(taskText)

	// Build status text. BMP Unicode iconography rendered by the bundled
	// Noto Sans (replaces color emoji that the font does not cover).
	statusText := ""

	// POW prefix if has video
	videoPath, hasPOW := s.powVideoMap[entry.ID]
	if hasPOW && videoPath != "" {
		statusText = "▶ "
	}

	// Warning prefix if missing description (only for pending entries)
	if !entry.Submitted && entry.ToTime != "" {
		desc := entry.GetDescriptionString()
		if desc == "" {
			statusText += "⚠ "
		}
	}

	if entry.ToTime == "" {
		statusText += "● Running"
	} else if entry.Submitted {
		statusText += "✓ Submitted"
	} else {
		statusText += "○ Pending"
	}
	row2.Objects[1].(*widget.Label).SetText(statusText)
}

func (s *HistoryScreen) showEntryDetail(index int) {
	if index >= len(s.entries) {
		return
	}

	entry := s.entries[index]
	isEditable := !entry.Submitted

	titleText := "Entry Details"
	if entry.Submitted {
		titleText = "Entry Details (Submitted)"
	}

	desc := ""
	if entry.Description != nil {
		desc = *entry.Description
	}

	fromTime, _ := entry.GetFromTimeAsTime()
	preFill := &widgets.EntryFormData{
		ProjectID:    entry.ProjectID,
		ProjectName:  entry.ProjectName,
		TaskID:       entry.TaskID.Value,
		TaskName:     entry.TaskName,
		ActivityID:   entry.ActivityID,
		ActivityName: entry.ActivityType,
		Description:  desc,
		StartDate:    fromTime.Local().Format("2006-01-02"),
		StartTime:    fromTime.Local().Format("15:04"),
	}
	if entry.ToTime != "" {
		toTime, _ := entry.GetToTimeAsTime()
		preFill.EndDate = toTime.Local().Format("2006-01-02")
		preFill.EndTime = toTime.Local().Format("15:04")
	}

	// Build extra content: status display + POW video button
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

	extraItems := container.NewVBox(container.NewCenter(statusDisplay))

	powVideoPath, hasPOW := s.powVideoMap[entry.ID]
	if hasPOW && powVideoPath != "" {
		playBtn := widget.NewButton("Play POW Video", func() {
			s.playPOWVideo(powVideoPath)
		})
		playBtn.Importance = widget.HighImportance
		extraItems.Add(container.NewHBox(layout.NewSpacer(), playBtn))
	}

	// Build buttons
	var buttons []widgets.EntryFormButton

	if isEditable {
		buttons = append(buttons, widgets.EntryFormButton{
			Label:       "Delete",
			Importance:  widget.DangerImportance,
			LeftAligned: true,
			OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
				close()
				s.confirmDelete(index)
			},
		})
	}
	buttons = append(buttons, widgets.EntryFormButton{
		Label: "Close",
		OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
			close()
		},
	})
	if isEditable {
		buttons = append(buttons, widgets.EntryFormButton{
			Label:      "Save Changes",
			Importance: widget.HighImportance,
			OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
				s.doSaveEntry(entry.ID, data, setError, close)
			},
		})
	}

	// Git fetcher for editable entries
	var gitFetcher func(descEntry *widget.Entry)
	if isEditable && s.hasRepoAssociationForProject(entry.ProjectID) {
		entryCopy := entry
		gitFetcher = func(descEntry *widget.Entry) {
			s.fetchGitCommitsForEntry(&entryCopy, descEntry)
		}
	}

	widgets.ShowEntryFormDialog(&widgets.EntryFormConfig{
		Title:           titleText,
		Window:          s.window,
		APIClient:       s.apiClient,
		ShowDescription: true,
		ShowDateTime:    true,
		ShowTask:        true,
		ShowActivity:    true,
		ReadOnly:        !isEditable,
		PreFill:         preFill,
		ExtraContent:    extraItems,
		GitFetcher:      gitFetcher,
		Buttons:         buttons,
	})
}

// doSaveEntry handles saving changes from the entry detail dialog
func (s *HistoryScreen) doSaveEntry(entryID int, data *widgets.EntryFormData, setError func(string), close func()) {
	if data.ProjectID == 0 {
		setError("Please select a project")
		return
	}
	if data.ActivityID == 0 {
		setError("Please select an activity")
		return
	}

	startParsed, err := timefmt.ParseFlexibleDateTime(data.StartDate, data.StartTime, time.Local)
	if err != nil {
		setError("Invalid start date/time: " + err.Error())
		return
	}
	startParsed = startParsed.UTC()

	var endTimePtr *time.Time
	var duration float64
	if data.EndDate != "" && data.EndTime != "" {
		endParsed, err := timefmt.ParseFlexibleDateTime(data.EndDate, data.EndTime, time.Local)
		if err != nil {
			setError("Invalid end date/time: " + err.Error())
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

	updatedEntry := models.TimeEntry{
		ProjectID:   fmt.Sprintf("%d", data.ProjectID),
		ActivityID:  fmt.Sprintf("%d", data.ActivityID),
		Description: data.Description,
		StartTime:   startParsed,
		EndTime:     endTimePtr,
		Duration:    duration,
	}
	if data.TaskID > 0 {
		taskID := fmt.Sprintf("%d", data.TaskID)
		updatedEntry.TaskID = &taskID
	}

	close()
	s.setStatus("Saving changes...")

	util.RunAsync(
		func() (bool, error) {
			return true, s.apiClient.UpdateTimesheet(
				fmt.Sprintf("%d", entryID), updatedEntry)
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
	// The app theme is dark, so the message text must be light to be
	// readable. (Previously this used #333333, intended for a light
	// background, and rendered as unreadable dark-on-dark.)
	redColor := color.NRGBA{R: 0xC0, G: 0x39, B: 0x2B, A: 0xFF}

	titleText := canvas.NewText("Delete Entry", redColor)
	titleText.TextSize = 18
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageText := canvas.NewText("Are you sure you want to delete this entry?", color.White)
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
			s.selectedIndex = -1
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

			// Update detail panel if an entry was selected
			s.updateDetailPanel()

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
	// Count pending entries and check for missing descriptions
	pendingEntries := make([]api.TimelogEntry, 0)
	entriesWithoutDesc := make([]api.TimelogEntry, 0)
	for _, e := range s.entries {
		if !e.Submitted && e.ToTime != "" {
			pendingEntries = append(pendingEntries, e)
			desc := e.GetDescriptionString()
			if desc == "" {
				entriesWithoutDesc = append(entriesWithoutDesc, e)
			}
		}
	}

	if len(pendingEntries) == 0 {
		s.setStatus("No pending entries to submit")
		return
	}

	// Reject submission if there are entries without descriptions
	if len(entriesWithoutDesc) > 0 {
		s.showMissingDescriptionsDialog(entriesWithoutDesc)
		return
	}

	// The app theme is dark, so the message text must be light to be
	// readable. (Previously this used #333333, intended for a light
	// background, and rendered as unreadable dark-on-dark.)
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	titleText := canvas.NewText("Submit Timesheets", goldColor)
	titleText.TextSize = 18
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageText := canvas.NewText(fmt.Sprintf("Submit %d pending entries?", len(pendingEntries)), color.White)
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

// showMissingDescriptionsDialog shows an error dialog when entries are missing descriptions
func (s *HistoryScreen) showMissingDescriptionsDialog(entries []api.TimelogEntry) {
	redColor := color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}

	titleText := canvas.NewText("Cannot Submit", redColor)
	titleText.TextSize = 18
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageText := canvas.NewText(
		fmt.Sprintf("%d entr%s missing descriptions.", len(entries), func() string {
			if len(entries) == 1 {
				return "y is"
			}
			return "ies are"
		}()),
		color.White,
	)
	messageText.TextSize = 14

	var d dialog.Dialog

	// Build list of clickable entries without descriptions (max 5)
	var entryList []fyne.CanvasObject
	maxShow := 5
	if len(entries) < maxShow {
		maxShow = len(entries)
	}
	for i := 0; i < maxShow; i++ {
		e := entries[i]
		entryIndex := s.findEntryIndex(e.ID)
		fromTime, _ := e.GetFromTimeAsTime()
		entryLabel := fmt.Sprintf("• %s - %s (%.1fh)",
			e.ProjectName,
			fromTime.Local().Format("Mon Jan 02"),
			e.Hours)

		// Create a clickable button styled as text
		btn := widget.NewButton(entryLabel, func() {
			d.Hide()
			if entryIndex >= 0 {
				s.selectedIndex = entryIndex
				s.entryList.Select(widget.ListItemID(entryIndex))
				s.updateDetailPanel()
				s.showEntryDetail(entryIndex)
			}
		})
		btn.Importance = widget.LowImportance
		entryList = append(entryList, btn)
	}
	if len(entries) > 5 {
		moreText := canvas.NewText(fmt.Sprintf("... and %d more", len(entries)-5), grayColor)
		moreText.TextSize = 12
		moreText.TextStyle = fyne.TextStyle{Italic: true}
		entryList = append(entryList, moreText)
	}

	helpText := canvas.NewText("Click on each entry to add a description.", grayColor)
	helpText.TextSize = 12
	helpText.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		container.NewCenter(titleText),
		widget.NewSeparator(),
		container.NewCenter(messageText),
		widget.NewSeparator(),
	)
	for _, item := range entryList {
		content.Add(item)
	}
	content.Add(widget.NewSeparator())
	content.Add(container.NewCenter(helpText))

	okBtn := widget.NewButton("OK", func() {
		d.Hide()
	})

	buttons := container.NewHBox(layout.NewSpacer(), okBtn, layout.NewSpacer())
	fullContent := container.NewVBox(content, widget.NewSeparator(), buttons)

	d = dialog.NewCustomWithoutButtons("", fullContent, s.window)
	d.Resize(fyne.NewSize(450, 350))
	d.Show()
}

// findEntryIndex finds the index of an entry by ID in s.entries
func (s *HistoryScreen) findEntryIndex(entryID int) int {
	for i, e := range s.entries {
		if e.ID == entryID {
			return i
		}
	}
	return -1
}
