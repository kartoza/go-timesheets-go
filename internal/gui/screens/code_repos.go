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

// CodeReposScreen represents the code repositories screen
type CodeReposScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	store     *storage.Storage

	// Widgets
	repoList    *widget.List
	addButton   *widget.Button
	statusLabel *canvas.Text
	backButton  *widget.Button

	// Data
	associations *models.CodeRepoAssociations

	// Callbacks
	OnBack func()
}

// NewCodeReposScreen creates a new code repos screen
func NewCodeReposScreen(apiClient *api.Client, window fyne.Window) *CodeReposScreen {
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	s := &CodeReposScreen{
		apiClient: apiClient,
		window:    window,
		store:     store,
	}
	s.loadAssociations()
	s.build()
	return s
}

func (s *CodeReposScreen) loadAssociations() {
	s.associations, _ = s.store.LoadCodeRepoAssociations()
	if s.associations == nil {
		s.associations = models.NewCodeRepoAssociations()
	}
}

func (s *CodeReposScreen) saveAssociations() error {
	return s.store.SaveCodeRepoAssociations(s.associations)
}

func (s *CodeReposScreen) build() {
	// Title
	title := canvas.NewText("Code Repositories", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Link projects to git repositories", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	subtitle.TextSize = 14
	subtitle.Alignment = fyne.TextAlignCenter

	// Add button
	s.addButton = widget.NewButton("+ Add Repository", s.addRepo)
	s.addButton.Importance = widget.HighImportance

	// Repository list
	s.repoList = widget.NewList(
		func() int { return len(s.associations.Associations) },
		func() fyne.CanvasObject {
			return s.createRepoRow()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			s.updateRepoRow(int(id), obj)
		},
	)
	s.repoList.OnSelected = func(id widget.ListItemID) {
		s.editRepo(int(id))
		s.repoList.UnselectAll()
	}

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
	s.Container = container.NewBorder(
		container.NewVBox(
			container.NewCenter(title),
			container.NewCenter(subtitle),
			widget.NewSeparator(),
			container.NewCenter(s.addButton),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewCenter(s.statusLabel),
			container.NewCenter(s.backButton),
		),
		nil,
		nil,
		s.repoList,
	)
}

// repoRow holds references to the widgets in a repository list row
type repoRow struct {
	projectLabel *widget.Label
	repoLabel    *widget.Label
	editBtn      *widget.Button
	deleteBtn    *widget.Button
	container    fyne.CanvasObject
}

func (s *CodeReposScreen) createRepoRow() fyne.CanvasObject {
	r := &repoRow{}
	r.projectLabel = widget.NewLabel("Project")
	r.projectLabel.TextStyle = fyne.TextStyle{Bold: true}
	r.projectLabel.Truncation = fyne.TextTruncateEllipsis

	r.repoLabel = widget.NewLabel("Repository")
	r.repoLabel.Truncation = fyne.TextTruncateEllipsis

	r.editBtn = widget.NewButton("Edit", nil)
	r.deleteBtn = widget.NewButton("Delete", nil)
	r.deleteBtn.Importance = widget.DangerImportance

	buttons := container.NewHBox(r.editBtn, r.deleteBtn)
	r.container = container.NewBorder(nil, nil, r.projectLabel, buttons, r.repoLabel)
	return r.container
}

func (s *CodeReposScreen) updateRepoRow(index int, obj fyne.CanvasObject) {
	if index >= len(s.associations.Associations) {
		return
	}

	assoc := s.associations.Associations[index]

	// Walk the object tree to find our widgets by type
	projectLabel, repoLabel, editBtn, deleteBtn := s.findRepoRowWidgets(obj)
	if projectLabel == nil {
		return
	}

	projectLabel.SetText(assoc.ProjectName)
	repoLabel.SetText(assoc.RepoURL)

	editBtn.OnTapped = func() {
		s.editRepo(index)
	}
	deleteBtn.OnTapped = func() {
		s.confirmDelete(index)
	}
}

// findRepoRowWidgets extracts the widgets from a repo row container
func (s *CodeReposScreen) findRepoRowWidgets(obj fyne.CanvasObject) (projectLabel, repoLabel *widget.Label, editBtn, deleteBtn *widget.Button) {
	c, ok := obj.(*fyne.Container)
	if !ok {
		return
	}

	var labels []*widget.Label
	var buttons []*widget.Button

	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok {
			labels = append(labels, l)
			return
		}
		if b, ok := o.(*widget.Button); ok {
			buttons = append(buttons, b)
			return
		}
		if cc, ok := o.(*fyne.Container); ok {
			for _, child := range cc.Objects {
				walk(child)
			}
		}
	}
	for _, child := range c.Objects {
		walk(child)
	}

	if len(labels) >= 2 && len(buttons) >= 2 {
		projectLabel = labels[0]
		repoLabel = labels[1]
		editBtn = buttons[0]
		deleteBtn = buttons[1]
	}
	return
}

func (s *CodeReposScreen) addRepo() {
	s.showRepoDialog(-1, nil)
}

