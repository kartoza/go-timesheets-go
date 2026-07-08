package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// TimesheetService provides business logic for timesheet operations
type TimesheetService struct {
	storage *storage.Storage
	userID  string
}

// New creates a new timesheet service
func New(storage *storage.Storage, userID string) *TimesheetService {
	return &TimesheetService{
		storage: storage,
		userID:  userID,
	}
}

// StartTimeEntry starts a new time entry
func (ts *TimesheetService) StartTimeEntry(projectID, activityID string, taskID *string, description string) (*models.TimeEntry, error) {
	// Stop any currently active time entry first
	if err := ts.StopActiveTimeEntry(); err != nil {
		return nil, fmt.Errorf("failed to stop active time entry: %w", err)
	}

	// Create new time entry
	entry := models.NewTimeEntry(ts.userID, projectID, activityID, taskID, description)

	// Populate display names
	if err := ts.populateTimeEntryNames(entry); err != nil {
		return nil, fmt.Errorf("failed to populate entry names: %w", err)
	}

	// Save time entry
	if err := ts.storage.SaveTimeEntry(entry); err != nil {
		return nil, fmt.Errorf("failed to save time entry: %w", err)
	}

	// Save as active entry
	activeEntry := entry.ToActiveTimeEntry()
	if err := ts.storage.SaveActiveTimeEntry(activeEntry); err != nil {
		return nil, fmt.Errorf("failed to save active entry: %w", err)
	}

	return entry, nil
}

// StopActiveTimeEntry stops the currently active time entry
func (ts *TimesheetService) StopActiveTimeEntry() error {
	activeEntry, err := ts.storage.LoadActiveTimeEntry()
	if err != nil {
		return fmt.Errorf("failed to load active entry: %w", err)
	}

	if activeEntry == nil {
		return nil // No active entry to stop
	}

	// Load the full time entry
	entries, err := ts.storage.LoadTimeEntries()
	if err != nil {
		return fmt.Errorf("failed to load time entries: %w", err)
	}

	var entry *models.TimeEntry
	for i, e := range entries {
		if e.ID == activeEntry.ID {
			entry = &entries[i]
			break
		}
	}

	if entry == nil {
		return fmt.Errorf("active entry not found in time entries")
	}

	// Stop the entry
	entry.Stop()

	// Save the updated entry
	if err := ts.storage.SaveTimeEntry(entry); err != nil {
		return fmt.Errorf("failed to save stopped entry: %w", err)
	}

	// Remove active entry
	if err := ts.storage.SaveActiveTimeEntry(nil); err != nil {
		return fmt.Errorf("failed to clear active entry: %w", err)
	}

	return nil
}

// GetActiveTimeEntry returns the currently active time entry
func (ts *TimesheetService) GetActiveTimeEntry() (*models.ActiveTimeEntry, error) {
	return ts.storage.LoadActiveTimeEntry()
}

// GetTimeEntries returns time entries for a date range
func (ts *TimesheetService) GetTimeEntries(start, end time.Time) ([]models.TimeEntry, error) {
	entries, err := ts.storage.GetTimeEntriesByDateRange(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load time entries: %w", err)
	}

	// Filter by user ID and populate display names
	var userEntries []models.TimeEntry
	for _, entry := range entries {
		if entry.UserID == ts.userID {
			if err := ts.populateTimeEntryNames(&entry); err != nil {
				continue // Skip entries with missing references
			}
			userEntries = append(userEntries, entry)
		}
	}

	// Sort by start time (newest first)
	sort.Slice(userEntries, func(i, j int) bool {
		return userEntries[i].StartTime.After(userEntries[j].StartTime)
	})

	return userEntries, nil
}

// GetTodaysEntries returns today's time entries
func (ts *TimesheetService) GetTodaysEntries() ([]models.TimeEntry, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)

	return ts.GetTimeEntries(start, end)
}

// GetWeeklyEntries returns this week's time entries (grouped by day)
func (ts *TimesheetService) GetWeeklyEntries() (map[string][]models.TimeEntry, error) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}

	// Start of week (Monday)
	start := now.AddDate(0, 0, -weekday+1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

	// End of week (Sunday)
	end := start.AddDate(0, 0, 7)

	entries, err := ts.GetTimeEntries(start, end)
	if err != nil {
		return nil, err
	}

	// Group by day
	grouped := make(map[string][]models.TimeEntry)
	for _, entry := range entries {
		day := entry.StartTime.Format("2006-01-02")
		grouped[day] = append(grouped[day], entry)
	}

	return grouped, nil
}

