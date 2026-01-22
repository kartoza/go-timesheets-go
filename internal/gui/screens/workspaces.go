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
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/gui/widgets"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// WorkspacesScreen represents the workspace associations screen
type WorkspacesScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	store     *storage.Storage

	// Widgets
	workspaceList *widget.List
	statusLabel   *canvas.Text
	backButton    *widget.Button

	// Data
	associations *models.WorkspaceAssociations
	activities   []api.ActivityListItem

	// Callbacks
	OnBack func()
}

// NewWorkspacesScreen creates a new workspaces screen
func NewWorkspacesScreen(apiClient *api.Client, window fyne.Window) *WorkspacesScreen {
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	s := &WorkspacesScreen{
		apiClient: apiClient,
		window:    window,
		store:     store,
	}
	s.loadAssociations()
	s.build()
	return s
}

func (s *WorkspacesScreen) loadAssociations() {
	s.associations, _ = s.store.LoadWorkspaceAssociations()
	if s.associations == nil {
		s.associations = models.NewWorkspaceAssociations()
	}
}

func (s *WorkspacesScreen) saveAssociations() error {
	return s.store.SaveWorkspaceAssociations(s.associations)
}

func (s *WorkspacesScreen) build() {
	// Title
	title := canvas.NewText("Workspace Associations", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Map virtual desktops to projects", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	subtitle.TextSize = 14
	subtitle.Alignment = fyne.TextAlignCenter

	// Workspace list
	s.workspaceList = widget.NewList(
		func() int { return 10 },
		func() fyne.CanvasObject {
			return s.createWorkspaceRow()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			s.updateWorkspaceRow(int(id), obj)
		},
	)
	s.workspaceList.OnSelected = func(id widget.ListItemID) {
		s.editWorkspace(int(id))
		s.workspaceList.UnselectAll()
	}

	// Status label
	s.statusLabel = canvas.NewText("Click a workspace to edit", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Back button
	s.backButton = widget.NewButton("← Back to Settings", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Main layout
	s.Container = container.NewBorder(
		container.NewVBox(
			container.NewCenter(title),
			container.NewCenter(subtitle),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewCenter(s.statusLabel),
			container.NewCenter(s.backButton),
		),
		nil,
		nil,
		s.workspaceList,
	)
}

func (s *WorkspacesScreen) createWorkspaceRow() fyne.CanvasObject {
	numLabel := widget.NewLabel("#")
	numLabel.TextStyle = fyne.TextStyle{Bold: true}

	nameLabel := widget.NewLabel("Name")
	assocLabel := widget.NewLabel("Association")
	assocLabel.Wrapping = fyne.TextWrapWord

	editBtn := widget.NewButton("Edit", nil)
	clearBtn := widget.NewButton("Clear", nil)
	clearBtn.Importance = widget.DangerImportance

	return container.NewHBox(
		numLabel,
		nameLabel,
		layout.NewSpacer(),
		assocLabel,
		editBtn,
		clearBtn,
	)
}

func (s *WorkspacesScreen) updateWorkspaceRow(index int, obj fyne.CanvasObject) {
	row := obj.(*fyne.Container)
	assoc := s.associations.Associations[index]

	row.Objects[0].(*widget.Label).SetText(fmt.Sprintf("%d", index+1))

	name := assoc.WorkspaceName
	if name == "" {
		name = fmt.Sprintf("Workspace %d", index+1)
	}
	row.Objects[1].(*widget.Label).SetText(name)

	assocText := ""
	if assoc.ProjectID > 0 {
		assocText = assoc.ProjectName
		if assoc.TaskName != "" {
			assocText += " / " + assoc.TaskName
		}
	} else {
		assocText = "(Not configured)"
	}
	row.Objects[3].(*widget.Label).SetText(assocText)

	// Edit button
	row.Objects[4].(*widget.Button).OnTapped = func() {
		s.editWorkspace(index)
	}

	// Clear button
	row.Objects[5].(*widget.Button).OnTapped = func() {
		s.confirmClear(index)
	}
}

func (s *WorkspacesScreen) editWorkspace(index int) {
	assoc := s.associations.Associations[index]

	// Create edit form
	nameEntry := widget.NewEntry()
	nameEntry.SetText(assoc.WorkspaceName)
	nameEntry.SetPlaceHolder(fmt.Sprintf("Workspace %d", index+1))

	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	if assoc.ProjectID > 0 {
		projectSelect.SetSelected(&widgets.SelectItem{ID: assoc.ProjectID, Label: assoc.ProjectName})
	}
	projectSelect.OnSearch = func(query string) {
		s.searchProjects(query, projectSelect)
	}

	taskSelect := widget.NewSelect([]string{}, nil)
	taskSelect.PlaceHolder = "Select task..."

	activitySelect := widget.NewSelect([]string{}, nil)
	activitySelect.PlaceHolder = "Select activity..."

	// Load activities
	if len(s.activities) == 0 {
		util.RunAsync(
			func() ([]api.ActivityListItem, error) {
				return s.apiClient.GetActivities()
			},
			func(activities []api.ActivityListItem, err error) {
				if err == nil {
					s.activities = activities
					options := make([]string, len(activities))
					for i, a := range activities {
						options[i] = a.Label
					}
					activitySelect.Options = options
					if assoc.ActivityID > 0 {
						activitySelect.SetSelected(assoc.ActivityName)
					}
					activitySelect.Refresh()
				}
			},
		)
	} else {
		options := make([]string, len(s.activities))
		for i, a := range s.activities {
			options[i] = a.Label
		}
		activitySelect.Options = options
		if assoc.ActivityID > 0 {
			activitySelect.SetSelected(assoc.ActivityName)
		}
	}

	var selectedProject *api.ProjectListItem
	var selectedTask *api.TaskListItem
	var selectedActivity *api.ActivityListItem
	var tasks []api.TaskListItem

	projectSelect.OnChanged = func(item *widgets.SelectItem) {
		if item != nil {
			selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
			util.RunAsync(
				func() ([]api.TaskListItem, error) {
					return s.apiClient.GetTasks(fmt.Sprintf("%d", item.ID))
				},
				func(result []api.TaskListItem, err error) {
					if err == nil {
						tasks = result
						options := make([]string, len(tasks))
						for i, t := range tasks {
							options[i] = t.Label
						}
						taskSelect.Options = options
						taskSelect.Refresh()
					}
				},
			)
		}
	}

	taskSelect.OnChanged = func(selected string) {
		for _, t := range tasks {
			if t.Label == selected {
				selectedTask = &t
				break
			}
		}
	}

	activitySelect.OnChanged = func(selected string) {
		for _, a := range s.activities {
			if a.Label == selected {
				selectedActivity = &a
				break
			}
		}
	}

	// Pre-select existing values
	if assoc.ProjectID > 0 {
		selectedProject = &api.ProjectListItem{ID: assoc.ProjectID, Label: assoc.ProjectName}
		util.RunAsync(
			func() ([]api.TaskListItem, error) {
				return s.apiClient.GetTasks(fmt.Sprintf("%d", assoc.ProjectID))
			},
			func(result []api.TaskListItem, err error) {
				if err == nil {
					tasks = result
					options := make([]string, len(tasks))
					for i, t := range tasks {
						options[i] = t.Label
					}
					taskSelect.Options = options
					if assoc.TaskID > 0 {
						taskSelect.SetSelected(assoc.TaskName)
					}
					taskSelect.Refresh()
				}
			},
		)
	}

	if assoc.ActivityID > 0 {
		selectedActivity = &api.ActivityListItem{ID: assoc.ActivityID, Label: assoc.ActivityName}
	}

	formContent := container.NewVBox(
		widget.NewLabel("Name"),
		nameEntry,
		widget.NewLabel("Project"),
		projectSelect,
		widget.NewLabel("Task"),
		taskSelect,
		widget.NewLabel("Activity"),
		activitySelect,
	)

	d := dialog.NewCustomConfirm(
		fmt.Sprintf("Edit Workspace %d", index+1),
		"Save",
		"Cancel",
		formContent,
		func(confirmed bool) {
			if confirmed {
				s.saveWorkspace(index, nameEntry.Text, selectedProject, selectedTask, selectedActivity)
			}
		},
		s.window,
	)
	d.Resize(fyne.NewSize(400, 400))
	d.Show()
}

func (s *WorkspacesScreen) searchProjects(query string, selectWidget *widgets.SearchableSelect) {
	util.RunAsync(
		func() ([]api.ProjectListItem, error) {
			return s.apiClient.GetProjects(query)
		},
		func(projects []api.ProjectListItem, err error) {
			if err == nil {
				items := make([]widgets.SelectItem, len(projects))
				for i, p := range projects {
					items[i] = widgets.SelectItem{ID: p.ID, Label: p.Label}
				}
				selectWidget.SetItems(items)
			}
		},
	)
}

func (s *WorkspacesScreen) saveWorkspace(index int, name string, project *api.ProjectListItem, task *api.TaskListItem, activity *api.ActivityListItem) {
	assoc := &s.associations.Associations[index]
	assoc.WorkspaceName = name

	if project != nil {
		assoc.ProjectID = project.ID
		assoc.ProjectName = project.Label
	}
	if task != nil {
		assoc.TaskID = task.ID
		assoc.TaskName = task.Label
	}
	if activity != nil {
		assoc.ActivityID = activity.ID
		assoc.ActivityName = activity.Label
	}

	if err := s.saveAssociations(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus(fmt.Sprintf("Workspace %d saved", index+1))
	s.workspaceList.Refresh()
}

func (s *WorkspacesScreen) confirmClear(index int) {
	assoc := s.associations.Associations[index]
	if assoc.ProjectID == 0 {
		s.setStatus("Workspace is already empty")
		return
	}

	name := assoc.WorkspaceName
	if name == "" {
		name = fmt.Sprintf("Workspace %d", index+1)
	}

	dialog.ShowConfirm(
		"Confirm Clear",
		fmt.Sprintf("Clear association for \"%s\"?", name),
		func(confirmed bool) {
			if confirmed {
				s.clearWorkspace(index)
			}
		},
		s.window,
	)
}

func (s *WorkspacesScreen) clearWorkspace(index int) {
	s.associations.ClearAssociation(index + 1)
	if err := s.saveAssociations(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus(fmt.Sprintf("Workspace %d cleared", index+1))
	s.workspaceList.Refresh()
}

func (s *WorkspacesScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// Refresh reloads data
func (s *WorkspacesScreen) Refresh() {
	s.loadAssociations()
	s.workspaceList.Refresh()
}
