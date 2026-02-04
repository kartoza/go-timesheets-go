package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SelectItem represents a selectable item with an ID and display label.
type SelectItem struct {
	ID    int
	Label string
}

// ProjectPicker is a project selection widget with a filter text entry and a combo box.
// The user types a few characters in the filter to narrow down the project list,
// then clicks the combo box to select from the filtered results.
// Pressing Tab in the filter entry moves focus to the project select combo.
type ProjectPicker struct {
	Container fyne.CanvasObject

	OnChanged func(item *SelectItem)
	OnSearch  func(query string)

	filterEntry   *tabAwareEntry
	projectSelect *widget.Select

	items    []SelectItem
	selected *SelectItem
}

// tabAwareEntry is a widget.Entry that intercepts Tab to invoke a callback.
type tabAwareEntry struct {
	widget.Entry
	onTab func()
}

func newTabAwareEntry() *tabAwareEntry {
	e := &tabAwareEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// AcceptsTab returns true so the Fyne driver passes Tab key events to TypedKey
// instead of moving focus to the next widget.
func (e *tabAwareEntry) AcceptsTab() bool {
	return true
}

func (e *tabAwareEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyTab && e.onTab != nil {
		e.onTab()
		return
	}
	e.Entry.TypedKey(ev)
}

// NewProjectPicker creates a new project picker with filter entry + combo box.
func NewProjectPicker() *ProjectPicker {
	p := &ProjectPicker{}

	p.filterEntry = newTabAwareEntry()
	p.filterEntry.SetPlaceHolder("Filter")
	p.filterEntry.OnChanged = func(text string) {
		// Limit to 3 characters
		if len(text) > 3 {
			p.filterEntry.SetText(text[:3])
			return
		}
		if p.OnSearch != nil {
			p.OnSearch(text)
		}
	}

	p.projectSelect = widget.NewSelect([]string{}, func(selected string) {
		// Map selected label back to SelectItem
		for i := range p.items {
			if p.items[i].Label == selected {
				p.selected = &p.items[i]
				if p.OnChanged != nil {
					p.OnChanged(p.selected)
				}
				return
			}
		}
	})
	p.projectSelect.PlaceHolder = "Select project..."

	// Wire Tab from filter entry to open the project select dropdown
	p.filterEntry.onTab = func() {
		p.projectSelect.Tapped(&fyne.PointEvent{})
	}

	// Layout: small filter on left, combo takes remaining space
	p.Container = container.NewBorder(
		nil, nil,
		container.NewGridWrap(fyne.NewSize(60, p.filterEntry.MinSize().Height), p.filterEntry),
		nil,
		p.projectSelect,
	)

	return p
}

// SetItems updates the combo box options from API results.
// Preserves the current selection if it's still in the new list.
func (p *ProjectPicker) SetItems(items []SelectItem) {
	p.items = items
	options := make([]string, len(items))
	for i, item := range items {
		options[i] = item.Label
	}
	p.projectSelect.Options = options

	// Preserve selection if still present
	if p.selected != nil {
		found := false
		for _, item := range items {
			if item.ID == p.selected.ID {
				found = true
				break
			}
		}
		if found {
			p.projectSelect.SetSelected(p.selected.Label)
		} else {
			p.projectSelect.ClearSelected()
		}
	}

	p.projectSelect.Refresh()
}

// SetSelected sets the currently selected project.
func (p *ProjectPicker) SetSelected(item *SelectItem) {
	if item == nil {
		p.selected = nil
		p.projectSelect.ClearSelected()
		return
	}
	p.selected = item
	// Ensure the item is in the options list
	found := false
	for _, opt := range p.projectSelect.Options {
		if opt == item.Label {
			found = true
			break
		}
	}
	if !found {
		p.projectSelect.Options = append(p.projectSelect.Options, item.Label)
		p.items = append(p.items, *item)
	}
	p.projectSelect.SetSelected(item.Label)
}

// GetSelected returns the currently selected project.
func (p *ProjectPicker) GetSelected() *SelectItem {
	return p.selected
}

// ClearSelection clears the current selection.
func (p *ProjectPicker) ClearSelection() {
	p.selected = nil
	p.projectSelect.ClearSelected()
}

// Disable disables the project picker widgets.
func (p *ProjectPicker) Disable() {
	p.filterEntry.Disable()
	p.projectSelect.Disable()
}

// Enable enables the project picker widgets.
func (p *ProjectPicker) Enable() {
	p.filterEntry.Enable()
	p.projectSelect.Enable()
}
