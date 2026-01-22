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

// FavouritesScreen represents the favourites editor screen
type FavouritesScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	store     *storage.Storage

	// Widgets
	favButtons  []*widgets.FavouriteButton
	statusLabel *canvas.Text
	backButton  *widget.Button

	// Data
	favourites *models.FavouriteAssociations
	activities []api.ActivityListItem

	// Callbacks
	OnBack func()
}

// NewFavouritesScreen creates a new favourites editor screen
func NewFavouritesScreen(apiClient *api.Client, window fyne.Window) *FavouritesScreen {
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	s := &FavouritesScreen{
		apiClient: apiClient,
		window:    window,
		store:     store,
	}
	s.loadFavourites()
	s.build()
	return s
}

func (s *FavouritesScreen) loadFavourites() {
	s.favourites, _ = s.store.LoadFavouriteAssociations()
	if s.favourites == nil {
		s.favourites = models.NewFavouriteAssociations()
	}
}

func (s *FavouritesScreen) saveFavourites() error {
	return s.store.SaveFavouriteAssociations(s.favourites)
}

func (s *FavouritesScreen) build() {
	// Title
	title := canvas.NewText("Edit Favourites", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Click a slot to configure", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	subtitle.TextSize = 14
	subtitle.Alignment = fyne.TextAlignCenter

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
			s.editFavourite(idx)
		})
	}

	favGrid := widgets.FavouritesGrid(s.favButtons)

	// Status label
	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Back button
	s.backButton = widget.NewButton("← Back to Settings", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Main layout
	s.Container = container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
		layout.NewSpacer(),
		container.NewCenter(favGrid),
		layout.NewSpacer(),
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
		container.NewCenter(s.backButton),
	)
}

func (s *FavouritesScreen) editFavourite(index int) {
	slotNum := index + 1
	fav := s.favourites.GetAssociation(slotNum)
	if fav == nil {
		fav = &models.FavouriteAssociation{SlotNumber: slotNum}
	}

	// Create edit form
	nameEntry := widget.NewEntry()
	nameEntry.SetText(fav.Name)
	nameEntry.SetPlaceHolder("Display name")

	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	if fav.ProjectID > 0 {
		projectSelect.SetSelected(&widgets.SelectItem{ID: fav.ProjectID, Label: fav.ProjectName})
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
		if fav.ActivityID > 0 {
			activitySelect.SetSelected(fav.ActivityName)
		}
	}

	// Load tasks when project changes
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
	if fav.ProjectID > 0 {
		selectedProject = &api.ProjectListItem{ID: fav.ProjectID, Label: fav.ProjectName}
		// Load tasks for existing project
		util.RunAsync(
			func() ([]api.TaskListItem, error) {
				return s.apiClient.GetTasks(fmt.Sprintf("%d", fav.ProjectID))
			},
			func(result []api.TaskListItem, err error) {
				if err == nil {
					tasks = result
					options := make([]string, len(tasks))
					for i, t := range tasks {
						options[i] = t.Label
					}
					taskSelect.Options = options
					if fav.TaskID > 0 {
						taskSelect.SetSelected(fav.TaskName)
					}
					taskSelect.Refresh()
				}
			},
		)
	}

	if fav.ActivityID > 0 {
		selectedActivity = &api.ActivityListItem{ID: fav.ActivityID, Label: fav.ActivityName}
	}

	// Form items
	formItems := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Project", projectSelect),
		widget.NewFormItem("Task", taskSelect),
		widget.NewFormItem("Activity", activitySelect),
	}

	// Clear button callback
	var formDialog dialog.Dialog
	clearBtn := widget.NewButton("Clear Slot", func() {
		s.clearFavourite(index)
		formDialog.Hide()
	})
	clearBtn.Importance = widget.DangerImportance

	formContent := container.NewVBox()
	for _, item := range formItems {
		formContent.Add(widget.NewLabel(item.Text))
		formContent.Add(item.Widget)
	}
	formContent.Add(widget.NewSeparator())
	formContent.Add(clearBtn)

	formDialog = dialog.NewCustomConfirm(
		fmt.Sprintf("Edit Favourite %d", slotNum),
		"Save",
		"Cancel",
		formContent,
		func(confirmed bool) {
			if confirmed {
				s.saveFavouriteSlot(index, nameEntry.Text, selectedProject, selectedTask, selectedActivity)
			}
		},
		s.window,
	)
	formDialog.Resize(fyne.NewSize(400, 400))
	formDialog.Show()
}

func (s *FavouritesScreen) searchProjects(query string, selectWidget *widgets.SearchableSelect) {
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

func (s *FavouritesScreen) saveFavouriteSlot(index int, name string, project *api.ProjectListItem, task *api.TaskListItem, activity *api.ActivityListItem) {
	slotNum := index + 1
	fav := models.FavouriteAssociation{
		SlotNumber: slotNum,
		Name:       name,
	}

	if project != nil {
		fav.ProjectID = project.ID
		fav.ProjectName = project.Label
	}
	if task != nil {
		fav.TaskID = task.ID
		fav.TaskName = task.Label
	}
	if activity != nil {
		fav.ActivityID = activity.ID
		fav.ActivityName = activity.Label
	}

	s.favourites.SetAssociation(slotNum, fav)
	if err := s.saveFavourites(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus(fmt.Sprintf("Favourite %d saved", slotNum))
	s.refreshButtons()
}

func (s *FavouritesScreen) clearFavourite(index int) {
	slotNum := index + 1
	s.favourites.ClearAssociation(slotNum)
	if err := s.saveFavourites(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus(fmt.Sprintf("Favourite %d cleared", slotNum))
	s.refreshButtons()
}

func (s *FavouritesScreen) refreshButtons() {
	for i, btn := range s.favButtons {
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
		btn.Update(name, projectName, false, isEmpty)
	}
}

func (s *FavouritesScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// Refresh reloads data
func (s *FavouritesScreen) Refresh() {
	s.loadFavourites()
	s.refreshButtons()
}
