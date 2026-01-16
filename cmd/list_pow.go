package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// listPowCmd represents the list-pow command
var listPowCmd = &cobra.Command{
	Use:   "list-pow",
	Short: "List all Proof of Work videos",
	Long: `Lists all POW (Proof of Work) timelapse videos that have been generated
during timer sessions.

Videos are stored in ~/.local/share/kartoza-timesheet/pow/

Examples:
  # List all POW videos
  kartoza-timesheet list-pow

  # List with full paths
  kartoza-timesheet list-pow --full-path`,
	Run: runListPow,
}

var listPowFullPath bool

func init() {
	rootCmd.AddCommand(listPowCmd)
	listPowCmd.Flags().BoolVarP(&listPowFullPath, "full-path", "f", false, "Show full file paths")
}

func runListPow(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not determine home directory: %v\n", err)
		os.Exit(1)
	}

	powDir := filepath.Join(homeDir, ".local", "share", "kartoza-timesheet", "pow")

	// Check if directory exists
	if _, err := os.Stat(powDir); os.IsNotExist(err) {
		fmt.Println("No POW videos found.")
		fmt.Printf("POW directory does not exist: %s\n", powDir)
		return
	}

	// Find all .mp4 files
	entries, err := os.ReadDir(powDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading POW directory: %v\n", err)
		os.Exit(1)
	}

	var videos []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp4") {
			videos = append(videos, entry.Name())
		}
	}

	if len(videos) == 0 {
		fmt.Println("No POW videos found.")
		fmt.Printf("POW directory: %s\n", powDir)
		return
	}

	// Sort by name (which includes timestamp, so chronological)
	sort.Strings(videos)

	fmt.Printf("Found %d POW video(s):\n\n", len(videos))

	for _, video := range videos {
		if listPowFullPath {
			fmt.Println(filepath.Join(powDir, video))
		} else {
			// Parse the filename to show more readable info
			// Format: pow_ProjectName_2006-01-02_15-04-05.mp4
			name := strings.TrimSuffix(video, ".mp4")
			name = strings.TrimPrefix(name, "pow_")

			// Find the timestamp part (last 19 chars before .mp4: 2006-01-02_15-04-05)
			if len(name) > 19 {
				timestampPart := name[len(name)-19:]
				projectPart := name[:len(name)-20] // -20 to remove the underscore before timestamp

				// Convert timestamp to more readable format
				timestamp := strings.Replace(timestampPart, "_", " ", 1)
				timestamp = strings.Replace(timestamp, "-", ":", 2)

				fmt.Printf("  %s\n", video)
				fmt.Printf("    Project: %s\n", projectPart)
				fmt.Printf("    Date:    %s\n", timestamp)
				fmt.Println()
			} else {
				fmt.Printf("  %s\n", video)
			}
		}
	}

	if !listPowFullPath {
		fmt.Printf("POW directory: %s\n", powDir)
	}
}
