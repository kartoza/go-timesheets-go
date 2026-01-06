package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Activity represents a type of work activity (e.g., "Coding", "Planning", "Review")
type Activity struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// Project represents a project that can have multiple tasks
type Project struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	IsActive    bool      `json:"is_active" yaml:"is_active"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
	Tasks       []Task    `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

// Task represents a specific task within a project
type Task struct {
	ID           string    `json:"id" yaml:"id"`
	ProjectID    string    `json:"project_id" yaml:"project_id"`
	Name         string    `json:"name" yaml:"name"`
	Description  string    `json:"description,omitempty" yaml:"description,omitempty"`
	ExpectedTime float64   `json:"expected_time,omitempty" yaml:"expected_time,omitempty"` // in hours
	ActualTime   float64   `json:"actual_time,omitempty" yaml:"actual_time,omitempty"`     // in hours
	IsActive     bool      `json:"is_active" yaml:"is_active"`
	CreatedAt    time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" yaml:"updated_at"`
}

// TimeEntry represents a single time log entry
type TimeEntry struct {
	ID          string     `json:"id" yaml:"id"`
	UserID      string     `json:"user_id" yaml:"user_id"`
	ProjectID   string     `json:"project_id" yaml:"project_id"`
	TaskID      *string    `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	ActivityID  string     `json:"activity_id" yaml:"activity_id"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	StartTime   time.Time  `json:"start_time" yaml:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration    float64    `json:"duration" yaml:"duration"` // in hours
	IsSubmitted bool       `json:"is_submitted" yaml:"is_submitted"`
	CreatedAt   time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" yaml:"updated_at"`

	// Populated fields for display
	ProjectName  string `json:"project_name,omitempty" yaml:"-"`
	TaskName     string `json:"task_name,omitempty" yaml:"-"`
	ActivityName string `json:"activity_name,omitempty" yaml:"-"`
}

// User represents a user of the timesheet application
type User struct {
	ID        string    `json:"id" yaml:"id"`
	Name      string    `json:"name" yaml:"name"`
	Email     string    `json:"email" yaml:"email"`
	IsActive  bool      `json:"is_active" yaml:"is_active"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// TimesheetSubmission represents a batch of time entries submitted together
type TimesheetSubmission struct {
	ID          string      `json:"id" yaml:"id"`
	UserID      string      `json:"user_id" yaml:"user_id"`
	Entries     []TimeEntry `json:"entries" yaml:"entries"`
	PeriodStart time.Time   `json:"period_start" yaml:"period_start"`
	PeriodEnd   time.Time   `json:"period_end" yaml:"period_end"`
	TotalHours  float64     `json:"total_hours" yaml:"total_hours"`
	SubmittedAt time.Time   `json:"submitted_at" yaml:"submitted_at"`
	Status      string      `json:"status" yaml:"status"` // "pending", "approved", "rejected"
}

// ActiveTimeEntry represents a currently running time entry
type ActiveTimeEntry struct {
	ID          string    `json:"id" yaml:"id"`
	ProjectID   string    `json:"project_id" yaml:"project_id"`
	TaskID      *string   `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	ActivityID  string    `json:"activity_id" yaml:"activity_id"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	StartTime   time.Time `json:"start_time" yaml:"start_time"`

	// Populated fields for display
	ProjectName  string `json:"project_name,omitempty" yaml:"-"`
	TaskName     string `json:"task_name,omitempty" yaml:"-"`
	ActivityName string `json:"activity_name,omitempty" yaml:"-"`
}

// NewProject creates a new project with generated ID and timestamps
func NewProject(name, description string) *Project {
	now := time.Now()
	return &Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewTask creates a new task with generated ID and timestamps
func NewTask(projectID, name, description string, expectedTime float64) *Task {
	now := time.Now()
	return &Task{
		ID:           uuid.New().String(),
		ProjectID:    projectID,
		Name:         name,
		Description:  description,
		ExpectedTime: expectedTime,
		ActualTime:   0,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// NewTimeEntry creates a new time entry with generated ID and timestamps
func NewTimeEntry(userID, projectID, activityID string, taskID *string, description string) *TimeEntry {
	now := time.Now()
	return &TimeEntry{
		ID:          uuid.New().String(),
		UserID:      userID,
		ProjectID:   projectID,
		TaskID:      taskID,
		ActivityID:  activityID,
		Description: description,
		StartTime:   now,
		Duration:    0,
		IsSubmitted: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}


// Stop stops the time entry by setting the end time and calculating duration
func (te *TimeEntry) Stop() {
	if te.EndTime == nil {
		now := time.Now()
		te.EndTime = &now
		te.Duration = now.Sub(te.StartTime).Hours()
		te.UpdatedAt = now
	}
}

// IsRunning returns true if the time entry is currently running (no end time)
func (te *TimeEntry) IsRunning() bool {
	return te.EndTime == nil
}

// GetDuration returns the current duration of the time entry
func (te *TimeEntry) GetDuration() time.Duration {
	if te.EndTime != nil {
		return te.EndTime.Sub(te.StartTime)
	}
	return time.Since(te.StartTime)
}

// GetFormattedDuration returns a formatted duration string (HH:MM:SS)
func (te *TimeEntry) GetFormattedDuration() string {
	d := te.GetDuration()
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// ToActiveTimeEntry converts a TimeEntry to an ActiveTimeEntry
func (te *TimeEntry) ToActiveTimeEntry() *ActiveTimeEntry {
	return &ActiveTimeEntry{
		ID:           te.ID,
		ProjectID:    te.ProjectID,
		TaskID:       te.TaskID,
		ActivityID:   te.ActivityID,
		Description:  te.Description,
		StartTime:    te.StartTime,
		ProjectName:  te.ProjectName,
		TaskName:     te.TaskName,
		ActivityName: te.ActivityName,
	}
}
