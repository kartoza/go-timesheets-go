package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/kartoza/go-timesheets-go/internal/api"
)

// statusCmd represents the status command for waybar integration
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get current timesheet status for waybar integration",
	Long: `Returns JSON formatted status information suitable for waybar custom modules.
	
The status includes:
- Current tracking state (idle/recording)
- Active project and task information
- Elapsed time for current session
- Today's total hours
	
Example waybar configuration:
{
    "custom/timesheet": {
        "exec": "kartoza-timesheet status",
        "return-type": "json",
        "interval": 5,
        "on-click": "kartoza-timesheet"
    }
}`,
	Run: func(cmd *cobra.Command, args []string) {
		// Use API client to fetch real-time status from server
		client, err := getAPIClient()
		if err != nil {
			printErrorStatus(fmt.Sprintf("API client error: %v", err))
			return
		}

		// Get active timesheet from API
		activeEntry, err := client.GetActiveTimesheet()
		if err != nil {
			printErrorStatus(fmt.Sprintf("Failed to get active timesheet: %v", err))
			return
		}

		// Get all timelogs to calculate today's total
		timelogs, err := client.GetTimelogs()
		if err != nil {
			printErrorStatus(fmt.Sprintf("Failed to get timelogs: %v", err))
			return
		}

		// Calculate today's total hours
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		var totalHours float64

		for _, entry := range timelogs {
			entryStart, err := entry.GetFromTimeAsTime()
			if err != nil {
				continue
			}

			// Only count entries from today
			if entryStart.After(todayStart) || entryStart.Equal(todayStart) {
				totalHours += entry.Duration()
			}
		}

		// Calculate current session duration if active
		var currentDuration time.Duration
		if activeEntry != nil {
			startTime, err := activeEntry.GetFromTimeAsTime()
			if err == nil {
				currentDuration = time.Since(startTime)
			}
		}

		status := createWaybarStatus(activeEntry, currentDuration, totalHours)

		jsonData, err := json.Marshal(status)
		if err != nil {
			printErrorStatus(fmt.Sprintf("JSON error: %v", err))
			return
		}

		fmt.Println(string(jsonData))
	},
}

// WaybarStatus represents the status information for waybar
type WaybarStatus struct {
	Text     string `json:"text"`
	Alt      string `json:"alt"`
	Tooltip  string `json:"tooltip"`
	Class    string `json:"class"`
	Icon     string `json:"icon,omitempty"`
}

func createWaybarStatus(activeEntry *api.TimelogEntry, currentDuration time.Duration, totalHours float64) WaybarStatus {
	if activeEntry != nil {
		// Recording state
		elapsed := formatDuration(currentDuration)

		// Build detailed tooltip with project/task/activity information
		tooltipLines := []string{"Recording time"}

		// Add project information
		if activeEntry.ProjectName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("Project: %s", activeEntry.ProjectName))
		}

		// Add task information
		if activeEntry.TaskName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("Task: %s", activeEntry.TaskName))
		}

		// Add activity information
		if activeEntry.ActivityType != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("Activity: %s", activeEntry.ActivityType))
		}

		// Add timing information
		tooltipLines = append(tooltipLines, fmt.Sprintf("Current session: %s", elapsed))
		tooltipLines = append(tooltipLines, fmt.Sprintf("Today's total: %.2fh", totalHours))

		tooltip := ""
		for i, line := range tooltipLines {
			if i > 0 {
				tooltip += "\n"
			}
			tooltip += line
		}

		return WaybarStatus{
			Text:    fmt.Sprintf("🔴 %s", elapsed),
			Alt:     "recording",
			Tooltip: tooltip,
			Class:   "recording",
			Icon:    "🔴",
		}
	}

	// Idle state
	return WaybarStatus{
		Text:    fmt.Sprintf("⏸️ %.1fh", totalHours),
		Alt:     "idle",
		Tooltip: fmt.Sprintf("Timesheet idle\nToday's total: %.2fh", totalHours),
		Class:   "idle",
		Icon:    "⏸️",
	}
}

func printErrorStatus(message string) {
	status := WaybarStatus{
		Text:    "❌ Error",
		Alt:     "error",
		Tooltip: message,
		Class:   "error",
		Icon:    "❌",
	}
	
	jsonData, _ := json.Marshal(status)
	fmt.Println(string(jsonData))
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}