func (s *CodeReposScreen) editRepo(index int) {
	if index >= len(s.associations.Associations) {
		return
	}
	assoc := &s.associations.Associations[index]
	s.showRepoDialog(index, assoc)
}

func (s *CodeReposScreen) showRepoDialog(index int, existing *models.CodeRepoAssociation) {
	isNew := index < 0

	// Colors matching timesheet dialog
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	labelGray := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	bgColor := color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}

	titleStr := "Add Repository"
	if !isNew {
		titleStr = "Edit Repository"
	}

	titleText := canvas.NewText(titleStr, goldColor)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Alignment = fyne.TextAlignCenter

	projectSelect := widgets.NewSearchableSelect("Search projects...", s.window)
	if existing != nil && existing.ProjectID > 0 {
		projectSelect.SetSelected(&widgets.SelectItem{ID: existing.ProjectID, Label: existing.ProjectName})
	}
	projectSelect.OnSearch = func(query string) {
		s.searchProjects(query, projectSelect)
	}

	repoEntry := widget.NewEntry()
	repoEntry.SetPlaceHolder("github.com/owner/repo")
	if existing != nil {
		repoEntry.SetText(existing.RepoURL)
	}

	var selectedProject *api.ProjectListItem
	projectSelect.OnChanged = func(item *widgets.SelectItem) {
		if item != nil {
			selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
		}
	}

	if existing != nil && existing.ProjectID > 0 {
		selectedProject = &api.ProjectListItem{ID: existing.ProjectID, Label: existing.ProjectName}
	}

	// Labels
	projectLabel := canvas.NewText("Project *", labelGray)
	projectLabel.TextSize = 12
	repoLabel := canvas.NewText("Repository URL *", labelGray)
	repoLabel.TextSize = 12

	// Buttons
	var customDialog *widget.PopUp

	saveBtn := widget.NewButton("Save", func() {
		if selectedProject == nil {
			s.setStatus("Please select a project")
			return
		}
		if repoEntry.Text == "" {
			s.setStatus("Please enter a repository URL")
			return
		}
		s.saveRepo(index, selectedProject, repoEntry.Text)
		if customDialog != nil {
			customDialog.Hide()
		}
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		if customDialog != nil {
			customDialog.Hide()
		}
	})

	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		cancelBtn,
		saveBtn,
	)

	// Form layout
	formContent := container.NewVBox(
		container.NewCenter(titleText),
		widget.NewSeparator(),
		projectLabel,
		projectSelect,
		repoLabel,
		repoEntry,
		layout.NewSpacer(),
		widget.NewSeparator(),
		buttonRow,
	)

	// Styled container with border
	formBorder := canvas.NewRectangle(goldColor)
	formBorder.StrokeColor = goldColor
	formBorder.StrokeWidth = 2
	formBorder.FillColor = bgColor
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(formContent)),
	)

	scrollContent := container.NewVScroll(formContainer)
	scrollContent.SetMinSize(fyne.NewSize(380, 320))

	popupBg := canvas.NewRectangle(darkGray)
	popupBg.SetMinSize(fyne.NewSize(400, 350))

	popupContent := container.NewStack(
		popupBg,
		container.NewPadded(scrollContent),
	)

	customDialog = widget.NewModalPopUp(popupContent, s.window.Canvas())
	customDialog.Show()
}

func (s *CodeReposScreen) searchProjects(query string, selectWidget *widgets.SearchableSelect) {
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

func (s *CodeReposScreen) saveRepo(index int, project *api.ProjectListItem, repoURL string) {
	assoc := models.CodeRepoAssociation{
		ProjectID:   project.ID,
		ProjectName: project.Label,
		RepoURL:     repoURL,
	}

	if index < 0 {
		// New association
		s.associations.Associations = append(s.associations.Associations, assoc)
	} else {
		// Update existing
		s.associations.Associations[index] = assoc
	}

	if err := s.saveAssociations(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus("Repository saved")
	s.repoList.Refresh()
}

func (s *CodeReposScreen) confirmDelete(index int) {
	if index >= len(s.associations.Associations) {
		return
	}

	assoc := s.associations.Associations[index]

	dialog.ShowConfirm(
		"Confirm Delete",
		fmt.Sprintf("Delete repo link for \"%s\"?", assoc.ProjectName),
		func(confirmed bool) {
			if confirmed {
				s.deleteRepo(index)
			}
		},
		s.window,
	)
}

func (s *CodeReposScreen) deleteRepo(index int) {
	if index >= len(s.associations.Associations) {
		return
	}

	// Remove from slice
	s.associations.Associations = append(
		s.associations.Associations[:index],
		s.associations.Associations[index+1:]...,
	)

	if err := s.saveAssociations(); err != nil {
		s.setStatus("Error saving: " + err.Error())
		return
	}

	s.setStatus("Repository deleted")
	s.repoList.Refresh()
}

func (s *CodeReposScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// Refresh reloads data
func (s *CodeReposScreen) Refresh() {
	s.loadAssociations()
	s.repoList.Refresh()
}
