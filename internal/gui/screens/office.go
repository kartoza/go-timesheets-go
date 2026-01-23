package screens

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/gui/widgets"
)

// OfficeScreen displays the virtual office scene
type OfficeScreen struct {
	Container fyne.CanvasObject

	office     *widgets.IsometricOffice
	backButton *widget.Button

	// Callbacks
	OnBack func()
}

// NewOfficeScreen creates a new office screen
func NewOfficeScreen(window fyne.Window) *OfficeScreen {
	s := &OfficeScreen{}
	s.build()
	return s
}

func (s *OfficeScreen) build() {
	// Office scene - fills entire container
	s.office = widgets.NewIsometricOffice()

	// Back button (floating over the office scene)
	s.backButton = widget.NewButton("← Back", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Stack the office scene with the back button overlaid
	// The office fills the entire space, back button floats in corner
	s.Container = container.NewStack(
		s.office,
		container.NewVBox(
			container.NewHBox(s.backButton),
		),
	)
}

// StartAnimation starts the office animation
func (s *OfficeScreen) StartAnimation() {
	if s.office != nil {
		s.office.StartAnimation()
	}
}

// StopAnimation stops the office animation
func (s *OfficeScreen) StopAnimation() {
	if s.office != nil {
		s.office.StopAnimation()
	}
}

// Refresh refreshes the office screen
func (s *OfficeScreen) Refresh() {
	// Nothing to refresh from API
}
