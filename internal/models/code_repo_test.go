package models

import (
	"testing"
)

func TestNewCodeRepoAssociations(t *testing.T) {
	assocs := NewCodeRepoAssociations()

	if assocs == nil {
		t.Fatal("NewCodeRepoAssociations returned nil")
	}

	if len(assocs.Associations) != 0 {
		t.Errorf("Expected empty associations, got %d", len(assocs.Associations))
	}
}

func TestGetAssociationByProject(t *testing.T) {
	assocs := NewCodeRepoAssociations()
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID:   1,
		ProjectName: "Project One",
		RepoURL:     "github.com/test/one",
		RepoName:    "one",
		RepoOwner:   "test",
	})
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID:   2,
		ProjectName: "Project Two",
		RepoURL:     "github.com/test/two",
		RepoName:    "two",
		RepoOwner:   "test",
	})

	// Test finding existing project
	result := assocs.GetAssociationByProject(1)
	if result == nil {
		t.Fatal("GetAssociationByProject returned nil for existing project")
	}
	if result.ProjectName != "Project One" {
		t.Errorf("Wrong project returned: %s", result.ProjectName)
	}

	// Test finding non-existent project
	result = assocs.GetAssociationByProject(999)
	if result != nil {
		t.Error("GetAssociationByProject should return nil for non-existent project")
	}
}

func TestGetAssociationByRepo(t *testing.T) {
	assocs := NewCodeRepoAssociations()
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID: 1,
		RepoURL:   "github.com/test/repo",
		RepoName:  "repo",
		RepoOwner: "test",
	})

	// Test finding existing repo
	result := assocs.GetAssociationByRepo("github.com/test/repo")
	if result == nil {
		t.Fatal("GetAssociationByRepo returned nil for existing repo")
	}
	if result.RepoName != "repo" {
		t.Errorf("Wrong repo returned: %s", result.RepoName)
	}

	// Test finding non-existent repo
	result = assocs.GetAssociationByRepo("github.com/nonexistent/repo")
	if result != nil {
		t.Error("GetAssociationByRepo should return nil for non-existent repo")
	}
}

func TestSetAssociationCodeRepo(t *testing.T) {
	assocs := NewCodeRepoAssociations()

	// Add new association
	assoc1 := CodeRepoAssociation{
		ProjectID: 1,
		RepoURL:   "github.com/test/one",
		RepoName:  "one",
		RepoOwner: "test",
	}
	assocs.SetAssociation(assoc1)

	if len(assocs.Associations) != 1 {
		t.Errorf("Expected 1 association, got %d", len(assocs.Associations))
	}

	// Update existing association
	assoc1Updated := CodeRepoAssociation{
		ProjectID: 1,
		RepoURL:   "github.com/test/updated",
		RepoName:  "updated",
		RepoOwner: "test",
	}
	assocs.SetAssociation(assoc1Updated)

	if len(assocs.Associations) != 1 {
		t.Errorf("Expected 1 association after update, got %d", len(assocs.Associations))
	}
	if assocs.Associations[0].RepoName != "updated" {
		t.Errorf("Association not updated: got %s", assocs.Associations[0].RepoName)
	}

	// Add second association
	assoc2 := CodeRepoAssociation{
		ProjectID: 2,
		RepoURL:   "github.com/test/two",
		RepoName:  "two",
		RepoOwner: "test",
	}
	assocs.SetAssociation(assoc2)

	if len(assocs.Associations) != 2 {
		t.Errorf("Expected 2 associations, got %d", len(assocs.Associations))
	}
}

func TestRemoveAssociation(t *testing.T) {
	assocs := NewCodeRepoAssociations()
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID: 1,
		RepoName:  "one",
	})
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID: 2,
		RepoName:  "two",
	})
	assocs.Associations = append(assocs.Associations, CodeRepoAssociation{
		ProjectID: 3,
		RepoName:  "three",
	})

	assocs.RemoveAssociation(2)

	if len(assocs.Associations) != 2 {
		t.Errorf("Expected 2 associations after remove, got %d", len(assocs.Associations))
	}

	// Verify project 2 was removed
	for _, a := range assocs.Associations {
		if a.ProjectID == 2 {
			t.Error("Project 2 should have been removed")
		}
	}

	// Verify projects 1 and 3 remain
	if assocs.GetAssociationByProject(1) == nil {
		t.Error("Project 1 should still exist")
	}
	if assocs.GetAssociationByProject(3) == nil {
		t.Error("Project 3 should still exist")
	}
}

func TestCodeRepoHasAssociation(t *testing.T) {
	assoc := CodeRepoAssociation{}

	if assoc.HasAssociation() {
		t.Error("Empty association should return false")
	}

	assoc.RepoURL = "github.com/test/repo"
	if !assoc.HasAssociation() {
		t.Error("Association with RepoURL should return true")
	}
}

func TestCodeRepoGetDisplayString(t *testing.T) {
	// Test no repo
	assoc := CodeRepoAssociation{}
	if assoc.GetDisplayString() != "No repository linked" {
		t.Errorf("Expected 'No repository linked', got %s", assoc.GetDisplayString())
	}

	// Test with repo
	assoc.RepoURL = "github.com/kartoza/test-repo"
	assoc.RepoOwner = "kartoza"
	assoc.RepoName = "test-repo"
	if assoc.GetDisplayString() != "kartoza/test-repo" {
		t.Errorf("Expected 'kartoza/test-repo', got %s", assoc.GetDisplayString())
	}
}
