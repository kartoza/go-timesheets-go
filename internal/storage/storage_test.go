package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

func setupTestStorage(t *testing.T) (*Storage, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "kartoza-timesheet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, err := New(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNewStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kartoza-timesheet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test creating storage in new directory
	storage, err := New(filepath.Join(tmpDir, "subdir"))
	if err != nil {
		t.Errorf("Failed to create storage: %v", err)
	}
	if storage == nil {
		t.Error("Storage should not be nil")
	}
}

func TestSaveAndLoadTimeEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create a time entry
	entry := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Test entry")

	// Save it
	err := storage.SaveTimeEntry(entry)
	if err != nil {
		t.Fatalf("Failed to save time entry: %v", err)
	}

	// Load all entries
	entries, err := storage.LoadTimeEntries()
	if err != nil {
		t.Fatalf("Failed to load time entries: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].ID != entry.ID {
		t.Errorf("Entry ID mismatch: expected %s, got %s", entry.ID, entries[0].ID)
	}
}

func TestUpdateTimeEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	entry := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Original")
	storage.SaveTimeEntry(entry)

	// Update the entry
	entry.Description = "Updated description"
	storage.SaveTimeEntry(entry)

	// Load and verify
	entries, _ := storage.LoadTimeEntries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry after update, got %d", len(entries))
	}
	if entries[0].Description != "Updated description" {
		t.Errorf("Description not updated: got %s", entries[0].Description)
	}
}

func TestDeleteTimeEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	entry1 := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Entry 1")
	entry2 := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Entry 2")

	storage.SaveTimeEntry(entry1)
	storage.SaveTimeEntry(entry2)

	// Delete first entry
	err := storage.DeleteTimeEntry(entry1.ID)
	if err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	// Verify only second entry remains
	entries, _ := storage.LoadTimeEntries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry after delete, got %d", len(entries))
	}
	if entries[0].ID != entry2.ID {
		t.Error("Wrong entry was deleted")
	}
}

