package widgets

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// favouritePrefix is prepended to favourite project labels in the dropdown
// so users can distinguish quick-pick favourites from regular search results.
const favouritePrefix = "★ "

// SelectItem represents a selectable item with an ID and display label.
type SelectItem struct {
	ID    int
	Label string
}

// ProjectPicker is a project selection widget with a filter text entry and a combo box.
// The user types a few characters in the filter to narrow down the project list,
// then clicks the combo box to select from the filtered results.
// Pressing Tab in the filter entry moves focus to the project select combo.
//
// When favourites are supplied via SetFavourites, they are pinned to the top of the
// dropdown (visible even before the user types) and are filtered locally alongside
// API-returned results.
type ProjectPicker struct {
	Container fyne.CanvasObject

	OnChanged func(item *SelectItem)
	OnSearch  func(query string)

	filterEntry   *tabAwareEntry
	projectSelect *widget.Select

	favourites    []SelectItem // pinned items shown at the top of the dropdown
	apiItems      []SelectItem // latest results from OnSearch
	items         []SelectItem // combined, filter-applied; index-aligned with projectSelect.Options
	currentFilter string
	selected      *SelectItem
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
		p.currentFilter = text
		// Re-filter favourites locally so they stay visible while the API search runs.
		p.rebuildOptions()
		if p.OnSearch != nil {
			p.OnSearch(text)
		}
	}

	p.projectSelect = widget.NewSelect([]string{}, func(selected string) {
		// Selected is a display label which may include the favourite prefix.
		// Find by matching position in the options slice (kept in sync with p.items).
		for i, label := range p.projectSelect.Options {
			if label == selected && i < len(p.items) {
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

// SetItems updates the combo box options from API search results.
// Favourites (if any) remain pinned at the top and are de-duplicated against the API list.
// Preserves the current selection if it's still in the resulting list.
func (p *ProjectPicker) SetItems(items []SelectItem) {
	p.apiItems = items
	p.rebuildOptions()
}

// SetFavourites pins a list of projects at the top of the dropdown so the user can
// pick them without typing a filter. Pass nil or an empty slice to disable.
func (p *ProjectPicker) SetFavourites(favs []SelectItem) {
	p.favourites = favs
	p.rebuildOptions()
}

// rebuildOptions merges favourites + API items, applies the current filter to both,
// de-duplicates by project ID, and updates the dropdown.
func (p *ProjectPicker) rebuildOptions() {
	filter := strings.ToLower(p.currentFilter)
	matches := func(label string) bool {
		if filter == "" {
			return true
		}
		return strings.Contains(strings.ToLower(label), filter)
	}

	seen := make(map[int]bool, len(p.favourites)+len(p.apiItems))
	combined := make([]SelectItem, 0, len(p.favourites)+len(p.apiItems))
	options := make([]string, 0, len(p.favourites)+len(p.apiItems))

	for _, f := range p.favourites {
		if f.ID == 0 || seen[f.ID] {
			continue
		}
		if !matches(f.Label) {
			continue
		}
		seen[f.ID] = true
		combined = append(combined, f)
		options = append(options, favouritePrefix+f.Label)
	}

	for _, a := range p.apiItems {
		if seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		combined = append(combined, a)
		options = append(options, a.Label)
	}

	p.items = combined
	p.projectSelect.Options = options

	// Preserve current selection if still present in the new list.
	if p.selected != nil {
		for i, it := range combined {
			if it.ID == p.selected.ID {
				p.projectSelect.SetSelected(options[i])
				p.projectSelect.Refresh()
				return
			}
		}
		p.projectSelect.ClearSelected()
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
	// If already in the merged list, just pick the matching display label.
	for i, it := range p.items {
		if it.ID == item.ID {
			p.projectSelect.SetSelected(p.projectSelect.Options[i])
			return
		}
	}
	// Not in the current list — append as a plain (non-favourite) item so it can be displayed.
	p.items = append(p.items, *item)
	p.projectSelect.Options = append(p.projectSelect.Options, item.Label)
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
