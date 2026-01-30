package widgets

import (
	"fyne.io/fyne/v2/widget"
)

// BoundedSelect is a select widget wrapping Fyne's native SelectEntry
// for reliable popup lifecycle, text clearing, and keyboard navigation.
type BoundedSelect struct {
	Entry     *widget.SelectEntry
	Options   []string
	Selected  string
	OnChanged func(string)
}

// NewBoundedSelect creates a new bounded select widget
func NewBoundedSelect(placeholder string, options []string, _ interface{}) *BoundedSelect {
	s := &BoundedSelect{
		Options: options,
	}

	s.Entry = widget.NewSelectEntry(options)
	s.Entry.PlaceHolder = placeholder
	s.Entry.OnChanged = func(text string) {
		s.Selected = text
		if s.OnChanged != nil {
			s.OnChanged(text)
		}
	}

	return s
}

// SetOptions updates the options list
func (s *BoundedSelect) SetOptions(options []string) {
	s.Options = options
	s.Entry.SetOptions(options)
}

// SetSelected sets the selected value
func (s *BoundedSelect) SetSelected(value string) {
	s.Selected = value
	s.Entry.SetText(value)
}

// ClearSelected clears the selection
func (s *BoundedSelect) ClearSelected() {
	s.Selected = ""
	s.Entry.SetText("")
}

// Disable disables the bounded select
func (s *BoundedSelect) Disable() {
	s.Entry.Disable()
}

// Enable enables the bounded select
func (s *BoundedSelect) Enable() {
	s.Entry.Enable()
}
