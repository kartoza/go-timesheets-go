package models

// CodeRepoAssociation represents a mapping between a project and a GitHub repository
type CodeRepoAssociation struct {
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
	RepoURL     string `json:"repo_url"`  // e.g., "github.com/kartoza/go-timesheets-go"
	RepoName    string `json:"repo_name"` // e.g., "go-timesheets-go"
	RepoOwner   string `json:"repo_owner"` // e.g., "kartoza"
}

// CodeRepoAssociations holds all code repo associations
type CodeRepoAssociations struct {
	Associations []CodeRepoAssociation `json:"associations"`
}

// NewCodeRepoAssociations creates a new code repo associations container
func NewCodeRepoAssociations() *CodeRepoAssociations {
	return &CodeRepoAssociations{
		Associations: []CodeRepoAssociation{},
	}
}

// GetAssociationByProject returns the association for a project ID
func (cra *CodeRepoAssociations) GetAssociationByProject(projectID int) *CodeRepoAssociation {
	for i := range cra.Associations {
		if cra.Associations[i].ProjectID == projectID {
			return &cra.Associations[i]
		}
	}
	return nil
}

// GetAssociationByRepo returns the association for a repo URL
func (cra *CodeRepoAssociations) GetAssociationByRepo(repoURL string) *CodeRepoAssociation {
	for i := range cra.Associations {
		if cra.Associations[i].RepoURL == repoURL {
			return &cra.Associations[i]
		}
	}
	return nil
}

// SetAssociation adds or updates an association
func (cra *CodeRepoAssociations) SetAssociation(assoc CodeRepoAssociation) {
	// Check if association already exists for this project
	for i := range cra.Associations {
		if cra.Associations[i].ProjectID == assoc.ProjectID {
			cra.Associations[i] = assoc
			return
		}
	}
	// Add new association
	cra.Associations = append(cra.Associations, assoc)
}

// RemoveAssociation removes an association by project ID
func (cra *CodeRepoAssociations) RemoveAssociation(projectID int) {
	var filtered []CodeRepoAssociation
	for _, assoc := range cra.Associations {
		if assoc.ProjectID != projectID {
			filtered = append(filtered, assoc)
		}
	}
	cra.Associations = filtered
}

// HasAssociation returns true if the project has a repo associated
func (a *CodeRepoAssociation) HasAssociation() bool {
	return a.RepoURL != ""
}

// GetDisplayString returns a formatted string for display
func (a *CodeRepoAssociation) GetDisplayString() string {
	if !a.HasAssociation() {
		return "No repository linked"
	}
	return a.RepoOwner + "/" + a.RepoName
}
