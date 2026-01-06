package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
)

// Storage provides persistent storage for timesheet data
type Storage struct {
	dataDir string
}

// New creates a new storage instance
func New(dataDir string) (*Storage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &Storage{
		dataDir: dataDir,
	}, nil
}

// SaveTimeEntry saves a time entry to persistent storage
func (s *Storage) SaveTimeEntry(entry *models.TimeEntry) error {
	filePath := filepath.Join(s.dataDir, "time_entries.json")
	
	var entries []models.TimeEntry
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("failed to unmarshal existing entries: %w", err)
		}
	}

	// Update existing entry or add new one
	found := false
	for i, e := range entries {
		if e.ID == entry.ID {
			entries[i] = *entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, *entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal entries: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadTimeEntries loads all time entries from storage
func (s *Storage) LoadTimeEntries() ([]models.TimeEntry, error) {
	filePath := filepath.Join(s.dataDir, "time_entries.json")
	
	var entries []models.TimeEntry
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read time entries file: %w", err)
	}

	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal time entries: %w", err)
	}

	return entries, nil
}

// GetTimeEntriesByDateRange returns time entries within a date range
func (s *Storage) GetTimeEntriesByDateRange(start, end time.Time) ([]models.TimeEntry, error) {
	entries, err := s.LoadTimeEntries()
	if err != nil {
		return nil, err
	}

	var filtered []models.TimeEntry
	for _, entry := range entries {
		if (entry.StartTime.After(start) || entry.StartTime.Equal(start)) &&
			entry.StartTime.Before(end) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// SaveProject saves a project to persistent storage
func (s *Storage) SaveProject(project *models.Project) error {
	filePath := filepath.Join(s.dataDir, "projects.json")
	
	var projects []models.Project
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &projects); err != nil {
			return fmt.Errorf("failed to unmarshal existing projects: %w", err)
		}
	}

	// Update existing project or add new one
	found := false
	for i, p := range projects {
		if p.ID == project.ID {
			projects[i] = *project
			found = true
			break
		}
	}
	if !found {
		projects = append(projects, *project)
	}

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal projects: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadProjects loads all projects from storage
func (s *Storage) LoadProjects() ([]models.Project, error) {
	filePath := filepath.Join(s.dataDir, "projects.json")
	
	var projects []models.Project
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return projects, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read projects file: %w", err)
	}

	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal projects: %w", err)
	}

	return projects, nil
}

// SaveTask saves a task to persistent storage
func (s *Storage) SaveTask(task *models.Task) error {
	filePath := filepath.Join(s.dataDir, "tasks.json")
	
	var tasks []models.Task
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &tasks); err != nil {
			return fmt.Errorf("failed to unmarshal existing tasks: %w", err)
		}
	}

	// Update existing task or add new one
	found := false
	for i, t := range tasks {
		if t.ID == task.ID {
			tasks[i] = *task
			found = true
			break
		}
	}
	if !found {
		tasks = append(tasks, *task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadTasks loads all tasks from storage
func (s *Storage) LoadTasks() ([]models.Task, error) {
	filePath := filepath.Join(s.dataDir, "tasks.json")
	
	var tasks []models.Task
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return tasks, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}

	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	return tasks, nil
}

// GetTasksByProjectID returns tasks for a specific project
func (s *Storage) GetTasksByProjectID(projectID string) ([]models.Task, error) {
	tasks, err := s.LoadTasks()
	if err != nil {
		return nil, err
	}

	var filtered []models.Task
	for _, task := range tasks {
		if task.ProjectID == projectID {
			filtered = append(filtered, task)
		}
	}

	return filtered, nil
}

// SaveActivities saves activities to persistent storage
func (s *Storage) SaveActivities(activities []models.Activity) error {
	filePath := filepath.Join(s.dataDir, "activities.json")
	
	data, err := json.MarshalIndent(activities, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal activities: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadActivities loads all activities from storage
func (s *Storage) LoadActivities() ([]models.Activity, error) {
	filePath := filepath.Join(s.dataDir, "activities.json")
	
	var activities []models.Activity
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// Return default activities if none exist
		return []models.Activity{
			{ID: "coding", Name: "Coding"},
			{ID: "planning", Name: "Planning"},
			{ID: "review", Name: "Review"},
			{ID: "meeting", Name: "Meeting"},
			{ID: "documentation", Name: "Documentation"},
			{ID: "testing", Name: "Testing"},
			{ID: "debugging", Name: "Debugging"},
			{ID: "research", Name: "Research"},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read activities file: %w", err)
	}

	if err := json.Unmarshal(data, &activities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal activities: %w", err)
	}

	return activities, nil
}

// SaveActiveTimeEntry saves the currently active time entry
func (s *Storage) SaveActiveTimeEntry(entry *models.ActiveTimeEntry) error {
	filePath := filepath.Join(s.dataDir, "active_entry.json")
	
	if entry == nil {
		// Remove active entry file
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove active entry file: %w", err)
		}
		return nil
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal active entry: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadActiveTimeEntry loads the currently active time entry
func (s *Storage) LoadActiveTimeEntry() (*models.ActiveTimeEntry, error) {
	filePath := filepath.Join(s.dataDir, "active_entry.json")
	
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read active entry file: %w", err)
	}

	var entry models.ActiveTimeEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal active entry: %w", err)
	}

	return &entry, nil
}

// DeleteTimeEntry removes a time entry from storage
func (s *Storage) DeleteTimeEntry(entryID string) error {
	filePath := filepath.Join(s.dataDir, "time_entries.json")

	var entries []models.TimeEntry
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("failed to unmarshal existing entries: %w", err)
		}
	}

	// Filter out the entry to delete
	var filtered []models.TimeEntry
	for _, entry := range entries {
		if entry.ID != entryID {
			filtered = append(filtered, entry)
		}
	}

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal entries: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}