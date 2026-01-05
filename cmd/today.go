package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// todayCmd represents the today command for getting all entries from today
var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Get all time entries from today as JSON",
	Long: `Returns a JSON array of all time entries from today, including any currently running timer.

The output includes:
- Array of all completed entries from today
- Currently running entry (if any)
- Summary statistics (total hours, entry count)

Example output:
{
  "date": "2026-01-05",
  "entries": [
    {
      "id": "entry-123",
      "project": {"id": "proj-1", "name": "WB GEEST 2"},
      "task": {"id": "task-3", "name": "Task 3"},
      "activity": {"id": "act-1", "name": "Coding"},
      "description": "Feature work",
      "start_time": "2026-01-05T09:00:00Z",
      "end_time": "2026-01-05T11:30:00Z",
      "duration_hours": 2.5,
      "is_running": false
    },
    {
      "id": "entry-124",
      "project": {"id": "proj-2", "name": "Project Alpha"},
      "activity": {"id": "act-2", "name": "Planning"},
      "description": "Sprint planning",
      "start_time": "2026-01-05T14:00:00Z",
      "end_time": null,
      "duration_hours": 1.5,
      "is_running": true
    }
  ],
  "summary": {
    "total_entries": 2,
    "total_hours": 4.0,
    "running_entry": true
  }
}`,
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			printJSONError(fmt.Sprintf("Service error: %v", err))
			return
		}

		// Get today's completed entries
		todaysEntries, err := service.GetTodaysEntries()
		if err != nil {
			printJSONError(fmt.Sprintf("Failed to get today's entries: %v", err))
			return
		}

		// Get active entry if any
		activeEntry, err := service.GetActiveTimeEntry()
		if err != nil {
			printJSONError(fmt.Sprintf("Failed to get active entry: %v", err))
			return
		}

		// Build entries array
		var entries []map[string]interface{}
		var totalHours float64

		// Add completed entries
		for _, entry := range todaysEntries {
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
				"is_running":     false,
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

			entries = append(entries, entryData)
			totalHours += entry.Duration
		}

		// Add active entry if present
		if activeEntry != nil {
			elapsed := time.Since(activeEntry.StartTime)
			elapsedHours := elapsed.Hours()

			entryData := map[string]interface{}{
				"id": activeEntry.ID,
				"project": map[string]string{
					"id":   activeEntry.ProjectID,
					"name": activeEntry.ProjectName,
				},
				"activity": map[string]string{
					"id":   activeEntry.ActivityID,
					"name": activeEntry.ActivityName,
				},
				"description":    activeEntry.Description,
				"start_time":     activeEntry.StartTime.Format(time.RFC3339),
				"end_time":       nil,
				"duration_hours": elapsedHours,
				"is_running":     true,
			}

			// Add task if present
			if activeEntry.TaskID != nil && activeEntry.TaskName != "" {
				entryData["task"] = map[string]string{
					"id":   *activeEntry.TaskID,
					"name": activeEntry.TaskName,
				}
			}

			entries = append(entries, entryData)
			totalHours += elapsedHours
		}

		// Build response
		result := map[string]interface{}{
			"date":    time.Now().Format("2006-01-02"),
			"entries": entries,
			"summary": map[string]interface{}{
				"total_entries": len(entries),
				"total_hours":   totalHours,
				"running_entry": activeEntry != nil,
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
	rootCmd.AddCommand(todayCmd)
}
