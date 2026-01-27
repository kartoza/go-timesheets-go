package widgets

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SelectItem represents an item in the searchable select
type SelectItem struct {
	ID    int
	Label string
}

// SearchableSelect is a searchable dropdown widget
type SearchableSelect struct {
	widget.BaseWidget

	PlaceHolder string
	Items       []SelectItem
	Selected    *SelectItem
	OnChanged   func(*SelectItem)
	OnSearch    func(query string) // Called when user types to trigger async search

	entry     *widget.Entry
	list      *widget.List
	popup     *widget.PopUp
	filtered  []SelectItem
	parentWin fyne.Window
	isTyping  bool
}

// NewSearchableSelect creates a new searchable select widget
func NewSearchableSelect(placeholder string, parentWin fyne.Window) *SearchableSelect {
	s := &SearchableSelect{
		PlaceHolder: placeholder,
		Items:       []SelectItem{},
		filtered:    []SelectItem{},
		parentWin:   parentWin,
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetItems updates the items list (called from async via fyne.Do)
func (s *SearchableSelect) SetItems(items []SelectItem) {
	s.Items = items
	s.filtered = items

	if s.list != nil {
		s.list.Refresh()
	}
	// Show popup if we have items and user is typing
	if len(items) > 0 && s.isTyping {
		s.showPopup()
	}
}

// SetSelected sets the selected item
func (s *SearchableSelect) SetSelected(item *SelectItem) {
	s.Selected = item
	if s.entry != nil {
		if item != nil {
			s.entry.SetText(item.Label)
		} else {
			s.entry.SetText("")
		}
	}
}

// GetSelected returns the selected item
func (s *SearchableSelect) GetSelected() *SelectItem {
	return s.Selected
}

// ClearSelection clears the current selection
func (s *SearchableSelect) ClearSelection() {
	s.Selected = nil
	if s.entry != nil {
		s.entry.SetText("")
	}
}

// Disable disables the searchable select
func (s *SearchableSelect) Disable() {
	if s.entry != nil {
		s.entry.Disable()
	}
}

// Enable enables the searchable select
func (s *SearchableSelect) Enable() {
	if s.entry != nil {
		s.entry.Enable()
	}
}

// CreateRenderer creates the renderer for the searchable select
func (s *SearchableSelect) CreateRenderer() fyne.WidgetRenderer {
	s.entry = widget.NewEntry()
	s.entry.PlaceHolder = s.PlaceHolder

	if s.Selected != nil {
		s.entry.SetText(s.Selected.Label)
	}

	s.entry.OnChanged = func(text string) {
		s.isTyping = true
		s.filterItems(text)

		// Trigger search callback - API requires 2+ characters
		if s.OnSearch != nil && len(text) >= 2 {
			s.OnSearch(text)
		}

		// Show popup when typing (with 2+ chars to match API requirement)
		if len(text) >= 2 {
			s.showPopup()
		} else {
			s.hidePopup()
		}
	}

	// Dark text color for light popup background
	darkGray := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}

	s.list = widget.NewList(
		func() int {
			return len(s.filtered)
		},
		func() fyne.CanvasObject {
			label := canvas.NewText("Template item text here", darkGray)
			label.TextSize = 14
			return container.NewPadded(label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(s.filtered) {
				padded := obj.(*fyne.Container)
				label := padded.Objects[0].(*canvas.Text)
				label.Text = s.filtered[id].Label
				label.Refresh()
			}
		},
	)

	s.list.OnSelected = func(id widget.ListItemID) {
		if id < len(s.filtered) {
			item := s.filtered[id]
			s.Selected = &item
			s.isTyping = false
			s.entry.SetText(item.Label)

			if s.OnChanged != nil {
				s.OnChanged(&item)
			}
		}
		s.hidePopup()
	}

	return &searchableSelectRenderer{
		sel:   s,
		entry: s.entry,
	}
}

func (s *SearchableSelect) filterItems(query string) {
	if query == "" {
		s.filtered = s.Items
	} else {
		query = strings.ToLower(query)
		s.filtered = nil
		for _, item := range s.Items {
			if strings.Contains(strings.ToLower(item.Label), query) {
				s.filtered = append(s.filtered, item)
			}
		}
	}

	if s.list != nil {
		s.list.Refresh()
	}
}

func (s *SearchableSelect) showPopup() {
	if s.parentWin == nil || s.entry == nil {
		return
	}

	// Don't show empty popup
	if len(s.filtered) == 0 {
		s.hidePopup()
		return
	}

	if s.popup != nil {
		s.popup.Show()
		return
	}

	listContainer := container.NewVScroll(s.list)
	listContainer.SetMinSize(fyne.NewSize(350, 200))

	s.popup = widget.NewPopUp(listContainer, s.parentWin.Canvas())

	// Get entry position
	var entryPos fyne.Position
	if driver := fyne.CurrentApp().Driver(); driver != nil {
		entryPos = driver.AbsolutePositionForObject(s.entry)
	}
	entryPos.Y += s.entry.Size().Height + 2 // Position below entry with small gap

	s.popup.ShowAtPosition(entryPos)
}

func (s *SearchableSelect) hidePopup() {
	if s.popup != nil {
		s.popup.Hide()
	}
}

// FocusGained is called when the entry gains focus
func (s *SearchableSelect) FocusGained() {
	// Show existing items when focused
	if len(s.Items) > 0 {
		s.showPopup()
	}
}

// FocusLost is called when the entry loses focus
func (s *SearchableSelect) FocusLost() {
	// Delay hiding to allow list selection to complete
	// The popup will be hidden when an item is selected
}

type searchableSelectRenderer struct {
	sel   *SearchableSelect
	entry *widget.Entry
}

func (r *searchableSelectRenderer) Layout(size fyne.Size) {
	r.entry.Resize(size)
	r.entry.Move(fyne.NewPos(0, 0))
}

func (r *searchableSelectRenderer) MinSize() fyne.Size {
	return fyne.NewSize(250, 36)
}

func (r *searchableSelectRenderer) Refresh() {
	r.entry.Refresh()
}

func (r *searchableSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.entry}
}

func (r *searchableSelectRenderer) Destroy() {
	r.sel.hidePopup()
}
