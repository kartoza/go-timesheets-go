package models

import (
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	name := "Test Project"
	description := "A test project description"

	project := NewProject(name, description)

	if project == nil {
		t.Fatal("NewProject returned nil")
	}
	if project.ID == "" {
		t.Error("Project ID should not be empty")
	}
	if project.Name != name {
		t.Errorf("Expected name %q, got %q", name, project.Name)
	}
	if project.Description != description {
		t.Errorf("Expected description %q, got %q", description, project.Description)
	}
	if !project.IsActive {
		t.Error("New project should be active by default")
	}
	if project.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestNewTask(t *testing.T) {
	projectID := "project-123"
	name := "Test Task"
	description := "A test task description"
	expectedTime := 8.0

	task := NewTask(projectID, name, description, expectedTime)

	if task == nil {
		t.Fatal("NewTask returned nil")
	}
	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.ProjectID != projectID {
		t.Errorf("Expected project ID %q, got %q", projectID, task.ProjectID)
	}
	if task.Name != name {
		t.Errorf("Expected name %q, got %q", name, task.Name)
	}
	if task.Description != description {
		t.Errorf("Expected description %q, got %q", description, task.Description)
	}
	if task.ExpectedTime != expectedTime {
		t.Errorf("Expected time %f, got %f", expectedTime, task.ExpectedTime)
	}
	if task.ActualTime != 0 {
		t.Error("ActualTime should be 0 for new task")
	}
	if !task.IsActive {
		t.Error("New task should be active by default")
	}
}

func TestNewTimeEntry(t *testing.T) {
	userID := "user-123"
	projectID := "project-456"
	activityID := "coding"
	taskID := "task-789"
	description := "Working on feature X"

	entry := NewTimeEntry(userID, projectID, activityID, &taskID, description)

	if entry == nil {
		t.Fatal("NewTimeEntry returned nil")
	}
	if entry.ID == "" {
		t.Error("TimeEntry ID should not be empty")
	}
	if entry.UserID != userID {
		t.Errorf("Expected user ID %q, got %q", userID, entry.UserID)
	}
	if entry.ProjectID != projectID {
		t.Errorf("Expected project ID %q, got %q", projectID, entry.ProjectID)
	}
	if entry.ActivityID != activityID {
		t.Errorf("Expected activity ID %q, got %q", activityID, entry.ActivityID)
	}
	if entry.TaskID == nil || *entry.TaskID != taskID {
		t.Error("Task ID not set correctly")
	}
	if entry.Description != description {
		t.Errorf("Expected description %q, got %q", description, entry.Description)
	}
	if entry.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
	if entry.EndTime != nil {
		t.Error("EndTime should be nil for new entry")
	}
	if entry.Duration != 0 {
		t.Error("Duration should be 0 for new entry")
	}
	if entry.IsSubmitted {
		t.Error("New entry should not be submitted")
	}
}

func TestNewTimeEntryWithoutTask(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	if entry.TaskID != nil {
		t.Error("TaskID should be nil when not provided")
	}
}

func TestTimeEntryStop(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	// Entry should be running initially
	if !entry.IsRunning() {
		t.Error("New entry should be running")
	}

	// Wait a tiny bit to ensure duration > 0
	time.Sleep(10 * time.Millisecond)

	entry.Stop()

	if entry.IsRunning() {
		t.Error("Entry should not be running after Stop()")
	}
	if entry.EndTime == nil {
		t.Error("EndTime should be set after Stop()")
	}
	if entry.Duration <= 0 {
		t.Error("Duration should be positive after Stop()")
	}
	if entry.UpdatedAt.Before(entry.CreatedAt) {
		t.Error("UpdatedAt should be >= CreatedAt after Stop()")
	}
}

func TestTimeEntryStopIdempotent(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")
	time.Sleep(10 * time.Millisecond)

	entry.Stop()
	firstEndTime := *entry.EndTime
	firstDuration := entry.Duration

	// Call Stop() again - should have no effect
	time.Sleep(10 * time.Millisecond)
	entry.Stop()

	if !entry.EndTime.Equal(firstEndTime) {
		t.Error("EndTime should not change on second Stop() call")
	}
	if entry.Duration != firstDuration {
		t.Error("Duration should not change on second Stop() call")
	}
}

func TestTimeEntryIsRunning(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	if !entry.IsRunning() {
		t.Error("New entry should be running")
	}

	entry.Stop()

	if entry.IsRunning() {
		t.Error("Stopped entry should not be running")
	}
}

func TestTimeEntryGetDurationWhileRunning(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	time.Sleep(50 * time.Millisecond)
	duration1 := entry.GetDuration()

	time.Sleep(50 * time.Millisecond)
	duration2 := entry.GetDuration()

	if duration2 <= duration1 {
		t.Error("Duration should increase while entry is running")
	}
}

func TestTimeEntryGetDurationAfterStop(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	time.Sleep(50 * time.Millisecond)
	entry.Stop()

	duration1 := entry.GetDuration()
	time.Sleep(50 * time.Millisecond)
	duration2 := entry.GetDuration()

	if duration1 != duration2 {
		t.Error("Duration should be fixed after entry is stopped")
	}
}

func TestTimeEntryGetFormattedDuration(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	// Set known start and end times
	entry.StartTime = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 11, 30, 45, 0, time.UTC)
	entry.EndTime = &endTime

	formatted := entry.GetFormattedDuration()

	expected := "01:30:45"
	if formatted != expected {
		t.Errorf("Expected formatted duration %q, got %q", expected, formatted)
	}
}

