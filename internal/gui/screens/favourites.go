package screens

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
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
		colorHex := ""
		if fav != nil {
			colorHex = fav.Color
		}
		idx := i
		s.favButtons[i] = widgets.NewFavouriteButton(slotNum, name, projectName, false, isEmpty, colorHex, func() {
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

	// Main layout - use Border so the grid expands to fill available space
	header := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
	)
	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
		container.NewCenter(s.backButton),
	)
	s.Container = container.NewBorder(header, footer, nil, nil, favGrid)
}

func (s *FavouritesScreen) editFavourite(index int) {
	slotNum := index + 1
	fav := s.favourites.GetAssociation(slotNum)
	if fav == nil {
		fav = &models.FavouriteAssociation{SlotNumber: slotNum}
	}

	preFill := &widgets.EntryFormData{
		ProjectID:    fav.ProjectID,
		ProjectName:  fav.ProjectName,
		TaskID:       fav.TaskID,
		TaskName:     fav.TaskName,
		ActivityID:   fav.ActivityID,
		ActivityName: fav.ActivityName,
		Name:         fav.Name,
		ColorHex:     fav.Color,
	}

	widgets.ShowEntryFormDialog(&widgets.EntryFormConfig{
		Title:           fmt.Sprintf("Edit Favourite %d", slotNum),
		Window:          s.window,
		APIClient:       s.apiClient,
		ShowName:        true,
		ShowTask:        true,
		ShowActivity:    true,
		ShowColorPicker: true,
		PreFill:         preFill,
		Buttons: []widgets.EntryFormButton{
			{
				Label:       "Clear Slot",
				Importance:  widget.DangerImportance,
				LeftAligned: true,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					s.clearFavourite(index)
					close()
				},
			},
			{
				Label: "Cancel",
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					close()
				},
			},
			{
				Label:      "Save",
				Importance: widget.HighImportance,
				OnTapped: func(data *widgets.EntryFormData, setError func(string), close func()) {
					if data.ProjectID == 0 {
						setError("Please select a project")
						return
					}
					if data.ActivityID == 0 {
						setError("Please select an activity")
						return
					}
					s.saveFavouriteSlot(index, data.Name, data.ColorHex,
						&api.ProjectListItem{ID: data.ProjectID, Label: data.ProjectName},
						func() *api.TaskListItem {
							if data.TaskID > 0 {
								return &api.TaskListItem{ID: data.TaskID, Label: data.TaskName}
							}
							return nil
						}(),
						&api.ActivityListItem{ID: data.ActivityID, Label: data.ActivityName},
					)
					close()
				},
			},
		},
	})
}

func (s *FavouritesScreen) saveFavouriteSlot(index int, name, colorHex string, project *api.ProjectListItem, task *api.TaskListItem, activity *api.ActivityListItem) {
	slotNum := index + 1
	fav := models.FavouriteAssociation{
		SlotNumber: slotNum,
		Name:       name,
		Color:      colorHex,
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
		colorHex := ""
		if fav != nil {
			colorHex = fav.Color
		}
		btn.Update(name, projectName, false, isEmpty, colorHex)
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
