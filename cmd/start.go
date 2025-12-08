package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [project] [activity] [task]",
	Short: "Start time tracking for a project",
	Long: `Start time tracking for a specific project and activity.
	
Arguments:
  project   - Project name or ID (required)
  activity  - Activity type (required)
  task      - Task name or ID (optional)

Examples:
  kartoza-timesheet start "WB GEEST 2" "Coding"
  kartoza-timesheet start "WB GEEST 2" "Coding" "Task 3: Improved Functionalities"
  kartoza-timesheet start project1 coding task1

This command is designed for automation and quick time tracking starts.
It will stop any currently active time entry before starting a new one.`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		service, err := getService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing service: %v\n", err)
			os.Exit(1)
		}

		projectName := args[0]
		activityName := args[1]
		var taskName string
		if len(args) > 2 {
			taskName = args[2]
		}

		// Find project by name
		projects, err := service.GetProjects()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading projects: %v\n", err)
			os.Exit(1)
		}

		var projectID string
		for _, project := range projects {
			if project.Name == projectName || project.ID == projectName {
				projectID = project.ID
				break
			}
		}
		if projectID == "" {
			fmt.Fprintf(os.Stderr, "Project not found: %s\n", projectName)
			fmt.Fprintf(os.Stderr, "Available projects:\n")
			for _, project := range projects {
				fmt.Fprintf(os.Stderr, "  - %s\n", project.Name)
			}
			os.Exit(1)
		}

		// Find activity by name
		activities, err := service.GetActivities()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading activities: %v\n", err)
			os.Exit(1)
		}

		var activityID string
		for _, activity := range activities {
			if activity.Name == activityName || activity.ID == activityName {
				activityID = activity.ID
				break
			}
		}
		if activityID == "" {
			fmt.Fprintf(os.Stderr, "Activity not found: %s\n", activityName)
			fmt.Fprintf(os.Stderr, "Available activities:\n")
			for _, activity := range activities {
				fmt.Fprintf(os.Stderr, "  - %s\n", activity.Name)
			}
			os.Exit(1)
		}

		// Find task by name (optional)
		var taskID *string
		if taskName != "" {
			tasks, err := service.GetTasks(projectID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading tasks: %v\n", err)
				os.Exit(1)
			}

			for _, task := range tasks {
				if task.Name == taskName || task.ID == taskName {
					taskID = &task.ID
					break
				}
			}
			if taskID == nil {
				fmt.Fprintf(os.Stderr, "Task not found: %s\n", taskName)
				fmt.Fprintf(os.Stderr, "Available tasks for project %s:\n", projectName)
				for _, task := range tasks {
					fmt.Fprintf(os.Stderr, "  - %s\n", task.Name)
				}
				os.Exit(1)
			}
		}

		// Start time entry
		description, _ := cmd.Flags().GetString("description")
		entry, err := service.StartTimeEntry(projectID, activityID, taskID, description)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting time entry: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("⏱️  Time tracking started!\n")
		fmt.Printf("📊 Project: %s\n", entry.ProjectName)
		if entry.TaskName != "" {
			fmt.Printf("📝 Task: %s\n", entry.TaskName)
		}
		fmt.Printf("⚡ Activity: %s\n", entry.ActivityName)
		if description != "" {
			fmt.Printf("📄 Description: %s\n", description)
		}
		fmt.Printf("🕐 Started: %s\n", entry.StartTime.Format("15:04:05"))
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("description", "d", "", "Description for the time entry")
}