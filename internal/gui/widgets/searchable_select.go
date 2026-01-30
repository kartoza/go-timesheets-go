package widgets

import (
	"strings"

	"fyne.io/fyne/v2/widget"
)

// SelectItem represents an item in the searchable select
type SelectItem struct {
	ID    int
	Label string
}

// SearchableSelect is a searchable dropdown widget wrapping Fyne's native
// SelectEntry for reliable popup lifecycle, text clearing, and keyboard navigation.
// It adds ID-based item mapping and async search support on top.
type SearchableSelect struct {
	Entry     *widget.SelectEntry
	Items     []SelectItem
	Selected  *SelectItem
	OnChanged func(*SelectItem)
	OnSearch  func(query string) // Called when user types to trigger async search

	settingItems bool // guard to prevent re-triggering search during SetItems
}

// NewSearchableSelect creates a new searchable select widget
func NewSearchableSelect(placeholder string, _ interface{}) *SearchableSelect {
	s := &SearchableSelect{
		Items: []SelectItem{},
	}

	s.Entry = widget.NewSelectEntry([]string{})
	s.Entry.PlaceHolder = placeholder
	s.Entry.OnChanged = func(text string) {
		// Trigger async search for 2+ characters (but not when we're re-setting text from SetItems)
		if s.OnSearch != nil && len(text) >= 2 && !s.settingItems {
			s.OnSearch(text)
		}

		// Try to match typed text to an existing item
		matched := false
		for _, item := range s.Items {
			if strings.EqualFold(item.Label, text) {
				itemCopy := item
				s.Selected = &itemCopy
				if s.OnChanged != nil {
					s.OnChanged(&itemCopy)
				}
				matched = true
				break
			}
		}
		if !matched && s.Selected != nil {
			// Text changed to something that doesn't match — clear selection
			s.Selected = nil
			if s.OnChanged != nil {
				s.OnChanged(nil)
			}
		}
	}

	return s
}

// SetItems updates the items list (called from async via fyne.Do)
func (s *SearchableSelect) SetItems(items []SelectItem) {
	s.Items = items
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	// Set options with ALL labels (unfiltered) so the dropdown shows them.
	// SelectEntry filters internally based on current text.
	s.Entry.SetOptions(labels)

	// Re-set the current text to force SelectEntry to re-evaluate
	// its filtered dropdown with the new options while user is typing.
	currentText := s.Entry.Text
	if currentText != "" {
		s.settingItems = true
		s.Entry.SetText(currentText)
		s.settingItems = false
	}
}

// SetSelected sets the selected item
func (s *SearchableSelect) SetSelected(item *SelectItem) {
	s.Selected = item
	if item != nil {
		s.Entry.SetText(item.Label)
	} else {
		s.Entry.SetText("")
	}
}

// GetSelected returns the selected item
func (s *SearchableSelect) GetSelected() *SelectItem {
	return s.Selected
}

// ClearSelection clears the current selection
func (s *SearchableSelect) ClearSelection() {
	s.Selected = nil
	s.Entry.SetText("")
}

// Disable disables the searchable select
func (s *SearchableSelect) Disable() {
	s.Entry.Disable()
}

// Enable enables the searchable select
func (s *SearchableSelect) Enable() {
	s.Entry.Enable()
}
