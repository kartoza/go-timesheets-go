package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// playPowCmd represents the play-pow command
var playPowCmd = &cobra.Command{
	Use:   "play-pow [video-file]",
	Short: "Play a Proof of Work video",
	Long: `Select and play a POW (Proof of Work) timelapse video using your
system's default media player.

If no video file is specified, an interactive selector will be shown.

Examples:
  # Interactive selection
  kartoza-timesheet play-pow

  # Play specific video
  kartoza-timesheet play-pow pow_MyProject_2024-01-15_09-30-00.mp4`,
	Run: runPlayPow,
}

func init() {
	rootCmd.AddCommand(playPowCmd)
}

func runPlayPow(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not determine home directory: %v\n", err)
		os.Exit(1)
	}

	powDir := filepath.Join(homeDir, ".local", "share", "kartoza-timesheet", "pow")

	var videoPath string

	if len(args) > 0 {
		// Video file specified as argument
		videoPath = args[0]
		// If not an absolute path, assume it's in the POW directory
		if !filepath.IsAbs(videoPath) {
			videoPath = filepath.Join(powDir, videoPath)
		}
	} else {
		// Interactive selection
		videoPath, err = selectPowVideo(powDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if videoPath == "" {
			// User cancelled
			return
		}
	}

	// Verify file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Video file not found: %s\n", videoPath)
		os.Exit(1)
	}

	// Play the video
	if err := playVideo(videoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error playing video: %v\n", err)
		os.Exit(1)
	}
}

func selectPowVideo(powDir string) (string, error) {
	// Check if directory exists
	if _, err := os.Stat(powDir); os.IsNotExist(err) {
		return "", fmt.Errorf("no POW videos found (directory does not exist: %s)", powDir)
	}

	// Find all .mp4 files
	entries, err := os.ReadDir(powDir)
	if err != nil {
		return "", fmt.Errorf("failed to read POW directory: %w", err)
	}

	var videos []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp4") {
			videos = append(videos, entry.Name())
		}
	}

	if len(videos) == 0 {
		return "", fmt.Errorf("no POW videos found in %s", powDir)
	}

	// Sort by name descending (newest first, since names include timestamps)
	sort.Sort(sort.Reverse(sort.StringSlice(videos)))

	// Build options for the selector
	options := make([]huh.Option[string], len(videos))
	for i, video := range videos {
		// Parse the filename to show more readable info
		label := formatVideoLabel(video)
		options[i] = huh.NewOption(label, video)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a POW video to play").
				Description("Use arrow keys to navigate, Enter to select, Esc to cancel").
				Options(options...).
				Value(&selected),
		),
	)

	err = form.Run()
	if err != nil {
		if err.Error() == "user aborted" {
			return "", nil
		}
		return "", err
	}

	if selected == "" {
		return "", nil
	}

	return filepath.Join(powDir, selected), nil
}

func formatVideoLabel(filename string) string {
	// Format: pow_ProjectName_2006-01-02_15-04-05.mp4
	name := strings.TrimSuffix(filename, ".mp4")
	name = strings.TrimPrefix(name, "pow_")

	// Find the timestamp part (last 19 chars: 2006-01-02_15-04-05)
	if len(name) > 19 {
		timestampPart := name[len(name)-19:]
		projectPart := name[:len(name)-20] // -20 to remove the underscore before timestamp

		// Convert timestamp to more readable format
		// 2006-01-02_15-04-05 -> 2006-01-02 15:04:05
		date := timestampPart[:10]
		timePart := strings.ReplaceAll(timestampPart[11:], "-", ":")

		return fmt.Sprintf("%s (%s %s)", projectPart, date, timePart)
	}

	return filename
}

func playVideo(videoPath string) error {
	// Try different video players in order of preference
	players := []struct {
		name string
		args []string
	}{
		{"xdg-open", []string{videoPath}},  // Linux default
		{"mpv", []string{videoPath}},       // MPV player
		{"vlc", []string{videoPath}},       // VLC
		{"totem", []string{videoPath}},     // GNOME Videos
		{"celluloid", []string{videoPath}}, // Celluloid (MPV frontend)
		{"open", []string{videoPath}},      // macOS default
		{"start", []string{"", videoPath}}, // Windows default
	}

	for _, player := range players {
		path, err := exec.LookPath(player.name)
		if err != nil {
			continue
		}

		fmt.Printf("Playing %s with %s...\n", filepath.Base(videoPath), player.name)

		cmd := exec.Command(path, player.args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// For xdg-open and open, we don't wait for the process
		if player.name == "xdg-open" || player.name == "open" || player.name == "start" {
			return cmd.Start()
		}

		// For dedicated players, wait for them to finish
		return cmd.Run()
	}

	return fmt.Errorf("no video player found. Please install mpv, vlc, or another video player")
}
