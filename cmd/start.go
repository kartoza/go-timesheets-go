package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [project] [activity] [task]",
	Short: "Start time tracking for a project",
	Long: `Start time tracking for a specific project and activity.

Arguments:
  project   - Project name or ID (required)
  activity  - Activity name or ID (required)
  task      - Task name or ID (optional)

Flags:
  --json           Output result as JSON instead of human-readable format
  --description    Description for the time entry
  --no-validate    Skip validation when using IDs (faster, but assumes valid IDs)

Examples:
  kartoza-timesheet start "WB GEEST 2" "Coding"
  kartoza-timesheet start "WB GEEST 2" "Coding" "Task 3: Improved Functionalities"
  kartoza-timesheet start project-123 activity-456 task-789 --json
  kartoza-timesheet start proj-1 act-2 --description "Working on feature X" --json

When --json is used, outputs detailed JSON with the started entry.
This command is designed for automation and quick time tracking starts.
It will refuse to start if there is already an active timer running (checked via API).`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getAPIClient()
		if err != nil {
			outputError(cmd, "Error initializing API client", err)
			os.Exit(1)
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")
		noValidate, _ := cmd.Flags().GetBool("no-validate")

		// Check if there's already an active timer via API (canonical source)
		activeEntry, err := client.GetActiveTimesheet()
		if err != nil {
			outputError(cmd, "Error checking active timer", err)
			os.Exit(1)
		}

		if activeEntry != nil {
			startTime, _ := activeEntry.GetFromTimeAsTime()
			if jsonOutput {
				result := map[string]interface{}{
					"success": false,
					"error":   "Timer already running",
					"message": "Please stop the current timer before starting a new one",
					"active_entry": map[string]interface{}{
						"id":            activeEntry.ID,
						"project_name":  activeEntry.ProjectName,
						"task_name":     activeEntry.TaskName,
						"activity_name": activeEntry.ActivityType,
						"start_time":    startTime.Format(time.RFC3339),
					},
				}
				jsonData, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(jsonData))
			} else {
				fmt.Fprintf(os.Stderr, "Error: Timer already running!\n")
				fmt.Fprintf(os.Stderr, "Project: %s\n", activeEntry.ProjectName)
				if activeEntry.TaskName != "" {
					fmt.Fprintf(os.Stderr, "Task: %s\n", activeEntry.TaskName)
				}
				fmt.Fprintf(os.Stderr, "Activity: %s\n", activeEntry.ActivityType)
				fmt.Fprintf(os.Stderr, "\nPlease stop the current timer first: kartoza-timesheet stop\n")
			}
			os.Exit(1)
		}

		projectInput := args[0]
		activityInput := args[1]
		var taskInput string
		if len(args) > 2 {
			taskInput = args[2]
		}

		var projectID, activityID string
		var projectName, activityName, taskName string
		var taskID *string

		// When --no-validate is used, assume inputs are IDs
		if noValidate {
			projectID = projectInput
			activityID = activityInput
			if taskInput != "" {
				taskID = &taskInput
			}
		} else {
			// Find project by name or ID via API
			projects, err := client.GetProjects("")
			if err != nil {
				outputError(cmd, "Error loading projects", err)
				os.Exit(1)
			}

			for _, project := range projects {
				if project.Label == projectInput || strconv.Itoa(project.ID) == projectInput {
					projectID = strconv.Itoa(project.ID)
					projectName = project.Label
					break
				}
			}
			if projectID == "" {
				// Try fuzzy search
				projects, err = client.GetProjects(projectInput)
				if err == nil {
					for _, project := range projects {
						if strings.EqualFold(project.Label, projectInput) {
							projectID = strconv.Itoa(project.ID)
							projectName = project.Label
							break
						}
					}
				}
			}
			if projectID == "" {
				if jsonOutput {
					outputError(cmd, "Project not found", fmt.Errorf("%s", projectInput))
				} else {
					fmt.Fprintf(os.Stderr, "Project not found: %s\n", projectInput)
				}
				os.Exit(1)
			}

			// Find activity by name or ID via API
			activities, err := client.GetActivities()
			if err != nil {
				outputError(cmd, "Error loading activities", err)
				os.Exit(1)
			}

			for _, activity := range activities {
				if activity.Label == activityInput || strconv.Itoa(activity.ID) == activityInput {
					activityID = strconv.Itoa(activity.ID)
					activityName = activity.Label
					break
				}
			}
			if activityID == "" {
				// Try case-insensitive match
				for _, activity := range activities {
					if strings.EqualFold(activity.Label, activityInput) {
						activityID = strconv.Itoa(activity.ID)
						activityName = activity.Label
						break
					}
				}
			}
			if activityID == "" {
				if jsonOutput {
					outputError(cmd, "Activity not found", fmt.Errorf("%s", activityInput))
				} else {
					fmt.Fprintf(os.Stderr, "Activity not found: %s\n", activityInput)
					fmt.Fprintf(os.Stderr, "Available activities:\n")
					for _, activity := range activities {
						fmt.Fprintf(os.Stderr, "  - %s (ID: %d)\n", activity.Label, activity.ID)
					}
				}
				os.Exit(1)
			}

			// Find task by name or ID (optional) via API
			if taskInput != "" {
				tasks, err := client.GetTasks(projectID)
				if err != nil {
					outputError(cmd, "Error loading tasks", err)
					os.Exit(1)
				}

				for _, task := range tasks {
					if task.Label == taskInput || task.Name == taskInput || strconv.Itoa(task.ID) == taskInput {
						tid := strconv.Itoa(task.ID)
						taskID = &tid
						taskName = task.Label
						break
					}
				}
				if taskID == nil {
					if jsonOutput {
						outputError(cmd, "Task not found", fmt.Errorf("%s", taskInput))
					} else {
						fmt.Fprintf(os.Stderr, "Task not found: %s\n", taskInput)
					}
					os.Exit(1)
				}
			}
		}

		// Create time entry via API (canonical source)
		description, _ := cmd.Flags().GetString("description")
		entry := models.TimeEntry{
			ProjectID:   projectID,
			ActivityID:  activityID,
			Description: description,
			StartTime:   time.Now().UTC(),
		}
		if taskID != nil {
			entry.TaskID = taskID
		}

		err = client.CreateTimesheet(entry)
		if err != nil {
			outputError(cmd, "Error starting time entry", err)
			os.Exit(1)
		}

		// Output result
		if jsonOutput {
			result := map[string]interface{}{
				"success": true,
				"action":  "started",
				"entry": map[string]interface{}{
					"project": map[string]string{
						"id":   projectID,
						"name": projectName,
					},
					"activity": map[string]string{
						"id":   activityID,
						"name": activityName,
					},
					"description": description,
					"start_time":  entry.StartTime.Format(time.RFC3339),
				},
			}

			if taskID != nil {
				result["entry"].(map[string]interface{})["task"] = map[string]string{
					"id":   *taskID,
					"name": taskName,
				}
			}

			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Time tracking started!\n")
			if projectName != "" {
				fmt.Printf("Project: %s\n", projectName)
			}
			if taskName != "" {
				fmt.Printf("Task: %s\n", taskName)
			}
			if activityName != "" {
				fmt.Printf("Activity: %s\n", activityName)
			}
			if description != "" {
				fmt.Printf("Description: %s\n", description)
			}
			fmt.Printf("Started: %s\n", entry.StartTime.Local().Format("15:04:05"))
		}
	},
}

func outputError(cmd *cobra.Command, message string, err error) {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	if jsonOutput {
		result := map[string]interface{}{
			"success": false,
			"error":   message,
			"details": err.Error(),
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("description", "d", "", "Description for the time entry")
	startCmd.Flags().Bool("json", false, "Output result as JSON")
	startCmd.Flags().Bool("no-validate", false, "Skip validation when using IDs (faster)")
}
