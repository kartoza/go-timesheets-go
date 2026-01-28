package models

// FavouriteAssociation represents a single favourite button with preset project/task/activity
type FavouriteAssociation struct {
	SlotNumber   int    `json:"slot_number"`   // 1-9 (3x3 grid)
	Name         string `json:"name"`          // User-defined name for the favourite
	ProjectID    int    `json:"project_id,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
	TaskID       int    `json:"task_id,omitempty"`
	TaskName     string `json:"task_name,omitempty"`
	ActivityID   int    `json:"activity_id,omitempty"`
	ActivityName string `json:"activity_name,omitempty"`
	Color        string `json:"color,omitempty"` // Hex color for the button (e.g. "#E95420")
}

// FavouriteAssociations holds all 9 favourite slots (3x3 grid)
type FavouriteAssociations struct {
	Associations []FavouriteAssociation `json:"associations"`
}

// NewFavouriteAssociations creates a new favourite associations container with 9 empty slots
func NewFavouriteAssociations() *FavouriteAssociations {
	associations := make([]FavouriteAssociation, 9)
	for i := 0; i < 9; i++ {
		associations[i] = FavouriteAssociation{
			SlotNumber: i + 1,
			Name:       "",
		}
	}
	return &FavouriteAssociations{
		Associations: associations,
	}
}

// GetAssociation returns the association for a slot number (1-9)
func (fa *FavouriteAssociations) GetAssociation(slotNumber int) *FavouriteAssociation {
	if slotNumber < 1 || slotNumber > 9 {
		return nil
	}
	return &fa.Associations[slotNumber-1]
}

// SetAssociation sets the association for a slot number (1-9)
func (fa *FavouriteAssociations) SetAssociation(slotNumber int, assoc FavouriteAssociation) {
	if slotNumber < 1 || slotNumber > 9 {
		return
	}
	assoc.SlotNumber = slotNumber
	fa.Associations[slotNumber-1] = assoc
}

// ClearAssociation clears the association for a slot number (1-9)
func (fa *FavouriteAssociations) ClearAssociation(slotNumber int) {
	if slotNumber < 1 || slotNumber > 9 {
		return
	}
	fa.Associations[slotNumber-1] = FavouriteAssociation{
		SlotNumber: slotNumber,
		Name:       fa.Associations[slotNumber-1].Name, // Keep the name
	}
}

// HasAssociation returns true if the favourite has a project assigned
func (a *FavouriteAssociation) HasAssociation() bool {
	return a.ProjectID > 0
}

// GetDisplayString returns a formatted string for display
func (a *FavouriteAssociation) GetDisplayString() string {
	if !a.HasAssociation() {
		return "Not configured"
	}

	result := a.ProjectName
	if a.TaskName != "" {
		result += " / " + a.TaskName
	}
	if a.ActivityName != "" {
		result += " / " + a.ActivityName
	}
	return result
}

// GetShortDisplayString returns a shorter formatted string for button display
func (a *FavouriteAssociation) GetShortDisplayString(maxLen int) string {
	if !a.HasAssociation() {
		return "Empty"
	}

	// Just show the project name or task name if available
	result := a.ProjectName
	if a.TaskName != "" {
		result = a.TaskName
	}

	if len(result) > maxLen {
		result = result[:maxLen-1] + "…"
	}
	return result
}
