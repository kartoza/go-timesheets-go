package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	follow     bool
	logFile    string
	since      string
	filterPath string
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor API requests and application metrics",
	Long: `Monitor API requests, view logs, and track application metrics.

This command provides several ways to monitor the timesheet application:

1. View API request logs
2. Follow logs in real-time (tail -f style)
3. Filter logs by endpoint path
4. View metrics via expvar endpoint

The application exposes metrics on http://localhost:6060/debug/vars
You can also use expvarmon for real-time monitoring:

	expvarmon -ports="localhost:6060" \
	  -vars="api.requests.total,api.requests.errors,api.requests.inflight,api.cache_hit_ratio" \
	  -i 1s`,
	Run: func(cmd *cobra.Command, args []string) {
		if logFile == "" {
			// Default log file location
			homeDir, _ := os.UserHomeDir()
			logDir := filepath.Join(homeDir, ".config/.kartoza-timesheets/logs")
			logFile = filepath.Join(logDir, fmt.Sprintf("api-requests-%s.log", time.Now().Format("2006-01-02")))
		}

		// Check if log file exists
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Log file not found: %s\n", logFile)
			fmt.Fprintf(os.Stderr, "Make sure the timesheet app is running.\n")
			os.Exit(1)
		}

		// Parse since duration
		var sinceTime time.Time
		if since != "" {
			duration, err := time.ParseDuration(since)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid duration format: %v\n", err)
				os.Exit(1)
			}
			sinceTime = time.Now().Add(-duration)
		}

		if follow {
			// Follow mode - tail -f style
			followLogs(logFile, filterPath, sinceTime)
		} else {
			// One-time display
			displayLogs(logFile, filterPath, sinceTime)
		}
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (like tail -f)")
	monitorCmd.Flags().StringVarP(&logFile, "file", "l", "", "Log file to read (default: today's log)")
	monitorCmd.Flags().StringVarP(&since, "since", "s", "", "Show logs since duration (e.g., 5m, 1h, 2h30m)")
	monitorCmd.Flags().StringVarP(&filterPath, "path", "p", "", "Filter logs by API endpoint path")
}

func displayLogs(filename string, pathFilter string, sinceTime time.Time) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	displayCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		if shouldDisplayLine(line, pathFilter, sinceTime) {
			fmt.Println(line)
			displayCount++
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "\nDisplayed %d of %d log entries\n", displayCount, lineCount)
}

func followLogs(filename string, pathFilter string, sinceTime time.Time) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	// Seek to end of file
	file.Seek(0, os.SEEK_END)

	scanner := bufio.NewScanner(file)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			if shouldDisplayLine(line, pathFilter, sinceTime) {
				fmt.Println(line)
			}
		}

		// Wait a bit before checking for new lines
		time.Sleep(100 * time.Millisecond)
	}
}

func shouldDisplayLine(line string, pathFilter string, sinceTime time.Time) bool {
	// Apply path filter
	if pathFilter != "" && !strings.Contains(line, pathFilter) {
		return false
	}

	// Apply time filter
	if !sinceTime.IsZero() {
		// Try to parse timestamp from log line
		// Format: [2026-01-13 10:30:45]
		if len(line) > 21 {
			timeStr := line[1:20] // Extract timestamp
			logTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
			if err == nil && logTime.Before(sinceTime) {
				return false
			}
		}
	}

	return true
}
