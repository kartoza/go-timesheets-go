package widgets

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// BoundedSelect is a select widget with a bounded dropdown height and keyboard navigation
type BoundedSelect struct {
	widget.BaseWidget

	PlaceHolder string
	Options     []string
	Selected    string
	OnChanged   func(string)

	entry       *widget.Entry
	navEntryObj *navigableEntry // the actual canvas object for positioning
	list        *widget.List
	popup       *widget.PopUp
	parentWin   fyne.Window
	filtered    []string
	highlighted int  // keyboard cursor (-1 = none)
	isTyping    bool
}

// NewBoundedSelect creates a new bounded select widget
func NewBoundedSelect(placeholder string, options []string, parentWin fyne.Window) *BoundedSelect {
	s := &BoundedSelect{
		PlaceHolder: placeholder,
		Options:     options,
		filtered:    options,
		parentWin:   parentWin,
		highlighted: -1,
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetOptions updates the options list
func (s *BoundedSelect) SetOptions(options []string) {
	s.Options = options
	s.filterItems("")
	if s.list != nil {
		s.list.Refresh()
	}
}

// SetSelected sets the selected value
func (s *BoundedSelect) SetSelected(value string) {
	s.Selected = value
	if s.entry != nil {
		s.entry.SetText(value)
	}
}

// ClearSelected clears the selection
func (s *BoundedSelect) ClearSelected() {
	s.Selected = ""
	if s.entry != nil {
		s.entry.SetText("")
	}
}

// Disable disables the bounded select
func (s *BoundedSelect) Disable() {
	if s.entry != nil {
		s.entry.Disable()
	}
}

// Enable enables the bounded select
func (s *BoundedSelect) Enable() {
	if s.entry != nil {
		s.entry.Enable()
	}
}

func (s *BoundedSelect) filterItems(query string) {
	if query == "" {
		s.filtered = s.Options
	} else {
		q := strings.ToLower(query)
		s.filtered = nil
		for _, opt := range s.Options {
			if strings.Contains(strings.ToLower(opt), q) {
				s.filtered = append(s.filtered, opt)
			}
		}
	}
}

// CreateRenderer creates the renderer for the bounded select
func (s *BoundedSelect) CreateRenderer() fyne.WidgetRenderer {
	s.highlighted = -1

	navEntry := newNavigableEntry(func(ev *fyne.KeyEvent) bool {
		if s.popup == nil || !s.popup.Visible() {
			// Open popup on Down arrow even when closed
			if ev.Name == fyne.KeyDown && len(s.filtered) > 0 {
				s.highlighted = 0
				s.showPopup()
				s.refreshHighlight()
				return true
			}
			return false
		}
		switch ev.Name {
		case fyne.KeyDown:
			if s.highlighted < len(s.filtered)-1 {
				s.highlighted++
			}
			s.refreshHighlight()
			return true
		case fyne.KeyUp:
			if s.highlighted > 0 {
				s.highlighted--
			}
			s.refreshHighlight()
			return true
		case fyne.KeyReturn:
			if s.highlighted >= 0 && s.highlighted < len(s.filtered) {
				s.Selected = s.filtered[s.highlighted]
				s.isTyping = false
				s.entry.SetText(s.Selected)
				if s.OnChanged != nil {
					s.OnChanged(s.Selected)
				}
				s.hidePopup()
			}
			return true
		case fyne.KeyEscape:
			s.hidePopup()
			return true
		}
		return false
	})
	navEntry.PlaceHolder = s.PlaceHolder
	s.entry = &navEntry.Entry
	s.navEntryObj = navEntry

	if s.Selected != "" {
		navEntry.SetText(s.Selected)
	}

	navEntry.OnChanged = func(text string) {
		s.isTyping = true
		s.highlighted = -1
		s.filterItems(text)
		if s.list != nil {
			s.list.Refresh()
		}
		if len(s.filtered) > 0 {
			s.showPopup()
		} else {
			s.hidePopup()
		}
	}

	// Light text for dark popup background
	normalColor := color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
	highlightColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	s.list = widget.NewList(
		func() int {
			return len(s.filtered)
		},
		func() fyne.CanvasObject {
			label := canvas.NewText("Template option text", normalColor)
			label.TextSize = 14
			return container.NewPadded(label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(s.filtered) {
				padded := obj.(*fyne.Container)
				label := padded.Objects[0].(*canvas.Text)
				label.Text = s.filtered[id]
				if id == s.highlighted {
					label.Color = highlightColor
					label.TextStyle = fyne.TextStyle{Bold: true}
				} else {
					label.Color = normalColor
					label.TextStyle = fyne.TextStyle{}
				}
				label.Refresh()
			}
		},
	)

	s.list.OnSelected = func(id widget.ListItemID) {
		if id < len(s.filtered) {
			s.Selected = s.filtered[id]
			s.isTyping = false
			s.entry.SetText(s.Selected)
			if s.OnChanged != nil {
				s.OnChanged(s.Selected)
			}
		}
		s.hidePopup()
	}

	return &boundedSelectRenderer{
		sel:      s,
		navEntry: navEntry,
	}
}

func (s *BoundedSelect) refreshHighlight() {
	if s.list != nil {
		s.list.Refresh()
		if s.highlighted >= 0 {
			s.list.ScrollTo(s.highlighted)
		}
	}
}

func (s *BoundedSelect) showPopup() {
	if s.parentWin == nil || s.entry == nil {
		return
	}

	if len(s.filtered) == 0 {
		s.hidePopup()
		return
	}

	// Always recreate popup so position is recalculated
	s.hidePopup()

	// Dark themed background
	bgColor := color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	borderColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	// Calculate height based on number of items (max 8 items visible)
	itemHeight := float32(36)
	maxItems := 8
	numItems := len(s.filtered)
	if numItems > maxItems {
		numItems = maxItems
	}
	listHeight := float32(numItems) * itemHeight

	listContainer := container.NewVScroll(s.list)
	listContainer.SetMinSize(fyne.NewSize(250, listHeight))

	bg := canvas.NewRectangle(bgColor)
	bg.StrokeColor = borderColor
	bg.StrokeWidth = 1
	bg.CornerRadius = 4

	popupContent := container.NewStack(bg, listContainer)

	s.popup = widget.NewPopUp(popupContent, s.parentWin.Canvas())

	// Position below the BoundedSelect widget itself
	var entryPos fyne.Position
	if driver := fyne.CurrentApp().Driver(); driver != nil {
		entryPos = driver.AbsolutePositionForObject(s)
	}
	entryPos.Y += s.Size().Height + 2

	s.popup.ShowAtPosition(entryPos)
}

func (s *BoundedSelect) hidePopup() {
	if s.popup != nil {
		s.popup.Hide()
	}
}

type boundedSelectRenderer struct {
	sel      *BoundedSelect
	navEntry *navigableEntry
}

func (r *boundedSelectRenderer) Layout(size fyne.Size) {
	r.navEntry.Resize(size)
	r.navEntry.Move(fyne.NewPos(0, 0))
}

func (r *boundedSelectRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 36)
}

func (r *boundedSelectRenderer) Refresh() {
	r.navEntry.Refresh()
}

func (r *boundedSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.navEntry}
}

func (r *boundedSelectRenderer) Destroy() {
	r.sel.hidePopup()
}
