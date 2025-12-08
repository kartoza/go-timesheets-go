package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
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
		service, err := getService()
		if err != nil {
			printErrorStatus(fmt.Sprintf("Service error: %v", err))
			return
		}

		activeEntry, err := service.GetActiveTimeEntry()
		if err != nil {
			printErrorStatus(fmt.Sprintf("Failed to get active entry: %v", err))
			return
		}

		todaysEntries, err := service.GetTodaysEntries()
		if err != nil {
			printErrorStatus(fmt.Sprintf("Failed to get today's entries: %v", err))
			return
		}

		// Calculate today's total hours
		var totalHours float64
		for _, entry := range todaysEntries {
			totalHours += entry.Duration
		}

		// Add current session if active
		var currentDuration time.Duration
		if activeEntry != nil {
			currentDuration = time.Since(activeEntry.StartTime)
			totalHours += currentDuration.Hours()
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

func createWaybarStatus(activeEntry interface{}, currentDuration time.Duration, totalHours float64) WaybarStatus {
	if activeEntry != nil {
		// Recording state
		elapsed := formatDuration(currentDuration)
		return WaybarStatus{
			Text:    fmt.Sprintf("🔴 %s", elapsed),
			Alt:     "recording",
			Tooltip: fmt.Sprintf("Recording time\nCurrent session: %s\nToday's total: %.2fh", elapsed, totalHours),
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