func TestTimeEntryGetFormattedDurationMultiHour(t *testing.T) {
	entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")

	// Set 12 hours, 5 minutes, 30 seconds
	entry.StartTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 12, 5, 30, 0, time.UTC)
	entry.EndTime = &endTime

	formatted := entry.GetFormattedDuration()

	expected := "12:05:30"
	if formatted != expected {
		t.Errorf("Expected formatted duration %q, got %q", expected, formatted)
	}
}

func TestTimeEntryToActiveTimeEntry(t *testing.T) {
	taskID := "task-123"
	entry := NewTimeEntry("user-1", "project-1", "coding", &taskID, "Test description")
	entry.ProjectName = "Test Project"
	entry.TaskName = "Test Task"
	entry.ActivityName = "Coding"

	active := entry.ToActiveTimeEntry()

	if active == nil {
		t.Fatal("ToActiveTimeEntry returned nil")
	}
	if active.ID != entry.ID {
		t.Error("ID should match")
	}
	if active.ProjectID != entry.ProjectID {
		t.Error("ProjectID should match")
	}
	if active.TaskID == nil || *active.TaskID != *entry.TaskID {
		t.Error("TaskID should match")
	}
	if active.ActivityID != entry.ActivityID {
		t.Error("ActivityID should match")
	}
	if active.Description != entry.Description {
		t.Error("Description should match")
	}
	if !active.StartTime.Equal(entry.StartTime) {
		t.Error("StartTime should match")
	}
	if active.ProjectName != entry.ProjectName {
		t.Error("ProjectName should match")
	}
	if active.TaskName != entry.TaskName {
		t.Error("TaskName should match")
	}
	if active.ActivityName != entry.ActivityName {
		t.Error("ActivityName should match")
	}
}

func TestActivityStruct(t *testing.T) {
	activity := Activity{
		ID:   "coding",
		Name: "Coding",
	}

	if activity.ID != "coding" {
		t.Error("Activity ID not set correctly")
	}
	if activity.Name != "Coding" {
		t.Error("Activity Name not set correctly")
	}
}

func TestUserStruct(t *testing.T) {
	now := time.Now()
	user := User{
		ID:        "user-123",
		Name:      "John Doe",
		Email:     "john@example.com",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if user.ID != "user-123" {
		t.Error("User ID not set correctly")
	}
	if user.Name != "John Doe" {
		t.Error("User Name not set correctly")
	}
	if user.Email != "john@example.com" {
		t.Error("User Email not set correctly")
	}
	if !user.IsActive {
		t.Error("User should be active")
	}
}

func TestTimesheetSubmissionStruct(t *testing.T) {
	now := time.Now()
	entries := []TimeEntry{
		*NewTimeEntry("user-1", "project-1", "coding", nil, "Entry 1"),
		*NewTimeEntry("user-1", "project-1", "review", nil, "Entry 2"),
	}

	submission := TimesheetSubmission{
		ID:          "sub-123",
		UserID:      "user-1",
		Entries:     entries,
		PeriodStart: now.AddDate(0, 0, -7),
		PeriodEnd:   now,
		TotalHours:  16.0,
		SubmittedAt: now,
		Status:      "pending",
	}

	if submission.ID != "sub-123" {
		t.Error("Submission ID not set correctly")
	}
	if len(submission.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(submission.Entries))
	}
	if submission.TotalHours != 16.0 {
		t.Error("TotalHours not set correctly")
	}
	if submission.Status != "pending" {
		t.Error("Status should be pending")
	}
}

func TestUniqueIDs(t *testing.T) {
	// Create multiple projects and ensure unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		project := NewProject("Test", "Description")
		if ids[project.ID] {
			t.Errorf("Duplicate project ID generated: %s", project.ID)
		}
		ids[project.ID] = true
	}

	// Create multiple tasks and ensure unique IDs
	for i := 0; i < 100; i++ {
		task := NewTask("project-1", "Test", "Description", 1.0)
		if ids[task.ID] {
			t.Errorf("Duplicate task ID generated: %s", task.ID)
		}
		ids[task.ID] = true
	}

	// Create multiple time entries and ensure unique IDs
	for i := 0; i < 100; i++ {
		entry := NewTimeEntry("user-1", "project-1", "coding", nil, "Test")
		if ids[entry.ID] {
			t.Errorf("Duplicate time entry ID generated: %s", entry.ID)
		}
		ids[entry.ID] = true
	}
}
