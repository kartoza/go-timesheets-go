package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command for waybar integration
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get current timesheet status for waybar integration",
	Long: `Returns JSON formatted status information suitable for waybar custom modules.

This command reads exclusively from the local timelog cache to avoid unnecessary
API calls. The cache is kept up to date by state-changing operations (start, stop,
edit) which refresh it as a side-effect. This ensures waybar polling (every few
seconds) never generates API traffic.

The status includes:
- Current tracking state (idle/recording)
- Active project, task, and activity information
- Associated repository (if configured)
- Associated workspace/desktop (if configured)
- Timer start time
- Hours logged against current task
- Today's total hours

Example waybar configuration:
{
    "custom/timesheet": {
        "exec": "kartoza-timesheet status",
        "return-type": "json",
        "interval": 5,
        "on-click": "kitty kartoza-timesheet"
    }
}`,
	Run: func(cmd *cobra.Command, args []string) {
		runStatusCommand()
	},
}

// WaybarStatus represents the status information for waybar
type WaybarStatus struct {
	Text    string `json:"text"`
	Alt     string `json:"alt"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

// StatusInfo holds all the information we gather from local cache
type StatusInfo struct {
	IsRecording     bool
	ProjectID       int
	ProjectName     string
	TaskID          int
	TaskName        string
	ActivityName    string
	StartTime       time.Time
	CurrentDuration time.Duration
	TaskHours       float64 // Total hours logged against this task
	DailyHours      float64 // Total hours logged today
	RepoName        string  // Associated repository (if any)
	WorkspaceName   string  // Associated workspace/desktop name (if any)
	WorkspaceNumber int     // Associated workspace number
}

func runStatusCommand() {
	// Load config to get correct storage directory
	cfg, err := config.LoadConfig()
	if err != nil {
		printErrorStatus(fmt.Sprintf("Config error: %v", err))
		return
	}

	// Initialize storage using config's storage directory
	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		printErrorStatus(fmt.Sprintf("Storage error: %v", err))
		return
	}

	// Read exclusively from local cache. The cache is updated by state-changing
	// operations (start, stop, edit) so it stays current without polling the API.
	var entries []api.TimelogEntry

	cache, _ := store.LoadTimelogCache()
	if cache != nil {
		entries = cache.Entries
	}

	info := StatusInfo{}

	// Find active entry and calculate totals
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, entry := range entries {
		// Check if this is an active entry (to_time is empty or matches from_time)
		if isActiveEntry(&entry) {
			info.IsRecording = true
			info.ProjectID = entry.ProjectID
			info.ProjectName = entry.ProjectName
			info.TaskID = entry.TaskID.Value
			info.TaskName = entry.TaskName
			info.ActivityName = entry.ActivityType

			startTime, err := entry.GetFromTimeAsTime()
			if err == nil {
				info.StartTime = startTime
				info.CurrentDuration = time.Since(startTime)
			}
		}

		// Calculate task hours (entries with same task ID)
		if info.TaskID > 0 && entry.TaskID.Value == info.TaskID {
			info.TaskHours += entry.Duration()
		}

		// Calculate today's total hours
		entryStart, err := entry.GetFromTimeAsTime()
		if err == nil {
			if entryStart.After(todayStart) || entryStart.Equal(todayStart) {
				info.DailyHours += entry.Duration()
			}
		}
	}

	// If we found an active entry, add the current session duration to totals
	if info.IsRecording && info.CurrentDuration > 0 {
		currentHours := info.CurrentDuration.Hours()
		info.TaskHours += currentHours
		// Only add to daily if the entry started today
		if info.StartTime.After(todayStart) || info.StartTime.Equal(todayStart) {
			info.DailyHours += currentHours
		}
	}

	// Load code repo associations to find associated repository
	if info.ProjectID > 0 {
		repoAssocs, err := store.LoadCodeRepoAssociations()
		if err == nil && repoAssocs != nil {
			if assoc := repoAssocs.GetAssociationByProject(info.ProjectID); assoc != nil {
				info.RepoName = assoc.GetDisplayString()
			}
		}
	}

	// Load workspace associations to find associated desktop
	wsAssocs, err := store.LoadWorkspaceAssociations()
	if err == nil && wsAssocs != nil {
		info.WorkspaceName, info.WorkspaceNumber = findMatchingWorkspace(wsAssocs, info.ProjectID, info.TaskID)
	}

	status := createWaybarStatusFromInfo(info)

	jsonData, err := json.Marshal(status)
	if err != nil {
		printErrorStatus(fmt.Sprintf("JSON error: %v", err))
		return
	}

	fmt.Println(string(jsonData))
}

// isActiveEntry checks if a timelog entry is currently active
func isActiveEntry(entry *api.TimelogEntry) bool {
	// An entry is active if to_time is empty or equals from_time
	if entry.ToTime == "" {
		return true
	}
	// Also check if from and to times are the same (sometimes indicates running)
	return entry.ToTime == entry.FromTime
}

// findMatchingWorkspace finds a workspace that matches the current project/task
func findMatchingWorkspace(assocs *models.WorkspaceAssociations, projectID, taskID int) (string, int) {
	if assocs == nil {
		return "", 0
	}

	for _, assoc := range assocs.Associations {
		if assoc.ProjectID == projectID {
			// If we have a task, try to match that too
			if taskID > 0 && assoc.TaskID > 0 {
				if assoc.TaskID == taskID {
					return assoc.WorkspaceName, assoc.WorkspaceNumber
				}
			} else if assoc.TaskID == 0 {
				// Project-only match
				return assoc.WorkspaceName, assoc.WorkspaceNumber
			}
		}
	}

	return "", 0
}

// Nerd Font icons for waybar display
const (
	nfTimer     = "󰔛" // nf-md-timer - for recording state
	nfPause     = "󰏤" // nf-md-pause - for idle state
	nfError     = "󰅜" // nf-md-close-circle - for error state
	nfFolder    = "󰉋" // nf-md-folder - for project
	nfTask      = "󰄬" // nf-md-checkbox-marked - for task
	nfActivity  = "󰒓" // nf-md-cog - for activity
	nfRepo      = "󰊢" // nf-md-git - for repository
	nfDesktop   = "󰍹" // nf-md-monitor - for desktop
	nfClock     = "󰥔" // nf-md-clock - for start time
	nfChart     = "󰄧" // nf-md-chart-bar - for task total
	nfCalendar  = "󰃭" // nf-md-calendar - for today's total
)

func createWaybarStatusFromInfo(info StatusInfo) WaybarStatus {
	if info.IsRecording {
		// Recording state
		elapsed := formatDuration(info.CurrentDuration)

		// Build detailed tooltip with all information
		var tooltipLines []string
		tooltipLines = append(tooltipLines, nfTimer+" Recording time")
		tooltipLines = append(tooltipLines, "")

		// Project information
		if info.ProjectName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Project: %s", nfFolder, info.ProjectName))
		}

		// Task information
		if info.TaskName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Task: %s", nfTask, info.TaskName))
		}

		// Activity information
		if info.ActivityName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Activity: %s", nfActivity, info.ActivityName))
		}

		// Repository information (optional)
		if info.RepoName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Repo: %s", nfRepo, info.RepoName))
		}

		// Workspace/desktop information (optional)
		if info.WorkspaceName != "" {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Desktop: %s (#%d)", nfDesktop, info.WorkspaceName, info.WorkspaceNumber))
		} else if info.WorkspaceNumber > 0 {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Desktop: #%d", nfDesktop, info.WorkspaceNumber))
		}

		tooltipLines = append(tooltipLines, "")

		// Timer start time
		tooltipLines = append(tooltipLines, fmt.Sprintf("%s Started: %s", nfClock, info.StartTime.Format("15:04")))

		// Current session duration
		tooltipLines = append(tooltipLines, fmt.Sprintf("%s Session: %s", nfTimer, elapsed))

		// Task hours
		if info.TaskHours > 0 {
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s Task total: %.2fh", nfChart, info.TaskHours))
		}

		// Today's total
		tooltipLines = append(tooltipLines, fmt.Sprintf("%s Today: %.2fh", nfCalendar, info.DailyHours))

		tooltip := strings.Join(tooltipLines, "\n")

		return WaybarStatus{
			Text:    fmt.Sprintf("%s %s", nfTimer, elapsed),
			Alt:     "recording",
			Tooltip: tooltip,
			Class:   "recording",
		}
	}

	// Idle state
	var tooltipLines []string
	tooltipLines = append(tooltipLines, nfPause+" Timesheet idle")
	tooltipLines = append(tooltipLines, "")
	tooltipLines = append(tooltipLines, fmt.Sprintf("%s Today's total: %.2fh", nfCalendar, info.DailyHours))
	tooltipLines = append(tooltipLines, "")
	tooltipLines = append(tooltipLines, "Click to start tracking")

	return WaybarStatus{
		Text:    fmt.Sprintf("%s %.1fh", nfPause, info.DailyHours),
		Alt:     "idle",
		Tooltip: strings.Join(tooltipLines, "\n"),
		Class:   "idle",
	}
}

func printErrorStatus(message string) {
	status := WaybarStatus{
		Text:    nfError,
		Alt:     "error",
		Tooltip: fmt.Sprintf("Error: %s\n\nClick to open timesheet app", message),
		Class:   "error",
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
