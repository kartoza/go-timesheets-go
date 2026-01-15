package models

// WorkspaceAssociation represents a mapping between a workspace and a project/task/activity
type WorkspaceAssociation struct {
	WorkspaceNumber int    `json:"workspace_number"` // 1-10
	WorkspaceName   string `json:"workspace_name"`   // User-defined name for the workspace
	ProjectID       int    `json:"project_id,omitempty"`
	ProjectName     string `json:"project_name,omitempty"`
	TaskID          int    `json:"task_id,omitempty"`
	TaskName        string `json:"task_name,omitempty"`
	ActivityID      int    `json:"activity_id,omitempty"`
	ActivityName    string `json:"activity_name,omitempty"`
}

// WorkspaceAssociations holds all workspace associations
type WorkspaceAssociations struct {
	Associations []WorkspaceAssociation `json:"associations"`
}

// NewWorkspaceAssociations creates a new workspace associations container with 10 empty slots
func NewWorkspaceAssociations() *WorkspaceAssociations {
	associations := make([]WorkspaceAssociation, 10)
	for i := 0; i < 10; i++ {
		associations[i] = WorkspaceAssociation{
			WorkspaceNumber: i + 1,
			WorkspaceName:   "",
		}
	}
	return &WorkspaceAssociations{
		Associations: associations,
	}
}

// GetAssociation returns the association for a workspace number (1-10)
func (wa *WorkspaceAssociations) GetAssociation(workspaceNumber int) *WorkspaceAssociation {
	if workspaceNumber < 1 || workspaceNumber > 10 {
		return nil
	}
	return &wa.Associations[workspaceNumber-1]
}

// SetAssociation sets the association for a workspace number (1-10)
func (wa *WorkspaceAssociations) SetAssociation(workspaceNumber int, assoc WorkspaceAssociation) {
	if workspaceNumber < 1 || workspaceNumber > 10 {
		return
	}
	assoc.WorkspaceNumber = workspaceNumber
	wa.Associations[workspaceNumber-1] = assoc
}

// ClearAssociation clears the association for a workspace number (1-10)
func (wa *WorkspaceAssociations) ClearAssociation(workspaceNumber int) {
	if workspaceNumber < 1 || workspaceNumber > 10 {
		return
	}
	wa.Associations[workspaceNumber-1] = WorkspaceAssociation{
		WorkspaceNumber: workspaceNumber,
		WorkspaceName:   wa.Associations[workspaceNumber-1].WorkspaceName, // Keep the name
	}
}

// HasAssociation returns true if the workspace has a project assigned
func (a *WorkspaceAssociation) HasAssociation() bool {
	return a.ProjectID > 0
}

// GetDisplayString returns a formatted string for display
func (a *WorkspaceAssociation) GetDisplayString() string {
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
