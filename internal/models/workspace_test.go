package models

import (
	"testing"
)

func TestNewWorkspaceAssociations(t *testing.T) {
	assocs := NewWorkspaceAssociations()

	if assocs == nil {
		t.Fatal("NewWorkspaceAssociations returned nil")
	}

	if len(assocs.Associations) != 10 {
		t.Errorf("Expected 10 workspace slots, got %d", len(assocs.Associations))
	}

	// Verify workspace numbers are set correctly
	for i := 0; i < 10; i++ {
		if assocs.Associations[i].WorkspaceNumber != i+1 {
			t.Errorf("Workspace %d has wrong number: %d", i, assocs.Associations[i].WorkspaceNumber)
		}
	}
}

func TestGetAssociation(t *testing.T) {
	assocs := NewWorkspaceAssociations()
	assocs.Associations[4] = WorkspaceAssociation{
		WorkspaceNumber: 5,
		WorkspaceName:   "Development",
		ProjectID:       123,
		ProjectName:     "Test Project",
	}

	// Test valid workspace number
	result := assocs.GetAssociation(5)
	if result == nil {
		t.Fatal("GetAssociation returned nil for valid workspace")
	}
	if result.WorkspaceName != "Development" {
		t.Errorf("Wrong workspace returned: %s", result.WorkspaceName)
	}

	// Test out of range - low
	result = assocs.GetAssociation(0)
	if result != nil {
		t.Error("GetAssociation should return nil for workspace 0")
	}

	// Test out of range - high
	result = assocs.GetAssociation(11)
	if result != nil {
		t.Error("GetAssociation should return nil for workspace > 10")
	}
}

func TestSetAssociation(t *testing.T) {
	assocs := NewWorkspaceAssociations()

	assoc := WorkspaceAssociation{
		WorkspaceName: "Testing",
		ProjectID:     456,
		ProjectName:   "Another Project",
		TaskID:        789,
		TaskName:      "Some Task",
	}

	assocs.SetAssociation(3, assoc)

	result := assocs.GetAssociation(3)
	if result.WorkspaceName != "Testing" {
		t.Errorf("WorkspaceName not set: got %s", result.WorkspaceName)
	}
	if result.WorkspaceNumber != 3 {
		t.Errorf("WorkspaceNumber should be overwritten to 3, got %d", result.WorkspaceNumber)
	}
	if result.ProjectID != 456 {
		t.Errorf("ProjectID not set: got %d", result.ProjectID)
	}

	// Test out of range
	assocs.SetAssociation(0, assoc)  // Should do nothing
	assocs.SetAssociation(11, assoc) // Should do nothing
}

func TestClearAssociation(t *testing.T) {
	assocs := NewWorkspaceAssociations()

	// Set up an association
	assocs.Associations[2] = WorkspaceAssociation{
		WorkspaceNumber: 3,
		WorkspaceName:   "Keep This Name",
		ProjectID:       123,
		ProjectName:     "Clear Me",
		TaskID:          456,
	}

	assocs.ClearAssociation(3)

	result := assocs.GetAssociation(3)
	// Name should be preserved
	if result.WorkspaceName != "Keep This Name" {
		t.Errorf("WorkspaceName should be preserved: got %s", result.WorkspaceName)
	}
	// Project should be cleared
	if result.ProjectID != 0 {
		t.Errorf("ProjectID should be cleared: got %d", result.ProjectID)
	}
	if result.TaskID != 0 {
		t.Errorf("TaskID should be cleared: got %d", result.TaskID)
	}
}

func TestHasAssociation(t *testing.T) {
	assoc := WorkspaceAssociation{
		WorkspaceNumber: 1,
	}

	if assoc.HasAssociation() {
		t.Error("Empty association should return false")
	}

	assoc.ProjectID = 123
	if !assoc.HasAssociation() {
		t.Error("Association with ProjectID should return true")
	}
}

func TestGetDisplayString(t *testing.T) {
	// Test unconfigured
	assoc := WorkspaceAssociation{
		WorkspaceNumber: 1,
	}
	if assoc.GetDisplayString() != "Not configured" {
		t.Errorf("Expected 'Not configured', got %s", assoc.GetDisplayString())
	}

	// Test project only
	assoc.ProjectID = 1
	assoc.ProjectName = "Project A"
	if assoc.GetDisplayString() != "Project A" {
		t.Errorf("Expected 'Project A', got %s", assoc.GetDisplayString())
	}

	// Test project + task
	assoc.TaskName = "Task 1"
	if assoc.GetDisplayString() != "Project A / Task 1" {
		t.Errorf("Expected 'Project A / Task 1', got %s", assoc.GetDisplayString())
	}

	// Test project + task + activity
	assoc.ActivityName = "Coding"
	if assoc.GetDisplayString() != "Project A / Task 1 / Coding" {
		t.Errorf("Expected 'Project A / Task 1 / Coding', got %s", assoc.GetDisplayString())
	}
}
