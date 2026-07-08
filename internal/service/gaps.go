package service

import (
	"sort"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
)

// DayData holds the data for a single day's bar in the gaps view
type DayData struct {
	Date         time.Time
	TotalHours   float64
	GapHours     float64                  // Hours missing to reach target
	ProjectHours map[int]*ProjectHourData // projectID -> hours data
}

// ProjectHourData holds hours data for a project on a specific day
type ProjectHourData struct {
	ProjectID   int
	ProjectName string
	Hours       float64
	Entries     []EntryDetail // Individual entries for this project on this day
}

// EntryDetail holds details of a single time entry
type EntryDetail struct {
	ID           int
	TaskName     string
	ActivityName string
	Hours        float64
	Description  string
	Submitted    bool
}

// GapsService provides gap calculation logic shared between TUI and GUI
type GapsService struct {
	TargetHours float64
}

// NewGapsService creates a new gaps service with the specified target hours per day
func NewGapsService(targetHours float64) *GapsService {
	return &GapsService{
		TargetHours: targetHours,
	}
}

// CalculateGaps processes timelog entries and returns gap data for the last N days
func (s *GapsService) CalculateGaps(entries []api.TimelogEntry, days int) []DayData {
	dayData := make([]DayData, days)
	now := time.Now()

	// Initialize the last N days
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -(days-1)+i)
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		dayData[i] = DayData{
			Date:         day,
			TotalHours:   0,
			GapHours:     s.TargetHours,
			ProjectHours: make(map[int]*ProjectHourData),
		}
	}

	// Process entries and bucket them by day
	for _, entry := range entries {
		fromTime, err := entry.GetFromTimeAsTime()
		if err != nil {
			continue
		}
		fromTime = fromTime.Local()
		entryDate := time.Date(fromTime.Year(), fromTime.Month(), fromTime.Day(), 0, 0, 0, 0, fromTime.Location())

		// Find the matching day
		for i := range dayData {
			if dayData[i].Date.Equal(entryDate) {
				dayData[i].TotalHours += entry.Hours

				// Create entry detail
				detail := EntryDetail{
					ID:           entry.ID,
					TaskName:     entry.TaskName,
					ActivityName: entry.ActivityType,
					Hours:        entry.Hours,
					Description:  entry.GetDescriptionString(),
					Submitted:    entry.Submitted,
				}

				// Add to project hours
				if existing, ok := dayData[i].ProjectHours[entry.ProjectID]; ok {
					existing.Hours += entry.Hours
					existing.Entries = append(existing.Entries, detail)
				} else {
					dayData[i].ProjectHours[entry.ProjectID] = &ProjectHourData{
						ProjectID:   entry.ProjectID,
						ProjectName: entry.ProjectName,
						Hours:       entry.Hours,
						Entries:     []EntryDetail{detail},
					}
				}
				break
			}
		}
	}

	// Calculate gap hours for each day
	for i := range dayData {
		dayData[i].GapHours = s.TargetHours - dayData[i].TotalHours
		if dayData[i].GapHours < 0 {
			dayData[i].GapHours = 0
		}
	}

	return dayData
}

// GetSortedProjects returns projects sorted by hours (descending) for a given day
func (s *GapsService) GetSortedProjects(day *DayData) []*ProjectHourData {
	var projectList []*ProjectHourData
	for _, p := range day.ProjectHours {
		projectList = append(projectList, p)
	}
	sort.Slice(projectList, func(a, b int) bool {
		return projectList[a].Hours > projectList[b].Hours
	})
	return projectList
}

// CalculateGapStartTime calculates the start time for filling a gap
// If there are existing entries, start after the last one
// Otherwise, start at the default start hour (e.g., 9:00 AM)
func (s *GapsService) CalculateGapStartTime(day *DayData, defaultStartHour int) time.Time {
	if day.TotalHours > 0 {
		// Start after existing hours (approximate)
		startTime := time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(), defaultStartHour, 0, 0, 0, day.Date.Location())
		return startTime.Add(time.Duration(day.TotalHours * float64(time.Hour)))
	}
	// No entries for this day, start at default hour
	return time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(), defaultStartHour, 0, 0, 0, day.Date.Location())
}

// CalculateGapEndTime calculates the end time for filling a gap based on the gap hours
// The end time is start time + gap hours (up to 8 hours target)
func (s *GapsService) CalculateGapEndTime(day *DayData, startTime time.Time) time.Time {
	gapHours := day.GapHours
	if gapHours <= 0 {
		gapHours = 1 // Default to 1 hour if no gap
	}
	if gapHours > s.TargetHours {
		gapHours = s.TargetHours
	}
	return startTime.Add(time.Duration(gapHours * float64(time.Hour)))
}

// ProjectColors provides consistent colors for projects
var ProjectColors = []string{
	"#2ECC71", // Green
	"#3498DB", // Blue
	"#9B59B6", // Purple
	"#F39C12", // Orange
	"#1ABC9C", // Teal
	"#E91E63", // Pink
	"#00BCD4", // Cyan
	"#FFC107", // Amber
}

// GetProjectColorIndex returns a color index for a project based on its ID
func GetProjectColorIndex(projectID int) int {
	return projectID % len(ProjectColors)
}

// GetProjectColorHex returns a hex color string for a project based on its ID
func GetProjectColorHex(projectID int) string {
	return ProjectColors[GetProjectColorIndex(projectID)]
}
