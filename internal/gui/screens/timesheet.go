package screens

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/gui/widgets"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

// TimesheetScreen represents the timesheet creation screen
type TimesheetScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window

	// Form widgets
	projectSelect  *widgets.SearchableSelect
	taskSelect     *widget.Select
	activitySelect *widget.Select
	descEntry      *widget.Entry
	submitButton   *widget.Button
	cancelButton   *widget.Button
	errorLabel     *canvas.Text
	loadingBar     *widget.ProgressBarInfinite

	// Data
	projects   []api.ProjectListItem
	tasks      []api.TaskListItem
	activities []api.ActivityListItem

	// Selected values
	selectedProject  *api.ProjectListItem
	selectedTask     *api.TaskListItem
	selectedActivity *api.ActivityListItem

	// Callbacks
	OnBack          func()
	OnTimerStarted  func()
}

// NewTimesheetScreen creates a new timesheet creation screen
func NewTimesheetScreen(apiClient *api.Client, window fyne.Window) *TimesheetScreen {
	s := &TimesheetScreen{
		apiClient: apiClient,
		window:    window,
	}
	s.build()
	return s
}

func (s *TimesheetScreen) build() {
	// Title
	title := canvas.NewText("Start New Timer", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Project select
	s.projectSelect = widgets.NewSearchableSelect("Search projects...", s.window)
	s.projectSelect.OnChanged = func(item *widgets.SelectItem) {
		if item != nil {
			s.selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
			s.loadTasks(item.ID)
		} else {
			s.selectedProject = nil
			s.taskSelect.Options = []string{}
			s.taskSelect.Refresh()
		}
	}
	s.projectSelect.OnSearch = func(query string) {
		s.searchProjects(query)
	}

	// Task select
	s.taskSelect = widget.NewSelect([]string{}, func(selected string) {
		for _, t := range s.tasks {
			if t.Label == selected {
				s.selectedTask = &t
				break
			}
		}
	})
	s.taskSelect.PlaceHolder = "Select task..."

	// Activity select
	s.activitySelect = widget.NewSelect([]string{}, func(selected string) {
		for _, a := range s.activities {
			if a.Label == selected {
				s.selectedActivity = &a
				break
			}
		}
	})
	s.activitySelect.PlaceHolder = "Select activity..."

	// Description entry
	s.descEntry = widget.NewMultiLineEntry()
	s.descEntry.SetPlaceHolder("Description (optional)")
	s.descEntry.SetMinRowsVisible(3)

	// Error label
	s.errorLabel = canvas.NewText("", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF})
	s.errorLabel.TextSize = 12
	s.errorLabel.Alignment = fyne.TextAlignCenter

	// Loading indicator
	s.loadingBar = widget.NewProgressBarInfinite()
	s.loadingBar.Hide()

	// Buttons
	s.submitButton = widget.NewButton("Start Timer", s.onSubmit)
	s.submitButton.Importance = widget.HighImportance

	s.cancelButton = widget.NewButton("Cancel", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		s.cancelButton,
		s.submitButton,
	)

	// Form layout
	form := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		widget.NewLabel("Project *"),
		s.projectSelect,
		widget.NewLabel("Task"),
		s.taskSelect,
		widget.NewLabel("Activity *"),
		s.activitySelect,
		widget.NewLabel("Description"),
		s.descEntry,
		layout.NewSpacer(),
		s.errorLabel,
		s.loadingBar,
		widget.NewSeparator(),
		buttonRow,
	)

	// Container with border
	formBorder := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	formBorder.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	formBorder.StrokeWidth = 2
	formBorder.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(form)),
	)

	s.Container = container.NewCenter(formContainer)
}

func (s *TimesheetScreen) searchProjects(query string) {
	util.RunAsync(
		func() ([]api.ProjectListItem, error) {
			return s.apiClient.GetProjects(query)
		},
		func(projects []api.ProjectListItem, err error) {
			if err != nil {
				s.showError("Error loading projects: " + err.Error())
				return
			}
			s.projects = projects
			items := make([]widgets.SelectItem, len(projects))
			for i, p := range projects {
				items[i] = widgets.SelectItem{ID: p.ID, Label: p.Label}
			}
			s.projectSelect.SetItems(items)
		},
	)
}

func (s *TimesheetScreen) loadTasks(projectID int) {
	util.RunAsync(
		func() ([]api.TaskListItem, error) {
			return s.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		},
		func(tasks []api.TaskListItem, err error) {
			if err != nil {
				s.showError("Error loading tasks: " + err.Error())
				return
			}
			s.tasks = tasks
			options := make([]string, len(tasks))
			for i, t := range tasks {
				options[i] = t.Label
			}
			s.taskSelect.Options = options
			s.taskSelect.Refresh()
		},
	)
}

func (s *TimesheetScreen) onSubmit() {
	// Validation
	if s.selectedProject == nil {
		s.showError("Please select a project")
		return
	}
	if s.selectedActivity == nil {
		s.showError("Please select an activity")
		return
	}

	s.setLoading(true)
	s.clearError()

	util.RunAsync(
		func() (bool, error) {
			entry := models.TimeEntry{
				ProjectID:   fmt.Sprintf("%d", s.selectedProject.ID),
				ActivityID:  fmt.Sprintf("%d", s.selectedActivity.ID),
				Description: s.descEntry.Text,
				StartTime:   time.Now(),
			}
			if s.selectedTask != nil {
				taskID := fmt.Sprintf("%d", s.selectedTask.ID)
				entry.TaskID = &taskID
			}
			return true, s.apiClient.CreateTimesheet(entry)
		},
		func(_ bool, err error) {
			s.setLoading(false)
			if err != nil {
				s.showError("Error creating timer: " + err.Error())
				return
			}
			if s.OnTimerStarted != nil {
				s.OnTimerStarted()
			}
		},
	)
}

func (s *TimesheetScreen) showError(msg string) {
	s.errorLabel.Text = msg
	s.errorLabel.Refresh()
}

func (s *TimesheetScreen) clearError() {
	s.errorLabel.Text = ""
	s.errorLabel.Refresh()
}

func (s *TimesheetScreen) setLoading(loading bool) {
	if loading {
		s.loadingBar.Show()
		s.submitButton.Disable()
	} else {
		s.loadingBar.Hide()
		s.submitButton.Enable()
	}
}

// Refresh loads initial data
func (s *TimesheetScreen) Refresh() {
	// Load activities
	util.RunAsync(
		func() ([]api.ActivityListItem, error) {
			return s.apiClient.GetActivities()
		},
		func(activities []api.ActivityListItem, err error) {
			if err != nil {
				s.showError("Error loading activities: " + err.Error())
				return
			}
			s.activities = activities
			options := make([]string, len(activities))
			for i, a := range activities {
				options[i] = a.Label
			}
			s.activitySelect.Options = options
			s.activitySelect.Refresh()
		},
	)

	// Initial project search
	s.searchProjects("")
}

// Reset clears the form
func (s *TimesheetScreen) Reset() {
	s.selectedProject = nil
	s.selectedTask = nil
	s.selectedActivity = nil
	s.projectSelect.ClearSelection()
	s.taskSelect.ClearSelected()
	s.activitySelect.ClearSelected()
	s.descEntry.SetText("")
	s.clearError()
}
