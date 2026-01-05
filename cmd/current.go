package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// currentCmd represents the current command for getting detailed current timer info
var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Get detailed JSON of currently running timer",
	Long: `Returns detailed JSON information about the currently running time entry.

The output includes:
- Timer ID, project, task, and activity details
- Start time and elapsed duration
- Description and metadata
- Full timer object with all fields

If no timer is running, returns an empty object with a "running" field set to false.

Example output (timer running):
{
  "running": true,
  "id": "entry-123",
  "project": {
    "id": "proj-1",
    "name": "WB GEEST 2"
  },
  "task": {
    "id": "task-3",
    "name": "Task 3: Improved Functionalities"
  },
  "activity": {
    "id": "act-1",
    "name": "Coding"
  },
  "description": "Working on feature X",
  "start_time": "2026-01-05T10:30:00Z",
  "elapsed_seconds": 3665,
  "elapsed_formatted": "1:01:05"
}

Example output (no timer running):
{
  "running": false
}`,
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			printJSONError(fmt.Sprintf("Service error: %v", err))
			return
		}

		activeEntry, err := service.GetActiveTimeEntry()
		if err != nil {
			printJSONError(fmt.Sprintf("Failed to get active entry: %v", err))
			return
		}

		if activeEntry == nil {
			// No timer running
			result := map[string]interface{}{
				"running": false,
			}
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
			return
		}

		// Timer is running - build detailed response
		elapsed := time.Since(activeEntry.StartTime)

		result := map[string]interface{}{
			"running": true,
			"id":      activeEntry.ID,
			"project": map[string]string{
				"id":   activeEntry.ProjectID,
				"name": activeEntry.ProjectName,
			},
			"activity": map[string]string{
				"id":   activeEntry.ActivityID,
				"name": activeEntry.ActivityName,
			},
			"description":       activeEntry.Description,
			"start_time":        activeEntry.StartTime.Format(time.RFC3339),
			"elapsed_seconds":   int(elapsed.Seconds()),
			"elapsed_formatted": formatDuration(elapsed),
		}

		// Add task if present
		if activeEntry.TaskID != nil && activeEntry.TaskName != "" {
			result["task"] = map[string]string{
				"id":   *activeEntry.TaskID,
				"name": activeEntry.TaskName,
			}
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			printJSONError(fmt.Sprintf("JSON error: %v", err))
			return
		}

		fmt.Println(string(jsonData))
	},
}

func printJSONError(message string) {
	result := map[string]interface{}{
		"error":   true,
		"message": message,
	}
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))
}

func init() {
	rootCmd.AddCommand(currentCmd)
}