// SubmitTimeEntries submits time entries for a period
func (ts *TimesheetService) SubmitTimeEntries(entryIDs []string) (*models.TimesheetSubmission, error) {
	entries, err := ts.storage.LoadTimeEntries()
	if err != nil {
		return nil, fmt.Errorf("failed to load time entries: %w", err)
	}

	var submissionEntries []models.TimeEntry
	var totalHours float64
	var periodStart, periodEnd time.Time

	for _, entryID := range entryIDs {
		for i, entry := range entries {
			if entry.ID == entryID && entry.UserID == ts.userID && !entry.IsSubmitted {
				entry.IsSubmitted = true
				entries[i] = entry // Update the original slice

				if err := ts.populateTimeEntryNames(&entry); err != nil {
					continue
				}

				submissionEntries = append(submissionEntries, entry)
				totalHours += entry.Duration

				if periodStart.IsZero() || entry.StartTime.Before(periodStart) {
					periodStart = entry.StartTime
				}
				if periodEnd.IsZero() || entry.StartTime.After(periodEnd) {
					periodEnd = entry.StartTime
				}
			}
		}
	}

	if len(submissionEntries) == 0 {
		return nil, fmt.Errorf("no valid entries found for submission")
	}

	// Save updated entries
	for _, entry := range submissionEntries {
		if err := ts.storage.SaveTimeEntry(&entry); err != nil {
			return nil, fmt.Errorf("failed to save submitted entry: %w", err)
		}
	}

	submission := &models.TimesheetSubmission{
		ID:          fmt.Sprintf("sub_%d", time.Now().Unix()),
		UserID:      ts.userID,
		Entries:     submissionEntries,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TotalHours:  totalHours,
		SubmittedAt: time.Now().UTC(),
		Status:      "pending",
	}

	return submission, nil
}

// GetProjects returns all active projects
func (ts *TimesheetService) GetProjects() ([]models.Project, error) {
	projects, err := ts.storage.LoadProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	// Filter active projects and sort by name
	var activeProjects []models.Project
	for _, project := range projects {
		if project.IsActive {
			// Load tasks for the project
			tasks, _ := ts.storage.GetTasksByProjectID(project.ID)
			project.Tasks = tasks
			activeProjects = append(activeProjects, project)
		}
	}

	sort.Slice(activeProjects, func(i, j int) bool {
		return activeProjects[i].Name < activeProjects[j].Name
	})

	return activeProjects, nil
}

// GetTasks returns tasks for a specific project
func (ts *TimesheetService) GetTasks(projectID string) ([]models.Task, error) {
	tasks, err := ts.storage.GetTasksByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}

	// Filter active tasks and sort by name
	var activeTasks []models.Task
	for _, task := range tasks {
		if task.IsActive {
			activeTasks = append(activeTasks, task)
		}
	}

	sort.Slice(activeTasks, func(i, j int) bool {
		return activeTasks[i].Name < activeTasks[j].Name
	})

	return activeTasks, nil
}

// GetActivities returns all activities
func (ts *TimesheetService) GetActivities() ([]models.Activity, error) {
	activities, err := ts.storage.LoadActivities()
	if err != nil {
		return nil, fmt.Errorf("failed to load activities: %w", err)
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Name < activities[j].Name
	})

	return activities, nil
}

// CreateProject creates a new project
func (ts *TimesheetService) CreateProject(name, description string) (*models.Project, error) {
	project := models.NewProject(name, description)

	if err := ts.storage.SaveProject(project); err != nil {
		return nil, fmt.Errorf("failed to save project: %w", err)
	}

	return project, nil
}

// CreateTask creates a new task
func (ts *TimesheetService) CreateTask(projectID, name, description string, expectedTime float64) (*models.Task, error) {
	task := models.NewTask(projectID, name, description, expectedTime)

	if err := ts.storage.SaveTask(task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	return task, nil
}

// DeleteTimeEntry deletes a time entry
func (ts *TimesheetService) DeleteTimeEntry(entryID string) error {
	return ts.storage.DeleteTimeEntry(entryID)
}

// populateTimeEntryNames fills in the display names for project, task, and activity
func (ts *TimesheetService) populateTimeEntryNames(entry *models.TimeEntry) error {
	// Get project name
	projects, err := ts.storage.LoadProjects()
	if err != nil {
		return err
	}
	for _, project := range projects {
		if project.ID == entry.ProjectID {
			entry.ProjectName = project.Name
			break
		}
	}

	// Get task name if task ID is provided
	if entry.TaskID != nil {
		tasks, err := ts.storage.LoadTasks()
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if task.ID == *entry.TaskID {
				entry.TaskName = task.Name
				break
			}
		}
	}

	// Get activity name
	activities, err := ts.storage.LoadActivities()
	if err != nil {
		return err
	}
	for _, activity := range activities {
		if activity.ID == entry.ActivityID {
			entry.ActivityName = activity.Name
			break
		}
	}

	return nil
}
