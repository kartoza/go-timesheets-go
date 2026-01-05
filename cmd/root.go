package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
	"github.com/kartoza/go-timesheets-go/internal/tui"
)

var (
	dataDir string
	userID  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kartoza-timesheet",
	Short: "A beautiful TUI timesheet application for Kartoza",
	Long: `Kartoza Timesheet App - A terminal-based timesheet application
built with Go and Bubbletea

Features:
- Time tracking with projects, tasks, and activities
- Beautiful terminal user interface with responsive layout
- Weekly and daily time reports with charts
- Timesheet submission workflow
- Integration with waybar for desktop notifications
- Workspace automation support`,
	Run: func(cmd *cobra.Command, args []string) {
		// Launch TUI application with authentication
		app, err := tui.NewAppWithAuth()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing application: %v\n", err)
			os.Exit(1)
		}

		if err := app.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(homeDir, ".config/.kartoza-timesheets")
	
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", defaultDataDir, "Data directory for storing timesheet data")
	rootCmd.PersistentFlags().StringVar(&userID, "user", "default-user", "User ID for timesheet entries")
}

func initConfig() {
	// Initialize data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data directory: %v\n", err)
		os.Exit(1)
	}
}

// getService returns a configured timesheet service
func getService() (*service.TimesheetService, error) {
	storage, err := storage.New(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return service.New(storage, userID), nil
}
