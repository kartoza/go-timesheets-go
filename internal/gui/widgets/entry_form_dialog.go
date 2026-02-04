package widgets

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
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
)

// EntryFormConfig controls which fields appear and how the dialog behaves.
type EntryFormConfig struct {
	Title     string
	Window    fyne.Window
	APIClient *api.Client

	// Field visibility (all default false)
	ShowName        bool // favourites: custom display name
	ShowDescription bool // dashboard, timesheet, history
	ShowDateTime    bool // dashboard, timesheet, history
	ShowColorPicker bool // favourites: button colour
	ShowRepoURL     bool // code_repos: repository URL field
	ShowTask        bool // all except code_repos
	ShowActivity    bool // all except code_repos

	// Behaviour
	ReadOnly bool // history: disable all fields for submitted entries

	// Pre-fill data (nil = empty form)
	PreFill *EntryFormData

	// Extra content inserted after the standard fields
	ExtraContent fyne.CanvasObject

	// Buttons - caller defines all buttons
	Buttons []EntryFormButton

	// Optional git commit fetcher - if non-nil, a "Fetch from Git" button appears below description
	GitFetcher func(descEntry *widget.Entry)
}

// EntryFormData is both the pre-fill input and the output from the form.
type EntryFormData struct {
	ProjectID    int
	ProjectName  string
	TaskID       int
	TaskName     string
	ActivityID   int
	ActivityName string
	Name         string // favourites display name
	Description  string
	StartDate    string // "YYYY-MM-DD"
	StartTime    string // "HH:MM"
	EndDate      string
	EndTime      string
	ColorHex     string // favourites colour
	RepoURL      string // code_repos URL
}

// EntryFormButton defines a button in the dialog's button row.
type EntryFormButton struct {
	Label      string
	Importance widget.Importance
	// OnTapped receives: current form data, error-setter func, close-dialog func
	OnTapped func(data *EntryFormData, setError func(string), close func())
	// LeftAligned puts the button on the left (for Delete/Clear), default is right
	LeftAligned bool
}

