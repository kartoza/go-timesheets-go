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
	statusLabel := widget.NewLabel("Status") // Will contain icons + status text

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

	// Row 1: Project, Date, Hours
	row1.Objects[0].(*widget.Label).SetText(entry.ProjectName)

	fromTime, err := entry.GetFromTimeAsTime()
	if err == nil {
		row1.Objects[2].(*widget.Label).SetText(fromTime.Format("2006-01-02"))
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
}

func (s *HistoryScreen) showEntryDetail(index int) {
	if index >= len(s.entries) {
		return
	}

	entry := s.entries[index]

	// Colors - use dark colors that work on light dialog backgrounds
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	blueColor := color.NRGBA{R: 0x2B, G: 0x7A, B: 0xB0, A: 0xFF} // Darker blue for light bg
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}  // Dark gray for text
	greenColor := color.NRGBA{R: 0x1E, G: 0x88, B: 0x4B, A: 0xFF} // Darker green

	// Project name (title)
	projectTitle := canvas.NewText(entry.ProjectName, goldColor)
	projectTitle.TextSize = 18
	projectTitle.TextStyle = fyne.TextStyle{Bold: true}

	// Task and Activity - use canvas.NewText with dark color
	taskText := entry.TaskName
	if taskText == "" {
		taskText = "(No task)"
	}
	taskLabel := canvas.NewText(fmt.Sprintf("📋 Task: %s", taskText), darkGray)
	taskLabel.TextSize = 13

	activityLabel := canvas.NewText(fmt.Sprintf("🎯 Activity: %s", entry.ActivityType), darkGray)
	activityLabel.TextSize = 13

	// Time info
	fromTime, _ := entry.GetFromTimeAsTime()
	dateStr := fromTime.Format("2006-01-22 15:04")
	if entry.ToTime != "" {
		toTime, _ := entry.GetToTimeAsTime()
		dateStr = fmt.Sprintf("%s → %s", fromTime.Format("15:04"), toTime.Format("15:04"))
	}

	dateLabel := canvas.NewText(fmt.Sprintf("📅 %s  |  %s", fromTime.Format("2006-01-02"), dateStr), blueColor)
	dateLabel.TextSize = 12

	hoursLabel := canvas.NewText(fmt.Sprintf("⏱ %.2f hours", entry.Hours), goldColor)
	hoursLabel.TextSize = 14
	hoursLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Status
	statusText := "○ Pending"
	statusColor := goldColor
	if entry.ToTime == "" {
		statusText = "🟢 Running"
		statusColor = greenColor
	} else if entry.Submitted {
		statusText = "✓ Submitted"
		statusColor = blueColor
	}
	statusLabelText := canvas.NewText("Status: ", darkGray)
	statusLabelText.TextSize = 13
	statusLabel := canvas.NewText(statusText, statusColor)
	statusLabel.TextSize = 14
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Description
	description := "(No description)"
	if entry.Description != nil && *entry.Description != "" {
		description = *entry.Description
	}
	descTitle := canvas.NewText("📝 Description:", darkGray)
	descTitle.TextSize = 12

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(description)
	descEntry.Disable()
	descEntry.SetMinRowsVisible(4)

	// Build content
	content := container.NewVBox(
		container.NewCenter(projectTitle),
		widget.NewSeparator(),
		taskLabel,
		activityLabel,
		widget.NewSeparator(),
		container.NewHBox(dateLabel, layout.NewSpacer(), hoursLabel),
		container.NewHBox(statusLabelText, statusLabel),
		widget.NewSeparator(),
		descTitle,
		descEntry,
	)

	// POW Video section
	powVideoPath, hasPOW := s.powVideoMap[entry.ID]
	if hasPOW && powVideoPath != "" {
		content.Add(widget.NewSeparator())

		powTitle := canvas.NewText("🎬 Proof of Work Video", goldColor)
		powTitle.TextSize = 14
		powTitle.TextStyle = fyne.TextStyle{Bold: true}
		content.Add(powTitle)

		// Show video path (truncated) - use dark color
		pathDisplay := powVideoPath
		if len(pathDisplay) > 50 {
			pathDisplay = "..." + pathDisplay[len(pathDisplay)-47:]
		}
		pathLabel := canvas.NewText(pathDisplay, darkGray)
		pathLabel.TextSize = 11
		content.Add(pathLabel)

		// Play button
		playBtn := widget.NewButton("▶ Play POW Video", func() {
			s.playPOWVideo(powVideoPath)
		})
		playBtn.Importance = widget.HighImportance
		content.Add(container.NewCenter(playBtn))
	}

	// Buttons section
	content.Add(widget.NewSeparator())
	buttonBox := container.NewHBox(layout.NewSpacer())

	// Delete button for non-submitted entries
	if !entry.Submitted {
		deleteBtn := widget.NewButton("🗑 Delete", func() {
			s.confirmDelete(index)
		})
		deleteBtn.Importance = widget.DangerImportance
		buttonBox.Add(deleteBtn)
	}

	content.Add(buttonBox)

	// Create scrollable container
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(450, 450))

	d := dialog.NewCustom("Entry Details", "Close", scroll, s.window)
	d.Resize(fyne.NewSize(500, 550))
	d.Show()
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
