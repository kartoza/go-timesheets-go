package screens

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
)

// HistoryScreen represents the history screen
type HistoryScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window

	// Widgets
	entryList   *widget.List
	backButton  *widget.Button
	statusLabel *canvas.Text

	// Data
	entries []api.TimelogEntry

	// Callbacks
	OnBack func()
}

// NewHistoryScreen creates a new history screen
func NewHistoryScreen(apiClient *api.Client, window fyne.Window) *HistoryScreen {
	s := &HistoryScreen{
		apiClient: apiClient,
		window:    window,
		entries:   []api.TimelogEntry{},
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

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	buttonRow := container.NewHBox(
		s.backButton,
		layout.NewSpacer(),
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

	// Row 2: Task, Status
	taskText := entry.TaskName
	if taskText == "" {
		taskText = "(No task)"
	}
	row2.Objects[0].(*widget.Label).SetText(taskText)

	statusText := ""
	if entry.ToTime == "" {
		statusText = "Running"
	} else if entry.Submitted {
		statusText = "Submitted"
	} else {
		statusText = "Pending"
	}
	row2.Objects[2].(*widget.Label).SetText(statusText)
}

func (s *HistoryScreen) showEntryDetail(index int) {
	if index >= len(s.entries) {
		return
	}

	entry := s.entries[index]

	description := "(No description)"
	if entry.Description != nil && *entry.Description != "" {
		description = *entry.Description
	}

	// Build detail content
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Project: %s", entry.ProjectName)),
		widget.NewLabel(fmt.Sprintf("Task: %s", entry.TaskName)),
		widget.NewLabel(fmt.Sprintf("Activity: %s", entry.ActivityType)),
		widget.NewSeparator(),
		widget.NewLabel(fmt.Sprintf("Date: %s", entry.FromTime)),
		widget.NewLabel(fmt.Sprintf("Hours: %.2f", entry.Hours)),
		widget.NewSeparator(),
		widget.NewLabel("Description:"),
	)

	descLabel := widget.NewLabel(description)
	descLabel.Wrapping = fyne.TextWrapWord
	content.Add(descLabel)

	// Status
	content.Add(widget.NewSeparator())
	statusText := "Pending"
	if entry.ToTime == "" {
		statusText = "Running"
	} else if entry.Submitted {
		statusText = "Submitted"
	}
	content.Add(widget.NewLabel(fmt.Sprintf("Status: %s", statusText)))

	// Delete button for non-submitted entries
	if !entry.Submitted {
		content.Add(widget.NewSeparator())
		deleteBtn := widget.NewButton("Delete Entry", func() {
			s.confirmDelete(index)
		})
		deleteBtn.Importance = widget.DangerImportance
		content.Add(deleteBtn)
	}

	d := dialog.NewCustom("Entry Details", "Close", content, s.window)
	d.Resize(fyne.NewSize(400, 400))
	d.Show()
}

func (s *HistoryScreen) confirmDelete(index int) {
	dialog.ShowConfirm(
		"Delete Entry",
		"Are you sure you want to delete this entry?",
		func(confirmed bool) {
			if confirmed {
				s.deleteEntry(index)
			}
		},
		s.window,
	)
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

			// Count pending
			pendingCount := 0
			for _, e := range entries {
				if !e.Submitted && e.ToTime != "" {
					pendingCount++
				}
			}

			s.setStatus(fmt.Sprintf("%d entries, %d pending", len(entries), pendingCount))
			s.entryList.Refresh()
		},
	)
}
