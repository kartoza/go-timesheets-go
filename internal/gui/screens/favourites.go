package screens

import (
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

	// Colors matching timesheet dialog
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	labelGray := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	bgColor := color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}

	// Title
	titleText := canvas.NewText(fmt.Sprintf("Edit Favourite %d", slotNum), goldColor)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Alignment = fyne.TextAlignCenter

	// Name entry
	nameEntry := widget.NewEntry()
	nameEntry.SetText(fav.Name)
	nameEntry.SetPlaceHolder("Display name (optional)")

	// Project select (searchable)
	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	if fav.ProjectID > 0 {
		projectSelect.SetSelected(&widgets.SelectItem{ID: fav.ProjectID, Label: fav.ProjectName})
	}
	projectSelect.OnSearch = func(query string) {
		s.searchProjects(query, projectSelect)
	}

	// Task select (bounded height)
	taskSelect := widgets.NewBoundedSelect("Select task...", []string{}, s.window)

	// Activity select (bounded height)
	activitySelect := widgets.NewBoundedSelect("Select activity...", []string{}, s.window)

	// State tracking
	var selectedProject *api.ProjectListItem
	var selectedTask *api.TaskListItem
	var selectedActivity *api.ActivityListItem
	var tasks []api.TaskListItem

	// Load activities
	loadActivities := func() {
		if len(s.activities) == 0 {
			util.RunAsync(
				func() ([]api.ActivityListItem, error) {
					return s.apiClient.GetActivities()
				},
				func(activities []api.ActivityListItem, err error) {
					if err == nil {
						// Sort activities alphabetically
						sort.Slice(activities, func(i, j int) bool {
							return activities[i].Label < activities[j].Label
						})
						s.activities = activities
						options := make([]string, len(activities))
						for i, a := range activities {
							options[i] = a.Label
						}
						activitySelect.SetOptions(options)
						// Pre-select if we have an existing value
						if fav.ActivityID > 0 {
							activitySelect.SetSelected(fav.ActivityName)
							selectedActivity = &api.ActivityListItem{ID: fav.ActivityID, Label: fav.ActivityName}
						}
					}
				},
			)
		} else {
			options := make([]string, len(s.activities))
			for i, a := range s.activities {
				options[i] = a.Label
			}
			activitySelect.SetOptions(options)
			if fav.ActivityID > 0 {
				activitySelect.SetSelected(fav.ActivityName)
				selectedActivity = &api.ActivityListItem{ID: fav.ActivityID, Label: fav.ActivityName}
			}
		}
	}

	// Project change handler
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
						taskSelect.SetOptions(options)
					}
				},
			)
		} else {
			selectedProject = nil
			taskSelect.SetOptions([]string{})
		}
	}

	// Task change handler
	taskSelect.OnChanged = func(selected string) {
		for _, t := range tasks {
			if t.Label == selected {
				selectedTask = &t
				break
			}
		}
	}

	// Activity change handler
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
					taskSelect.SetOptions(options)
					if fav.TaskID > 0 {
						taskSelect.SetSelected(fav.TaskName)
						selectedTask = &api.TaskListItem{ID: fav.TaskID, Label: fav.TaskName}
					}
				}
			},
		)
	}

	// Load activities
	loadActivities()

	// Labels with consistent styling
	nameLabel := canvas.NewText("Name", labelGray)
	nameLabel.TextSize = 12
	projectLabel := canvas.NewText("Project *", labelGray)
	projectLabel.TextSize = 12
	taskLabel := canvas.NewText("Task", labelGray)
	taskLabel.TextSize = 12
	activityLabel := canvas.NewText("Activity *", labelGray)
	activityLabel.TextSize = 12

	// Clear button
	var customDialog *widget.PopUp
	clearBtn := widget.NewButton("Clear Slot", func() {
		s.clearFavourite(index)
		if customDialog != nil {
			customDialog.Hide()
		}
	})
	clearBtn.Importance = widget.DangerImportance

	// Save button
	saveBtn := widget.NewButton("Save", func() {
		// Validation
		if selectedProject == nil {
			s.setStatus("Please select a project")
			return
		}
		if selectedActivity == nil {
			s.setStatus("Please select an activity")
			return
		}
		s.saveFavouriteSlot(index, nameEntry.Text, selectedProject, selectedTask, selectedActivity)
		if customDialog != nil {
			customDialog.Hide()
		}
	})
	saveBtn.Importance = widget.HighImportance

	// Cancel button
	cancelBtn := widget.NewButton("Cancel", func() {
		if customDialog != nil {
			customDialog.Hide()
		}
	})

	// Button row
	buttonRow := container.NewHBox(
		clearBtn,
		layout.NewSpacer(),
		cancelBtn,
		saveBtn,
	)

	// Form layout
	formContent := container.NewVBox(
		container.NewCenter(titleText),
		widget.NewSeparator(),
		nameLabel,
		nameEntry,
		projectLabel,
		projectSelect,
		taskLabel,
		taskSelect,
		activityLabel,
		activitySelect,
		layout.NewSpacer(),
		widget.NewSeparator(),
		buttonRow,
	)

	// Container with border (matching timesheet styling)
	formBorder := canvas.NewRectangle(goldColor)
	formBorder.StrokeColor = goldColor
	formBorder.StrokeWidth = 2
	formBorder.FillColor = bgColor
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(formContent)),
	)

	// Wrap in scroll for safety
	scrollContent := container.NewVScroll(formContainer)
	scrollContent.SetMinSize(fyne.NewSize(380, 450))

	// Create custom popup with dark background
	popupBg := canvas.NewRectangle(darkGray)
	popupBg.SetMinSize(fyne.NewSize(400, 480))

	popupContent := container.NewStack(
		popupBg,
		container.NewPadded(scrollContent),
	)

	customDialog = widget.NewModalPopUp(popupContent, s.window.Canvas())
	customDialog.Show()
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
