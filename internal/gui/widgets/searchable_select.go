package widgets

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// navigableEntry is an Entry that forwards Up/Down/Enter/Escape to a callback
type navigableEntry struct {
	widget.Entry
	onNav func(key *fyne.KeyEvent) bool // return true if handled
}

func newNavigableEntry(onNav func(key *fyne.KeyEvent) bool) *navigableEntry {
	e := &navigableEntry{onNav: onNav}
	e.ExtendBaseWidget(e)
	return e
}

func (e *navigableEntry) TypedKey(ev *fyne.KeyEvent) {
	if e.onNav != nil && e.onNav(ev) {
		return
	}
	e.Entry.TypedKey(ev)
}

// Ensure navigableEntry implements desktop.Keyable for modifier key support
var _ desktop.Keyable = (*navigableEntry)(nil)

// SelectItem represents an item in the searchable select
type SelectItem struct {
	ID    int
	Label string
}

// SearchableSelect is a searchable dropdown widget with keyboard navigation
type SearchableSelect struct {
	widget.BaseWidget

	PlaceHolder string
	Items       []SelectItem
	Selected    *SelectItem
	OnChanged   func(*SelectItem)
	OnSearch    func(query string) // Called when user types to trigger async search

	Entry       *widget.Entry      // Exposed for layout purposes
	navEntry    *navigableEntry    // the actual canvas object for positioning
	list        *widget.List
	popup       *widget.PopUp
	filtered    []SelectItem
	parentWin   fyne.Window
	isTyping    bool
	highlighted int // keyboard cursor in the popup list (-1 = none)
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
	if s.Entry != nil {
		if item != nil {
			s.Entry.SetText(item.Label)
		} else {
			s.Entry.SetText("")
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
	if s.Entry != nil {
		s.Entry.SetText("")
	}
}

// Disable disables the searchable select
func (s *SearchableSelect) Disable() {
	if s.Entry != nil {
		s.Entry.Disable()
	}
}

// Enable enables the searchable select
func (s *SearchableSelect) Enable() {
	if s.Entry != nil {
		s.Entry.Enable()
	}
}

// CreateRenderer creates the renderer for the searchable select
func (s *SearchableSelect) CreateRenderer() fyne.WidgetRenderer {
	s.highlighted = -1

	navEntry := newNavigableEntry(func(ev *fyne.KeyEvent) bool {
		if s.popup == nil || !s.popup.Visible() {
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
				item := s.filtered[s.highlighted]
				s.Selected = &item
				s.isTyping = false
				s.Entry.SetText(item.Label)
				if s.OnChanged != nil {
					s.OnChanged(&item)
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
	s.Entry = &navEntry.Entry
	s.navEntry = navEntry

	if s.Selected != nil {
		navEntry.SetText(s.Selected.Label)
	}

	navEntry.OnChanged = func(text string) {
		s.isTyping = true
		s.highlighted = -1
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

	// Light text for dark popup background
	normalColor := color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
	highlightColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	s.list = widget.NewList(
		func() int {
			return len(s.filtered)
		},
		func() fyne.CanvasObject {
			label := canvas.NewText("Template item text here", normalColor)
			label.TextSize = 14
			return container.NewPadded(label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(s.filtered) {
				padded := obj.(*fyne.Container)
				label := padded.Objects[0].(*canvas.Text)
				label.Text = s.filtered[id].Label
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
			item := s.filtered[id]
			s.Selected = &item
			s.isTyping = false
			s.Entry.SetText(item.Label)

			if s.OnChanged != nil {
				s.OnChanged(&item)
			}
		}
		s.hidePopup()
	}

	return &searchableSelectRenderer{
		sel:      s,
		entry:    s.Entry,
		navEntry: navEntry,
	}
}

// refreshHighlight refreshes the list to update keyboard highlight styling
func (s *SearchableSelect) refreshHighlight() {
	if s.list != nil {
		s.list.Refresh()
		// Scroll to highlighted item
		if s.highlighted >= 0 {
			s.list.ScrollTo(s.highlighted)
		}
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
	if s.parentWin == nil || s.Entry == nil {
		return
	}

	// Don't show empty popup
	if len(s.filtered) == 0 {
		s.hidePopup()
		return
	}

	// Always recreate popup so position is recalculated
	s.hidePopup()

	// Dark themed background
	bgColor := color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	borderColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	listContainer := container.NewVScroll(s.list)
	itemHeight := float32(36)
	maxItems := 8
	numItems := len(s.filtered)
	if numItems > maxItems {
		numItems = maxItems
	}
	listHeight := float32(numItems) * itemHeight
	listContainer.SetMinSize(fyne.NewSize(350, listHeight))

	bg := canvas.NewRectangle(bgColor)
	bg.StrokeColor = borderColor
	bg.StrokeWidth = 1
	bg.CornerRadius = 4

	popupContent := container.NewStack(bg, listContainer)

	s.popup = widget.NewPopUp(popupContent, s.parentWin.Canvas())

	// Position below the SearchableSelect widget itself
	var entryPos fyne.Position
	if driver := fyne.CurrentApp().Driver(); driver != nil {
		entryPos = driver.AbsolutePositionForObject(s)
	}
	entryPos.Y += s.Size().Height + 2

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
	sel      *SearchableSelect
	entry    *widget.Entry
	navEntry *navigableEntry
}

func (r *searchableSelectRenderer) Layout(size fyne.Size) {
	r.navEntry.Resize(size)
	r.navEntry.Move(fyne.NewPos(0, 0))
}

func (r *searchableSelectRenderer) MinSize() fyne.Size {
	return fyne.NewSize(250, 36)
}

func (r *searchableSelectRenderer) Refresh() {
	r.navEntry.Refresh()
}

func (r *searchableSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.navEntry}
}

func (r *searchableSelectRenderer) Destroy() {
	r.sel.hidePopup()
}
