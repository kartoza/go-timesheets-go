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

	"github.com/kartoza/go-timesheets-go/internal/ai"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

// SettingsScreen represents the settings screen
type SettingsScreen struct {
	Container fyne.CanvasObject

	window fyne.Window

	// Status label for transient feedback (e.g. "Syncing projects…").
	statusLabel *canvas.Text

	// Callbacks
	OnBack             func()
	OnEditFavourites   func()
	OnCodeRepos        func()
	OnCalendarSettings func()
	OnRefreshProjects  func(done func(err error)) // async; call done(err) when finished
	OnLogout           func()
}

// NewSettingsScreen creates a new settings screen
func NewSettingsScreen(window fyne.Window) *SettingsScreen {
	s := &SettingsScreen{
		window: window,
	}
	s.build()
	return s
}

func (s *SettingsScreen) build() {
	// Title
	title := canvas.NewText("Settings", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Menu items
	favBtn := widget.NewButton("Edit Favourites", func() {
		if s.OnEditFavourites != nil {
			s.OnEditFavourites()
		}
	})
	favBtn.Importance = widget.MediumImportance

	codeReposBtn := widget.NewButton("Code Repositories", func() {
		if s.OnCodeRepos != nil {
			s.OnCodeRepos()
		}
	})

	calendarBtn := widget.NewButton("Google Calendar", func() {
		if s.OnCalendarSettings != nil {
			s.OnCalendarSettings()
		}
	})

	refreshProjectsBtn := widget.NewButton("Sync Projects from ERPNext", s.onRefreshProjects)

	s.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	importCSVBtn := widget.NewButton("Import CSV History", func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			csvPath := reader.URI().Path()
			reader.Close()

			assistant := ai.NewTimesheetAssistant()
			assistant.TrainFromCSVAsync(csvPath, func(count int, trainErr error) {
				assistant.Close()
				fyne.Do(func() {
					if trainErr != nil {
						dialog.ShowError(trainErr, s.window)
					} else {
						dialog.ShowInformation("Import Complete",
							fmt.Sprintf("Imported %d entries and trained AI model.", count),
							s.window)
					}
				})
			})
		}, s.window)
		d.Show()
	})

	logoutBtn := widget.NewButton("Log Out", s.confirmLogout)
	logoutBtn.Importance = widget.DangerImportance

	backBtn := widget.NewButton("← Back to Dashboard", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Menu layout
	menu := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		layout.NewSpacer(),
		container.NewCenter(favBtn),
		layout.NewSpacer(),
		container.NewCenter(codeReposBtn),
		layout.NewSpacer(),
		container.NewCenter(calendarBtn),
		layout.NewSpacer(),
		container.NewCenter(refreshProjectsBtn),
		layout.NewSpacer(),
		container.NewCenter(importCSVBtn),
		layout.NewSpacer(),
		container.NewCenter(s.statusLabel),
		widget.NewSeparator(),
		container.NewCenter(logoutBtn),
		layout.NewSpacer(),
		widget.NewSeparator(),
		container.NewCenter(backBtn),
	)

	// Container with border
	menuBorder := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	menuBorder.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	menuBorder.StrokeWidth = 2
	menuBorder.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	menuBorder.CornerRadius = 10

	menuContainer := container.NewStack(
		menuBorder,
		container.NewPadded(container.NewPadded(menu)),
	)

	s.Container = container.NewCenter(menuContainer)
}

// onRefreshProjects triggers the parent-supplied async refresh and reflects
// state in the status label. No-ops if no handler is wired.
func (s *SettingsScreen) onRefreshProjects() {
	if s.OnRefreshProjects == nil {
		s.setStatus("Refresh not available (no API client)")
		return
	}
	s.setStatus("Syncing projects from ERPNext…")
	s.OnRefreshProjects(func(err error) {
		fyne.Do(func() {
			if err != nil {
				s.setStatus("Sync failed: " + err.Error())
				return
			}
			s.setStatus("Projects synced.")
		})
	})
}

func (s *SettingsScreen) setStatus(msg string) {
	if s.statusLabel == nil {
		return
	}
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

func (s *SettingsScreen) confirmLogout() {
	dialog.ShowConfirm(
		"Confirm Logout",
		"Are you sure you want to log out?",
		func(confirmed bool) {
			if confirmed {
				// Delete token
				_ = config.DeleteToken()
				if s.OnLogout != nil {
					s.OnLogout()
				}
			}
		},
		s.window,
	)
}
