package widgets

import (
	"strings"
	"sync"

	"fyne.io/fyne/v2"
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

	entry      *widget.Entry
	list       *widget.List
	popup      *widget.PopUp
	filtered   []SelectItem
	mu         sync.Mutex
	parentWin  fyne.Window
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

// SetItems updates the items list
func (s *SearchableSelect) SetItems(items []SelectItem) {
	s.mu.Lock()
	s.Items = items
	s.filtered = items
	s.mu.Unlock()
	if s.list != nil {
		s.list.Refresh()
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

// CreateRenderer creates the renderer for the searchable select
func (s *SearchableSelect) CreateRenderer() fyne.WidgetRenderer {
	s.entry = widget.NewEntry()
	s.entry.PlaceHolder = s.PlaceHolder
	s.entry.OnChanged = func(text string) {
		s.filterItems(text)
		if len(text) >= 2 && s.OnSearch != nil {
			s.OnSearch(text)
		}
		if s.popup != nil && !s.popup.Visible() && len(text) > 0 {
			s.showPopup()
		}
	}

	s.list = widget.NewList(
		func() int {
			s.mu.Lock()
			defer s.mu.Unlock()
			return len(s.filtered)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if id < len(s.filtered) {
				obj.(*widget.Label).SetText(s.filtered[id].Label)
			}
		},
	)
	s.list.OnSelected = func(id widget.ListItemID) {
		s.mu.Lock()
		if id < len(s.filtered) {
			item := s.filtered[id]
			s.Selected = &item
			s.mu.Unlock()
			s.entry.SetText(item.Label)
			if s.OnChanged != nil {
				s.OnChanged(&item)
			}
		} else {
			s.mu.Unlock()
		}
		s.hidePopup()
	}

	return &searchableSelectRenderer{
		sel:   s,
		entry: s.entry,
	}
}

func (s *SearchableSelect) filterItems(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s.popup != nil {
		s.popup.Show()
		return
	}

	if s.parentWin == nil {
		return
	}

	listContainer := container.NewVScroll(s.list)
	listContainer.SetMinSize(fyne.NewSize(300, 200))

	s.popup = widget.NewPopUp(listContainer, s.parentWin.Canvas())

	// Position below the entry
	pos := s.entry.Position()
	pos.Y += s.entry.Size().Height
	s.popup.ShowAtPosition(pos)
}

func (s *SearchableSelect) hidePopup() {
	if s.popup != nil {
		s.popup.Hide()
	}
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
	return fyne.NewSize(200, 36)
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
