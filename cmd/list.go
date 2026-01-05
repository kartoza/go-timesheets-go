package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	fromDate string
	toDate   string
	week     bool
	month    bool
)

// listCmd represents the list command for getting entries with date filtering
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List time entries with date filtering",
	Long: `Returns a JSON array of time entries filtered by date range.

Date filtering options:
  --from DATE    Start date (format: YYYY-MM-DD)
  --to DATE      End date (format: YYYY-MM-DD)
  --week         Entries from current week (Monday-Sunday)
  --month        Entries from current month

If no filters are specified, returns entries from the last 7 days.

Examples:
  kartoza-timesheet list --week
  kartoza-timesheet list --month
  kartoza-timesheet list --from 2026-01-01 --to 2026-01-05
  kartoza-timesheet list --from 2026-01-01

Output format:
{
  "start_date": "2026-01-01",
  "end_date": "2026-01-05",
  "entries": [
    {
      "id": "entry-123",
      "project": {"id": "proj-1", "name": "WB GEEST 2"},
      "task": {"id": "task-3", "name": "Task 3"},
      "activity": {"id": "act-1", "name": "Coding"},
      "description": "Feature work",
      "start_time": "2026-01-05T09:00:00Z",
      "end_time": "2026-01-05T11:30:00Z",
      "duration_hours": 2.5
    }
  ],
  "summary": {
    "total_entries": 15,
    "total_hours": 38.5,
    "by_project": {
      "WB GEEST 2": 25.0,
      "Project Alpha": 13.5
    },
    "by_activity": {
      "Coding": 20.0,
      "Planning": 10.5,
      "Review": 8.0
    }
  }
}`,
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			printJSONError(fmt.Sprintf("Service error: %v", err))
			return
		}

		// Determine date range
		var startDate, endDate time.Time
		now := time.Now()

		if week {
			// Current week (Monday to Sunday)
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7 // Sunday
			}
			startDate = now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
			endDate = startDate.AddDate(0, 0, 6)
		} else if month {
			// Current month
			startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			endDate = startDate.AddDate(0, 1, -1)
		} else if fromDate != "" || toDate != "" {
			// Custom date range
			if fromDate != "" {
				startDate, err = time.Parse("2006-01-02", fromDate)
				if err != nil {
					printJSONError(fmt.Sprintf("Invalid from date format (use YYYY-MM-DD): %v", err))
					return
				}
			} else {
				// Default to 30 days ago if only --to is specified
				startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
			}

			if toDate != "" {
				endDate, err = time.Parse("2006-01-02", toDate)
				if err != nil {
					printJSONError(fmt.Sprintf("Invalid to date format (use YYYY-MM-DD): %v", err))
					return
				}
			} else {
				// Default to today if only --from is specified
				endDate = now
			}
		} else {
			// Default: last 7 days
			startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
			endDate = now
		}

		// Get entries in date range
		entries, err := service.GetTimeEntries(startDate, endDate)
		if err != nil {
			printJSONError(fmt.Sprintf("Failed to get entries: %v", err))
			return
		}

		// Build entries array and calculate summaries
		var entryList []map[string]interface{}
		var totalHours float64
		byProject := make(map[string]float64)
		byActivity := make(map[string]float64)

		for _, entry := range entries {
			entryData := map[string]interface{}{
				"id": entry.ID,
				"project": map[string]string{
					"id":   entry.ProjectID,
					"name": entry.ProjectName,
				},
				"activity": map[string]string{
					"id":   entry.ActivityID,
					"name": entry.ActivityName,
				},
				"description":    entry.Description,
				"start_time":     entry.StartTime.Format(time.RFC3339),
				"duration_hours": entry.Duration,
			}

			// Add task if present
			if entry.TaskID != nil && entry.TaskName != "" {
				entryData["task"] = map[string]string{
					"id":   *entry.TaskID,
					"name": entry.TaskName,
				}
			}

			// Add end time if present
			if entry.EndTime != nil {
				entryData["end_time"] = entry.EndTime.Format(time.RFC3339)
			}

			entryList = append(entryList, entryData)

			// Update summaries
			totalHours += entry.Duration
			byProject[entry.ProjectName] += entry.Duration
			byActivity[entry.ActivityName] += entry.Duration
		}

		// Build response
		result := map[string]interface{}{
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
			"entries":    entryList,
			"summary": map[string]interface{}{
				"total_entries": len(entryList),
				"total_hours":   totalHours,
				"by_project":    byProject,
				"by_activity":   byActivity,
			},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			printJSONError(fmt.Sprintf("JSON error: %v", err))
			return
		}

		fmt.Println(string(jsonData))
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	listCmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	listCmd.Flags().BoolVar(&week, "week", false, "Current week (Monday-Sunday)")
	listCmd.Flags().BoolVar(&month, "month", false, "Current month")
}
