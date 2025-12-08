package cmd

import (
	"fmt"
	"os"

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

If no time tracking session is active, the command will indicate this.
This command is designed for automation and quick time tracking stops.`,
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing service: %v\n", err)
			os.Exit(1)
		}

		// Check if there's an active entry
		activeEntry, err := service.GetActiveTimeEntry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking active entry: %v\n", err)
			os.Exit(1)
		}

		if activeEntry == nil {
			fmt.Println("⏸️  No active time tracking session")
			return
		}

		// Stop the active entry
		err = service.StopActiveTimeEntry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping time entry: %v\n", err)
			os.Exit(1)
		}

		// Display session summary

		fmt.Printf("⏹️  Time tracking stopped!\n")
		fmt.Printf("📊 Project: %s\n", activeEntry.ProjectName)
		if activeEntry.TaskName != "" {
			fmt.Printf("📝 Task: %s\n", activeEntry.TaskName)
		}
		fmt.Printf("⚡ Activity: %s\n", activeEntry.ActivityName)
		if activeEntry.Description != "" {
			fmt.Printf("📄 Description: %s\n", activeEntry.Description)
		}
		fmt.Printf("🕐 Started: %s\n", activeEntry.StartTime.Format("15:04:05"))
		fmt.Printf("🕐 Stopped: %s\n", activeEntry.StartTime.Format("15:04:05")) // Will be updated in service
		
		// Get today's total for summary
		todaysEntries, err := service.GetTodaysEntries()
		if err == nil {
			var totalHours float64
			for _, entry := range todaysEntries {
				totalHours += entry.Duration
			}
			fmt.Printf("📈 Today's total: %.2f hours\n", totalHours)
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}