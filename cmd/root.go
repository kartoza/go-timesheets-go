package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
	"github.com/kartoza/go-timesheets-go/internal/tui"
)

var (
	dataDir   string
	userID    string
	debugMode bool
	powMode   bool
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
- Workspace automation support
- Proof of Work screenshot capture (--pow flag)`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set debug mode in TUI package (only effective in dev builds)
		tui.SetDebugMode(debugMode)

		// Configure POW mode if enabled
		var powCapturer *pow.Capturer
		if powMode {
			cfg := pow.DefaultConfig()
			cfg.Enabled = true
			var err error
			powCapturer, err = pow.New(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to initialize POW capture: %v\n", err)
			} else if powCapturer.IsEnabled() {
				fmt.Fprintln(os.Stderr, "POW mode enabled - screenshots will be captured during timer sessions")
			} else {
				fmt.Fprintln(os.Stderr, "POW mode requested but disabled - missing screenshot tool or ffmpeg")
			}
		}

		// Launch TUI application with authentication
		app, err := tui.NewAppWithAuth(powCapturer)
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
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "Enable debug mode (shows developer menu options)")
	rootCmd.PersistentFlags().BoolVarP(&powMode, "pow", "p", false, "Enable Proof of Work mode (capture screenshots during timer sessions)")
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

// getAPIClient returns an authenticated API client
func getAPIClient() (*api.Client, error) {
	// Load auth token
	token, err := config.LoadToken()
	if err != nil || token == nil {
		return nil, fmt.Errorf("not logged in: please run the TUI first to authenticate")
	}

	// Create API client
	client, err := api.NewClient(api.Config{
		BaseURL:   token.BaseURL,
		AuthToken: token.Token,
		Timeout:   30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	client.SetAuthToken(token.Token)
	client.SetUsername(token.Username)
	return client, nil
}
