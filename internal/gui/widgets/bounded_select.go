package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// BoundedSelect is a traditional dropdown select widget (no typing allowed).
// Uses Fyne's native widget.Select for click-to-select behavior.
type BoundedSelect struct {
	Select      *widget.Select
	Options     []string
	Selected    string
	OnChanged   func(string)
	placeholder string
}

// NewBoundedSelect creates a new bounded select widget (traditional dropdown)
func NewBoundedSelect(placeholder string, options []string, _ interface{}) *BoundedSelect {
	s := &BoundedSelect{
		Options:     options,
		placeholder: placeholder,
	}

	s.Select = widget.NewSelect(options, func(selected string) {
		s.Selected = selected
		if s.OnChanged != nil {
			s.OnChanged(selected)
		}
	})
	s.Select.PlaceHolder = placeholder

	return s
}

// SetOptions updates the options list
func (s *BoundedSelect) SetOptions(options []string) {
	s.Options = options
	s.Select.Options = options
	s.Select.Refresh()
}

// SetSelected sets the selected value
func (s *BoundedSelect) SetSelected(value string) {
	s.Selected = value
	s.Select.SetSelected(value)
}

// ClearSelected clears the selection
func (s *BoundedSelect) ClearSelected() {
	s.Selected = ""
	s.Select.ClearSelected()
	s.Select.PlaceHolder = s.placeholder
	s.Select.Refresh()
}

// Disable disables the bounded select
func (s *BoundedSelect) Disable() {
	s.Select.Disable()
}

// Enable enables the bounded select
func (s *BoundedSelect) Enable() {
	s.Select.Enable()
}

// Widget returns the underlying fyne widget for layout purposes
func (s *BoundedSelect) Widget() fyne.CanvasObject {
	return s.Select
}
