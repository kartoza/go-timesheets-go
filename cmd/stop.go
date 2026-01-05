package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the currently active time tracking",
	Long: `Stop the currently active time tracking session.

This command will:
- Stop the active time entry if one is running
- Display a summary of the completed session
- Show the total time tracked

Flags:
  --json    Output result as JSON instead of human-readable format

If no time tracking session is active, the command will indicate this.
This command is designed for automation and quick time tracking stops.

Example JSON output:
{
  "success": true,
  "action": "stopped",
  "entry": {
    "id": "entry-123",
    "project": {"id": "proj-1", "name": "WB GEEST 2"},
    "task": {"id": "task-3", "name": "Task 3"},
    "activity": {"id": "act-1", "name": "Coding"},
    "start_time": "2026-01-05T10:00:00Z",
    "end_time": "2026-01-05T12:30:00Z",
    "duration_hours": 2.5
  },
  "today_total_hours": 6.5
}`,
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing service: %v\n", err)
			os.Exit(1)
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")

		// Check if there's an active entry
		activeEntry, err := service.GetActiveTimeEntry()
		if err != nil {
			if jsonOutput {
				result := map[string]interface{}{
					"success": false,
					"error":   "Error checking active entry",
					"details": err.Error(),
				}
				jsonData, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(jsonData))
			} else {
				fmt.Fprintf(os.Stderr, "Error checking active entry: %v\n", err)
			}
			os.Exit(1)
		}

		if activeEntry == nil {
			if jsonOutput {
				result := map[string]interface{}{
					"success": false,
					"error":   "No active timer",
					"message": "No time tracking session is currently running",
				}
				jsonData, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(jsonData))
			} else {
				fmt.Println("⏸️  No active time tracking session")
			}
			return
		}

		// Calculate duration before stopping
		startTime := activeEntry.StartTime
		endTime := time.Now()
		duration := endTime.Sub(startTime).Hours()

		// Stop the active entry
		err = service.StopActiveTimeEntry()
		if err != nil {
			if jsonOutput {
				result := map[string]interface{}{
					"success": false,
					"error":   "Error stopping time entry",
					"details": err.Error(),
				}
				jsonData, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(jsonData))
			} else {
				fmt.Fprintf(os.Stderr, "Error stopping time entry: %v\n", err)
			}
			os.Exit(1)
		}

		// Get today's total for summary
		var totalHours float64
		todaysEntries, err := service.GetTodaysEntries()
		if err == nil {
			for _, entry := range todaysEntries {
				totalHours += entry.Duration
			}
		}

		// Output result
		if jsonOutput {
			result := map[string]interface{}{
				"success": true,
				"action":  "stopped",
				"entry": map[string]interface{}{
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
					"start_time":     startTime.Format(time.RFC3339),
					"end_time":       endTime.Format(time.RFC3339),
					"duration_hours": duration,
				},
				"today_total_hours": totalHours,
			}

			// Add task if present
			if activeEntry.TaskID != nil && activeEntry.TaskName != "" {
				result["entry"].(map[string]interface{})["task"] = map[string]string{
					"id":   *activeEntry.TaskID,
					"name": activeEntry.TaskName,
				}
			}

			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("⏹️  Time tracking stopped!\n")
			fmt.Printf("📊 Project: %s\n", activeEntry.ProjectName)
			if activeEntry.TaskName != "" {
				fmt.Printf("📝 Task: %s\n", activeEntry.TaskName)
			}
			fmt.Printf("⚡ Activity: %s\n", activeEntry.ActivityName)
			if activeEntry.Description != "" {
				fmt.Printf("📄 Description: %s\n", activeEntry.Description)
			}
			fmt.Printf("🕐 Started: %s\n", startTime.Format("15:04:05"))
			fmt.Printf("🕐 Stopped: %s\n", endTime.Format("15:04:05"))
			fmt.Printf("⏱️  Duration: %.2f hours\n", duration)
			fmt.Printf("📈 Today's total: %.2f hours\n", totalHours)
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().Bool("json", false, "Output result as JSON")
}