func TestGetTimeEntriesByDateRange(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	// Create entries with different dates
	entry1 := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Today")
	entry1.StartTime = now

	entry2 := models.NewTimeEntry("user-1", "project-1", "coding", nil, "Yesterday")
	entry2.StartTime = yesterday

	storage.SaveTimeEntry(entry1)
	storage.SaveTimeEntry(entry2)

	// Get entries from today onwards
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	filtered, err := storage.GetTimeEntriesByDateRange(todayStart, tomorrow)
	if err != nil {
		t.Fatalf("Failed to get entries by date range: %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("Expected 1 entry in range, got %d", len(filtered))
	}
}

func TestSaveAndLoadProjects(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	project := models.NewProject("Test Project", "A test project")

	err := storage.SaveProject(project)
	if err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	projects, err := storage.LoadProjects()
	if err != nil {
		t.Fatalf("Failed to load projects: %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "Test Project" {
		t.Errorf("Project name mismatch: got %s", projects[0].Name)
	}
}

func TestSaveAndLoadTasks(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	task := models.NewTask("project-1", "Test Task", "A test task", 8.0)

	err := storage.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	tasks, err := storage.LoadTasks()
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestGetTasksByProjectID(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	task1 := models.NewTask("project-1", "Task 1", "", 0)
	task2 := models.NewTask("project-1", "Task 2", "", 0)
	task3 := models.NewTask("project-2", "Task 3", "", 0)

	storage.SaveTask(task1)
	storage.SaveTask(task2)
	storage.SaveTask(task3)

	tasks, err := storage.GetTasksByProjectID("project-1")
	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks for project-1, got %d", len(tasks))
	}
}

func TestSaveAndLoadActivities(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	activities := []models.Activity{
		{ID: "coding", Name: "Coding"},
		{ID: "review", Name: "Code Review"},
	}

	err := storage.SaveActivities(activities)
	if err != nil {
		t.Fatalf("Failed to save activities: %v", err)
	}

	loaded, err := storage.LoadActivities()
	if err != nil {
		t.Fatalf("Failed to load activities: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("Expected 2 activities, got %d", len(loaded))
	}
}

func TestDefaultActivities(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Without saving any activities, load should return defaults
	activities, err := storage.LoadActivities()
	if err != nil {
		t.Fatalf("Failed to load activities: %v", err)
	}

	if len(activities) == 0 {
		t.Error("Expected default activities, got none")
	}

	// Check for some expected defaults
	hasCoding := false
	for _, a := range activities {
		if a.ID == "coding" {
			hasCoding = true
			break
		}
	}
	if !hasCoding {
		t.Error("Expected 'coding' in default activities")
	}
}

func TestSaveAndLoadActiveTimeEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	active := &models.ActiveTimeEntry{
		ID:           "entry-123",
		ProjectID:    "project-1",
		ActivityID:   "coding",
		Description:  "Working on feature",
		StartTime:    time.Now(),
		ProjectName:  "Test Project",
		ActivityName: "Coding",
	}

	err := storage.SaveActiveTimeEntry(active)
	if err != nil {
		t.Fatalf("Failed to save active entry: %v", err)
	}

	loaded, err := storage.LoadActiveTimeEntry()
	if err != nil {
		t.Fatalf("Failed to load active entry: %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected active entry, got nil")
	}

	if loaded.ID != active.ID {
		t.Errorf("Active entry ID mismatch: expected %s, got %s", active.ID, loaded.ID)
	}
}

func TestClearActiveTimeEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Save an active entry
	active := &models.ActiveTimeEntry{
		ID:         "entry-123",
		ProjectID:  "project-1",
		ActivityID: "coding",
		StartTime:  time.Now(),
	}
	storage.SaveActiveTimeEntry(active)

	// Clear it by saving nil
	err := storage.SaveActiveTimeEntry(nil)
	if err != nil {
		t.Fatalf("Failed to clear active entry: %v", err)
	}

	// Should return nil now
	loaded, err := storage.LoadActiveTimeEntry()
	if err != nil {
		t.Fatalf("Error loading after clear: %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil after clearing active entry")
	}
}

func TestSaveAndLoadCodeRepoAssociations(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	assocs := models.NewCodeRepoAssociations()
	assocs.Associations = append(assocs.Associations, models.CodeRepoAssociation{
		ProjectID:   1,
		ProjectName: "Test Project",
		RepoURL:     "github.com/test/repo",
		RepoName:    "repo",
		RepoOwner:   "test",
	})

	err := storage.SaveCodeRepoAssociations(assocs)
	if err != nil {
		t.Fatalf("Failed to save code repo associations: %v", err)
	}

	loaded, err := storage.LoadCodeRepoAssociations()
	if err != nil {
		t.Fatalf("Failed to load code repo associations: %v", err)
	}

	if len(loaded.Associations) != 1 {
		t.Errorf("Expected 1 association, got %d", len(loaded.Associations))
	}

	if loaded.Associations[0].RepoOwner != "test" {
		t.Errorf("Repo owner mismatch: got %s", loaded.Associations[0].RepoOwner)
	}
}

func TestSaveAndLoadTimelogCache(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	entries := []api.TimelogEntry{
		{
			ID:           1,
			ProjectID:    1,
			ProjectName:  "Test Project",
			ActivityType: "Coding",
			FromTime:     "2024-01-01T09:00:00Z",
			ToTime:       "2024-01-01T17:00:00Z",
			Hours:        8.0,
		},
	}

	err := storage.SaveTimelogCache(entries)
	if err != nil {
		t.Fatalf("Failed to save timelog cache: %v", err)
	}

	loaded, err := storage.LoadTimelogCache()
	if err != nil {
		t.Fatalf("Failed to load timelog cache: %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected cache, got nil")
	}

	if len(loaded.Entries) != 1 {
		t.Errorf("Expected 1 cached entry, got %d", len(loaded.Entries))
	}

	if loaded.Entries[0].ProjectName != "Test Project" {
		t.Errorf("Project name mismatch: got %s", loaded.Entries[0].ProjectName)
	}

	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestUpdateTimelogCacheEntry(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	entries := []api.TimelogEntry{
		{
			ID:           1,
			ProjectID:    1,
			ProjectName:  "Test",
			ActivityType: "Coding",
			FromTime:     "2024-01-01T09:00:00Z",
			ToTime:       "2024-01-01T10:00:00Z",
			Hours:        1.0,
		},
	}
	storage.SaveTimelogCache(entries)

	// Update the entry
	err := storage.UpdateTimelogCacheEntry(1, "Updated description", "2024-01-01T09:00:00Z", "2024-01-01T12:00:00Z", 3.0)
	if err != nil {
		t.Fatalf("Failed to update cache entry: %v", err)
	}

	loaded, _ := storage.LoadTimelogCache()
	if loaded.Entries[0].Hours != 3.0 {
		t.Errorf("Hours not updated: got %f", loaded.Entries[0].Hours)
	}
}
