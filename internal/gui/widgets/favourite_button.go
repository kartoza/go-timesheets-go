package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// FavouriteButton is a custom widget for the 3x3 favourites grid
type FavouriteButton struct {
	widget.BaseWidget

	SlotNumber  int
	Name        string
	ProjectName string
	IsRunning   bool
	IsEmpty     bool
	OnTapped    func()

	bg          *canvas.Rectangle
	numberLabel *canvas.Text
	nameLabel   *canvas.Text
	statusLabel *canvas.Text
}

// NewFavouriteButton creates a new favourite button widget
func NewFavouriteButton(slotNumber int, name, projectName string, isRunning, isEmpty bool, onTapped func()) *FavouriteButton {
	f := &FavouriteButton{
		SlotNumber:  slotNumber,
		Name:        name,
		ProjectName: projectName,
		IsRunning:   isRunning,
		IsEmpty:     isEmpty,
		OnTapped:    onTapped,
	}
	f.ExtendBaseWidget(f)
	return f
}

// Update updates the button state
func (f *FavouriteButton) Update(name, projectName string, isRunning, isEmpty bool) {
	f.Name = name
	f.ProjectName = projectName
	f.IsRunning = isRunning
	f.IsEmpty = isEmpty
	f.Refresh()
}

// Tapped handles tap events
func (f *FavouriteButton) Tapped(*fyne.PointEvent) {
	if f.OnTapped != nil {
		f.OnTapped()
	}
}

// TappedSecondary handles secondary tap events (right-click)
func (f *FavouriteButton) TappedSecondary(*fyne.PointEvent) {}

// CreateRenderer creates the renderer for the favourite button
func (f *FavouriteButton) CreateRenderer() fyne.WidgetRenderer {
	// Background
	f.bg = canvas.NewRectangle(color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF})
	f.bg.CornerRadius = 6
	f.bg.StrokeWidth = 2
	f.bg.StrokeColor = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}

	// Slot number
	f.numberLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	f.numberLabel.TextSize = 12

	// Name
	f.nameLabel = canvas.NewText("", color.White)
	f.nameLabel.TextSize = 14
	f.nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	f.nameLabel.Alignment = fyne.TextAlignCenter

	// Status (RUNNING or project name)
	f.statusLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	f.statusLabel.TextSize = 10
	f.statusLabel.Alignment = fyne.TextAlignCenter

	return &favouriteButtonRenderer{
		button:      f,
		bg:          f.bg,
		numberLabel: f.numberLabel,
		nameLabel:   f.nameLabel,
		statusLabel: f.statusLabel,
	}
}

type favouriteButtonRenderer struct {
	button      *FavouriteButton
	bg          *canvas.Rectangle
	numberLabel *canvas.Text
	nameLabel   *canvas.Text
	statusLabel *canvas.Text
}

func (r *favouriteButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	// Number in top-left
	r.numberLabel.Move(fyne.NewPos(8, 4))

	// Name centered
	r.nameLabel.Resize(fyne.NewSize(size.Width-16, 20))
	r.nameLabel.Move(fyne.NewPos(8, size.Height/2-10))

	// Status at bottom
	r.statusLabel.Resize(fyne.NewSize(size.Width-16, 16))
	r.statusLabel.Move(fyne.NewPos(8, size.Height-22))
}

func (r *favouriteButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 70)
}

func (r *favouriteButtonRenderer) Refresh() {
	f := r.button

	r.numberLabel.Text = string(rune('0' + f.SlotNumber))

	if f.IsEmpty {
		r.nameLabel.Text = "Empty"
		r.nameLabel.Color = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
		r.statusLabel.Text = "Click to configure"
		r.bg.FillColor = color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
		r.bg.StrokeColor = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}
	} else if f.IsRunning {
		r.nameLabel.Text = f.Name
		r.nameLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // Green
		r.statusLabel.Text = "RUNNING"
		r.statusLabel.Color = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
		r.bg.FillColor = color.NRGBA{R: 0x1A, G: 0x3D, B: 0x26, A: 0xFF} // Dark green
		r.bg.StrokeColor = color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
	} else {
		r.nameLabel.Text = f.Name
		r.nameLabel.Color = color.White
		r.statusLabel.Text = f.ProjectName
		r.statusLabel.Color = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
		r.bg.FillColor = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}
		r.bg.StrokeColor = color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}
	}

	r.bg.Refresh()
	r.numberLabel.Refresh()
	r.nameLabel.Refresh()
	r.statusLabel.Refresh()
}

func (r *favouriteButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.numberLabel, r.nameLabel, r.statusLabel}
}

func (r *favouriteButtonRenderer) Destroy() {}

// FavouritesGrid creates a 3x3 grid of favourite buttons
func FavouritesGrid(buttons []*FavouriteButton) fyne.CanvasObject {
	grid := container.NewGridWithColumns(3)
	for _, btn := range buttons {
		grid.Add(btn)
	}
	return grid
}
