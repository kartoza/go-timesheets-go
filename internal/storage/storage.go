package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

// atomicWriteFile writes data to a file atomically by writing to a temporary
// file first, then renaming. This prevents corruption from concurrent writes
// or crashes mid-write.
func atomicWriteFile(filePath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, filepath.Base(filePath)+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

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

	return atomicWriteFile(filePath, data, 0644)
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

	return atomicWriteFile(filePath, data, 0644)
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

	return atomicWriteFile(filePath, data, 0644)
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

	return atomicWriteFile(filePath, data, 0644)
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

	return atomicWriteFile(filePath, data, 0644)
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

	return atomicWriteFile(filePath, data, 0644)
}

// SaveCodeRepoAssociations saves code repo associations to persistent storage
func (s *Storage) SaveCodeRepoAssociations(associations *models.CodeRepoAssociations) error {
	filePath := filepath.Join(s.dataDir, "code_repo_associations.json")

	data, err := json.MarshalIndent(associations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal code repo associations: %w", err)
	}

	return atomicWriteFile(filePath, data, 0644)
}

// LoadCodeRepoAssociations loads code repo associations from storage
func (s *Storage) LoadCodeRepoAssociations() (*models.CodeRepoAssociations, error) {
	filePath := filepath.Join(s.dataDir, "code_repo_associations.json")

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// Return empty associations if file doesn't exist
		return models.NewCodeRepoAssociations(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read code repo associations file: %w", err)
	}

	var associations models.CodeRepoAssociations
	if err := json.Unmarshal(data, &associations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal code repo associations: %w", err)
	}

	return &associations, nil
}

// TimelogCache represents a cached copy of timelog entries from the API
type TimelogCache struct {
	Entries   []api.TimelogEntry `json:"entries"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// SaveTimelogCache saves timelog entries to the cache file
func (s *Storage) SaveTimelogCache(entries []api.TimelogEntry) error {
	filePath := filepath.Join(s.dataDir, "timelog_cache.json")

	cache := TimelogCache{
		Entries:   entries,
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timelog cache: %w", err)
	}

	return atomicWriteFile(filePath, data, 0644)
}

// LoadTimelogCache loads timelog entries from the cache file
func (s *Storage) LoadTimelogCache() (*TimelogCache, error) {
	filePath := filepath.Join(s.dataDir, "timelog_cache.json")

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil, nil // No cache exists
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read timelog cache file: %w", err)
	}

	var cache TimelogCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal timelog cache: %w", err)
	}

	return &cache, nil
}

// UpdateTimelogCacheEntry updates a single entry in the timelog cache
func (s *Storage) UpdateTimelogCacheEntry(entryID int, description string, fromTime string, toTime string, hours float64) error {
	cache, err := s.LoadTimelogCache()
	if err != nil {
		return err
	}
	if cache == nil {
		return nil // No cache to update
	}

	// Find and update the entry
	for i := range cache.Entries {
		if cache.Entries[i].ID == entryID {
			cache.Entries[i].Description = &description
			cache.Entries[i].FromTime = fromTime
			cache.Entries[i].ToTime = toTime
			cache.Entries[i].Hours = hours
			// Also update the "all" fields which track aggregated time
			cache.Entries[i].AllFromTime = formatTimeForAllField(fromTime)
			cache.Entries[i].AllToTime = formatTimeForAllField(toTime)
			cache.Entries[i].AllHours = hours
			break
		}
	}

	cache.UpdatedAt = time.Now()

	// Save the updated cache
	return s.SaveTimelogCache(cache.Entries)
}

// formatTimeForAllField converts RFC3339 time to "YYYY-MM-DD HH:MM:SS" format for all_* fields
func formatTimeForAllField(rfc3339Time string) string {
	t, err := time.Parse(time.RFC3339, rfc3339Time)
	if err != nil {
		return rfc3339Time // Return as-is if parsing fails
	}
	return t.Format("2006-01-02 15:04:05")
}

// PowVideoMapping maps timesheet entry IDs to POW video file paths
type PowVideoMapping struct {
	Entries   map[int]string `json:"entries"` // Maps entry ID to video file path
	UpdatedAt time.Time      `json:"updated_at"`
}

// SavePowVideoPath saves a POW video path for a specific timesheet entry ID
func (s *Storage) SavePowVideoPath(entryID int, videoPath string) error {
	filePath := filepath.Join(s.dataDir, "pow_videos.json")

	mapping, err := s.LoadPowVideoMapping()
	if err != nil {
		return err
	}

	if mapping.Entries == nil {
		mapping.Entries = make(map[int]string)
	}

	mapping.Entries[entryID] = videoPath
	mapping.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal POW video mapping: %w", err)
	}

	return atomicWriteFile(filePath, data, 0644)
}

// GetPowVideoPath retrieves the POW video path for a specific timesheet entry ID
func (s *Storage) GetPowVideoPath(entryID int) (string, error) {
	mapping, err := s.LoadPowVideoMapping()
	if err != nil {
		return "", err
	}

	if mapping.Entries == nil {
		return "", nil
	}

	return mapping.Entries[entryID], nil
}

// LoadPowVideoMapping loads the POW video mapping from storage
func (s *Storage) LoadPowVideoMapping() (*PowVideoMapping, error) {
	filePath := filepath.Join(s.dataDir, "pow_videos.json")

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return &PowVideoMapping{
			Entries:   make(map[int]string),
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read POW video mapping file: %w", err)
	}

	var mapping PowVideoMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal POW video mapping: %w", err)
	}

	if mapping.Entries == nil {
		mapping.Entries = make(map[int]string)
	}

	return &mapping, nil
}

// SaveFavouriteAssociations saves favourite associations to persistent storage
func (s *Storage) SaveFavouriteAssociations(associations *models.FavouriteAssociations) error {
	filePath := filepath.Join(s.dataDir, "favourite_associations.json")

	data, err := json.MarshalIndent(associations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal favourite associations: %w", err)
	}

	return atomicWriteFile(filePath, data, 0644)
}

// LoadFavouriteAssociations loads favourite associations from storage
func (s *Storage) LoadFavouriteAssociations() (*models.FavouriteAssociations, error) {
	filePath := filepath.Join(s.dataDir, "favourite_associations.json")

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// Return default associations if file doesn't exist
		return models.NewFavouriteAssociations(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read favourite associations file: %w", err)
	}

	var associations models.FavouriteAssociations
	if err := json.Unmarshal(data, &associations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal favourite associations: %w", err)
	}

	// Ensure we have all 9 slots
	if len(associations.Associations) < 9 {
		defaults := models.NewFavouriteAssociations()
		for i := len(associations.Associations); i < 9; i++ {
			associations.Associations = append(associations.Associations, defaults.Associations[i])
		}
	}

	return &associations, nil
}
