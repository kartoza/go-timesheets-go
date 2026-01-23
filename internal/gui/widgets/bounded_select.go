package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// BoundedSelect is a select widget with a bounded dropdown height
type BoundedSelect struct {
	widget.BaseWidget

	PlaceHolder string
	Options     []string
	Selected    string
	OnChanged   func(string)

	button    *widget.Button
	list      *widget.List
	popup     *widget.PopUp
	parentWin fyne.Window
}

// NewBoundedSelect creates a new bounded select widget
func NewBoundedSelect(placeholder string, options []string, parentWin fyne.Window) *BoundedSelect {
	s := &BoundedSelect{
		PlaceHolder: placeholder,
		Options:     options,
		parentWin:   parentWin,
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetOptions updates the options list
func (s *BoundedSelect) SetOptions(options []string) {
	s.Options = options
	if s.list != nil {
		s.list.Refresh()
	}
}

// SetSelected sets the selected value
func (s *BoundedSelect) SetSelected(value string) {
	s.Selected = value
	s.updateButtonText()
}

// ClearSelected clears the selection
func (s *BoundedSelect) ClearSelected() {
	s.Selected = ""
	s.updateButtonText()
}

func (s *BoundedSelect) updateButtonText() {
	if s.button == nil {
		return
	}
	if s.Selected != "" {
		s.button.SetText(s.Selected)
	} else {
		s.button.SetText(s.PlaceHolder)
	}
}

// CreateRenderer creates the renderer for the bounded select
func (s *BoundedSelect) CreateRenderer() fyne.WidgetRenderer {
	text := s.PlaceHolder
	if s.Selected != "" {
		text = s.Selected
	}

	s.button = widget.NewButton(text+" ▾", func() {
		s.togglePopup()
	})

	// Dark text color for light popup background
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}

	s.list = widget.NewList(
		func() int {
			return len(s.Options)
		},
		func() fyne.CanvasObject {
			label := canvas.NewText("Template option text", darkGray)
			label.TextSize = 14
			return container.NewPadded(label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(s.Options) {
				padded := obj.(*fyne.Container)
				label := padded.Objects[0].(*canvas.Text)
				label.Text = s.Options[id]
				label.Refresh()
			}
		},
	)

	s.list.OnSelected = func(id widget.ListItemID) {
		if id < len(s.Options) {
			s.Selected = s.Options[id]
			s.updateButtonText()

			if s.OnChanged != nil {
				s.OnChanged(s.Selected)
			}
		}
		s.hidePopup()
	}

	return &boundedSelectRenderer{
		sel:    s,
		button: s.button,
	}
}

func (s *BoundedSelect) togglePopup() {
	if s.popup != nil && s.popup.Visible() {
		s.hidePopup()
	} else {
		s.showPopup()
	}
}

func (s *BoundedSelect) showPopup() {
	if s.parentWin == nil || s.button == nil {
		return
	}

	if len(s.Options) == 0 {
		return
	}

	if s.popup != nil {
		s.popup.Show()
		return
	}

	// Calculate height based on number of items (max 8 items visible)
	itemHeight := float32(36)
	maxItems := 8
	numItems := len(s.Options)
	if numItems > maxItems {
		numItems = maxItems
	}
	listHeight := float32(numItems) * itemHeight

	listContainer := container.NewVScroll(s.list)
	listContainer.SetMinSize(fyne.NewSize(250, listHeight))

	s.popup = widget.NewPopUp(listContainer, s.parentWin.Canvas())

	// Get button position
	var btnPos fyne.Position
	if driver := fyne.CurrentApp().Driver(); driver != nil {
		btnPos = driver.AbsolutePositionForObject(s.button)
	}
	btnPos.Y += s.button.Size().Height + 2 // Position below button

	s.popup.ShowAtPosition(btnPos)
}

func (s *BoundedSelect) hidePopup() {
	if s.popup != nil {
		s.popup.Hide()
	}
}

type boundedSelectRenderer struct {
	sel    *BoundedSelect
	button *widget.Button
}

func (r *boundedSelectRenderer) Layout(size fyne.Size) {
	r.button.Resize(size)
	r.button.Move(fyne.NewPos(0, 0))
}

func (r *boundedSelectRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 36)
}

func (r *boundedSelectRenderer) Refresh() {
	r.button.Refresh()
}

func (r *boundedSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.button}
}

func (r *boundedSelectRenderer) Destroy() {
	r.sel.hidePopup()
}