// ShowEntryFormDialog builds the form and displays it as a modal popup.
func ShowEntryFormDialog(cfg *EntryFormConfig) {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	labelGray := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	bgColor := color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}

	// State tracking
	var tasks []api.TaskListItem
	var activities []api.ActivityListItem
	var selectedProject *api.ProjectListItem
	var selectedTask *api.TaskListItem
	var selectedActivity *api.ActivityListItem

	// Pre-fill selected values
	preFill := cfg.PreFill
	if preFill == nil {
		preFill = &EntryFormData{}
	}
	if preFill.ProjectID > 0 {
		selectedProject = &api.ProjectListItem{ID: preFill.ProjectID, Label: preFill.ProjectName}
	}
	if preFill.TaskID > 0 {
		selectedTask = &api.TaskListItem{ID: preFill.TaskID, Label: preFill.TaskName}
	}
	if preFill.ActivityID > 0 {
		selectedActivity = &api.ActivityListItem{ID: preFill.ActivityID, Label: preFill.ActivityName}
	}

	// Title
	titleText := canvas.NewText(cfg.Title, goldColor)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Alignment = fyne.TextAlignCenter

	// Error label
	errorLabel := canvas.NewText("", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF})
	errorLabel.TextSize = 12
	errorLabel.Alignment = fyne.TextAlignCenter
	setError := func(msg string) {
		errorLabel.Text = msg
		errorLabel.Refresh()
	}

	// Build form fields
	formItems := []fyne.CanvasObject{
		container.NewCenter(titleText),
		widget.NewSeparator(),
	}

	// --- Name field (favourites) ---
	var nameEntry *widget.Entry
	if cfg.ShowName {
		nameEntry = widget.NewEntry()
		nameEntry.SetText(preFill.Name)
		nameEntry.SetPlaceHolder("Display name (optional)")
		nameLabel := canvas.NewText("Name", labelGray)
		nameLabel.TextSize = 12
		formItems = append(formItems, nameLabel, nameEntry)
		if cfg.ReadOnly {
			nameEntry.Disable()
		}
	}

	// --- Project picker (always shown) ---
	projectPicker := NewProjectPicker()
	if preFill.ProjectID > 0 {
		projectPicker.SetSelected(&SelectItem{ID: preFill.ProjectID, Label: preFill.ProjectName})
	}
	projectPicker.OnSearch = func(query string) {
		if cfg.APIClient == nil {
			return
		}
		util.RunAsync(
			func() ([]api.ProjectListItem, error) {
				return cfg.APIClient.GetProjects(query)
			},
			func(projects []api.ProjectListItem, err error) {
				if err == nil {
					items := make([]SelectItem, len(projects))
					for i, p := range projects {
						items[i] = SelectItem{ID: p.ID, Label: p.Label}
					}
					projectPicker.SetItems(items)
				}
			},
		)
	}

	// --- Task select ---
	var taskSelect *BoundedSelect
	if cfg.ShowTask {
		taskSelect = NewBoundedSelect("Select task...", []string{}, nil)
		taskSelect.OnChanged = func(selected string) {
			for _, t := range tasks {
				if t.Label == selected {
					selectedTask = &t
					break
				}
			}
		}
	}

	// --- Activity select ---
	var activitySelect *BoundedSelect
	if cfg.ShowActivity {
		activitySelect = NewBoundedSelect("Select activity...", []string{}, nil)
		activitySelect.OnChanged = func(selected string) {
			for _, a := range activities {
				if a.Label == selected {
					selectedActivity = &a
					break
				}
			}
		}
	}

	// Wire project picker OnChanged to load tasks
	projectPicker.OnChanged = func(item *SelectItem) {
		if item != nil {
			selectedProject = &api.ProjectListItem{ID: item.ID, Label: item.Label}
			if cfg.ShowTask && cfg.APIClient != nil {
				util.RunAsync(
					func() ([]api.TaskListItem, error) {
						return cfg.APIClient.GetTasks(fmt.Sprintf("%d", item.ID))
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
			}
		} else {
			selectedProject = nil
			if taskSelect != nil {
				taskSelect.SetOptions([]string{})
			}
		}
	}

	// Use the project picker's height as the reference for consistent row heights
	pickerHeight := projectPicker.Container.MinSize().Height

	projectLabel := canvas.NewText("Project *", labelGray)
	projectLabel.TextSize = 12
	formItems = append(formItems, projectLabel, projectPicker.Container)

	if cfg.ShowTask {
		taskLabel := canvas.NewText("Task", labelGray)
		taskLabel.TextSize = 12
		taskSpacer := canvas.NewRectangle(color.Transparent)
		taskSpacer.SetMinSize(fyne.NewSize(0, pickerHeight))
		taskRow := container.NewStack(taskSpacer, taskSelect.Select)
		formItems = append(formItems, taskLabel, taskRow)
	}

	if cfg.ShowActivity {
		activityLabel := canvas.NewText("Activity *", labelGray)
		activityLabel.TextSize = 12
		actSpacer := canvas.NewRectangle(color.Transparent)
		actSpacer.SetMinSize(fyne.NewSize(0, pickerHeight))
		activityRow := container.NewStack(actSpacer, activitySelect.Select)
		formItems = append(formItems, activityLabel, activityRow)
	}

	// --- Description field ---
	var descEntry *widget.Entry
	var descSection fyne.CanvasObject // will be placed in expanding center
	if cfg.ShowDescription {
		descEntry = widget.NewMultiLineEntry()
		descEntry.SetPlaceHolder("Description (optional)")
		descEntry.SetText(preFill.Description)
		descEntry.SetMinRowsVisible(3)

		descLabel := widget.NewLabel("Description")

		if cfg.GitFetcher != nil {
			fetchButton := widget.NewButton("Fetch from Git", func() {
				cfg.GitFetcher(descEntry)
			})
			if cfg.ReadOnly {
				fetchButton.Disable()
			}
			descSection = container.NewBorder(
				descLabel,
				container.NewHBox(layout.NewSpacer(), fetchButton),
				nil, nil,
				descEntry,
			)
		} else {
			descSection = container.NewBorder(
				descLabel,
				nil, nil, nil,
				descEntry,
			)
		}

		if cfg.ReadOnly {
			descEntry.Disable()
		}
	}

	// --- Repo URL field (code repos) ---
	var repoEntry *widget.Entry
	if cfg.ShowRepoURL {
		repoEntry = widget.NewEntry()
		repoEntry.SetPlaceHolder("github.com/owner/repo or /path/to/local/repo")
		repoEntry.SetText(preFill.RepoURL)
		repoLabel := canvas.NewText("Repository URL *", labelGray)
		repoLabel.TextSize = 12
		formItems = append(formItems, repoLabel, repoEntry)
		if cfg.ReadOnly {
			repoEntry.Disable()
		}
	}

	// --- Bottom section: date/time, color picker, extra content, buttons ---
	var bottomItems []fyne.CanvasObject

	// --- Date/time fields ---
	var startDateEntry, startTimeEntry, endDateEntry, endTimeEntry *widget.Entry
	if cfg.ShowDateTime {
		startDateEntry = widget.NewEntry()
		startDateEntry.SetPlaceHolder("YYYY-MM-DD")
		startDateEntry.SetText(preFill.StartDate)

		startTimeEntry = widget.NewEntry()
		startTimeEntry.SetPlaceHolder("HH:MM")
		startTimeEntry.SetText(preFill.StartTime)

		endDateEntry = widget.NewEntry()
		endDateEntry.SetPlaceHolder("YYYY-MM-DD")
		endDateEntry.SetText(preFill.EndDate)

		endTimeEntry = widget.NewEntry()
		endTimeEntry.SetPlaceHolder("HH:MM (optional)")
		endTimeEntry.SetText(preFill.EndTime)

		dateTimeRow := container.NewGridWithColumns(4,
			container.NewVBox(widget.NewLabel("Start Date"), startDateEntry),
			container.NewVBox(widget.NewLabel("Start Time"), startTimeEntry),
			container.NewVBox(widget.NewLabel("End Date"), endDateEntry),
			container.NewVBox(widget.NewLabel("End Time"), endTimeEntry),
		)
		bottomItems = append(bottomItems, dateTimeRow)

		if cfg.ReadOnly {
			startDateEntry.Disable()
			startTimeEntry.Disable()
			endDateEntry.Disable()
			endTimeEntry.Disable()
		}
	}

	// --- Color picker (favourites) ---
	var colorPicker *ColorPicker
	if cfg.ShowColorPicker {
		selectedColor := preFill.ColorHex
		colorLabel := canvas.NewText("Button Colour", labelGray)
		colorLabel.TextSize = 12
		colorPicker = NewColorPicker(preFill.ColorHex, func(hex string) {
			selectedColor = hex
			_ = selectedColor // used in collectData
		})
		bottomItems = append(bottomItems, widget.NewSeparator(), colorLabel, colorPicker)
	}

	// --- Extra content ---
	if cfg.ExtraContent != nil {
		bottomItems = append(bottomItems, cfg.ExtraContent)
	}

	// --- collectData gathers current form values into EntryFormData ---
	collectData := func() *EntryFormData {
		data := &EntryFormData{}
		if selectedProject != nil {
			data.ProjectID = selectedProject.ID
			data.ProjectName = selectedProject.Label
		}
		if selectedTask != nil {
			data.TaskID = selectedTask.ID
			data.TaskName = selectedTask.Label
		}
		if selectedActivity != nil {
			data.ActivityID = selectedActivity.ID
			data.ActivityName = selectedActivity.Label
		}
		if nameEntry != nil {
			data.Name = nameEntry.Text
		}
		if descEntry != nil {
			data.Description = descEntry.Text
		}
		if startDateEntry != nil {
			data.StartDate = startDateEntry.Text
		}
		if startTimeEntry != nil {
			data.StartTime = startTimeEntry.Text
		}
		if endDateEntry != nil {
			data.EndDate = endDateEntry.Text
		}
		if endTimeEntry != nil {
			data.EndTime = endTimeEntry.Text
		}
		if colorPicker != nil {
			data.ColorHex = colorPicker.Selected
		}
		if repoEntry != nil {
			data.RepoURL = repoEntry.Text
		}
		return data
	}

	// --- Button row ---
	var popup *widget.PopUp

	closeDialog := func() {
		if popup != nil {
			popup.Hide()
		}
	}

	leftButtons := container.NewHBox()
	rightButtons := container.NewHBox()

	for _, btn := range cfg.Buttons {
		btn := btn // capture
		b := widget.NewButton(btn.Label, func() {
			data := collectData()
			btn.OnTapped(data, setError, closeDialog)
		})
		b.Importance = btn.Importance
		if btn.LeftAligned {
			leftButtons.Add(b)
		} else {
			rightButtons.Add(b)
		}
	}

	buttonRow := container.NewHBox()
	buttonRow.Add(leftButtons)
	buttonRow.Add(layout.NewSpacer())
	buttonRow.Add(rightButtons)

	// Assemble bottom section: extra items + error + buttons
	bottomItems = append(bottomItems, errorLabel, widget.NewSeparator(), buttonRow)
	bottomSection := container.NewVBox(bottomItems...)

	// Top section: title, separator, name, project, task, activity
	topSection := container.NewVBox(formItems...)

	// Assemble form using Border layout so description expands to fill space
	var formContent fyne.CanvasObject
	if descSection != nil {
		formContent = container.NewBorder(
			topSection,
			bottomSection,
			nil, nil,
			descSection,
		)
	} else {
		// No description: just stack top + bottom with spacer between
		topSection.Add(layout.NewSpacer())
		topSection.Add(bottomSection)
		formContent = topSection
	}

	// Styled container
	formBorder := canvas.NewRectangle(goldColor)
	formBorder.StrokeColor = goldColor
	formBorder.StrokeWidth = 2
	formBorder.FillColor = bgColor
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(formContent)),
	)

	// Show as modal popup
	popup = widget.NewModalPopUp(formContainer, cfg.Window.Canvas())
	popup.Resize(fyne.NewSize(550, 650))
	popup.Show()

	// Disable project picker for read-only mode
	if cfg.ReadOnly {
		projectPicker.Disable()
	}

	// --- Async data loading ---

	// Load tasks for pre-selected project
	if preFill.ProjectID > 0 && cfg.ShowTask && cfg.APIClient != nil {
		util.RunAsync(
			func() ([]api.TaskListItem, error) {
				return cfg.APIClient.GetTasks(fmt.Sprintf("%d", preFill.ProjectID))
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
				if preFill.TaskID > 0 {
					taskSelect.SetSelected(preFill.TaskName)
				}
				if cfg.ReadOnly {
					taskSelect.Disable()
				}
			},
		)
	}

	// Load activities
	if cfg.ShowActivity && cfg.APIClient != nil {
		util.RunAsync(
			func() ([]api.ActivityListItem, error) {
				return cfg.APIClient.GetActivities()
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
				if preFill.ActivityID > 0 {
					activitySelect.SetSelected(preFill.ActivityName)
				}
				if cfg.ReadOnly {
					activitySelect.Disable()
				}
			},
		)
	}
}
