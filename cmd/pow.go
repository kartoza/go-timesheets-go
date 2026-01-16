package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	powInterval   int
	powOutputDir  string
	powKeepFrames bool
)

// powCmd represents the pow (proof of work) command
var powCmd = &cobra.Command{
	Use:   "pow",
	Short: "Capture screenshots while timer is active (Proof of Work)",
	Long: `Captures full-screen screenshots at regular intervals while a timesheet timer is active.

When the timer stops (or you press Ctrl+C), the screenshots are automatically
compiled into a timelapse video using ffmpeg, and the individual frames are deleted.

This provides "proof of work" - a visual record of what was on screen during
the work session.

Requirements:
  - scrot (for X11) or grim (for Wayland) for screenshots
  - ffmpeg for video compilation

Examples:
  # Start capturing with default 1-minute interval
  kartoza-timesheet pow

  # Capture every 30 seconds
  kartoza-timesheet pow --interval 30

  # Keep the individual frames after creating video
  kartoza-timesheet pow --keep-frames`,
	Run: runPow,
}

func init() {
	rootCmd.AddCommand(powCmd)

	powCmd.Flags().IntVarP(&powInterval, "interval", "i", 60, "Screenshot interval in seconds")
	powCmd.Flags().StringVarP(&powOutputDir, "output", "o", "", "Output directory for screenshots and video (default: ~/.local/share/kartoza-timesheet/pow)")
	powCmd.Flags().BoolVarP(&powKeepFrames, "keep-frames", "k", false, "Keep individual frame images after creating video")
}

func runPow(cmd *cobra.Command, args []string) {
	// Check for required tools
	screenshotTool := detectScreenshotTool()
	if screenshotTool == "" {
		fmt.Fprintln(os.Stderr, "Error: No screenshot tool found. Please install 'scrot' (X11) or 'grim' (Wayland)")
		os.Exit(1)
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: ffmpeg not found. Please install ffmpeg for video compilation")
		os.Exit(1)
	}

	// Setup output directory
	outputDir := powOutputDir
	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, ".local", "share", "kartoza-timesheet", "pow")
	}

	// Get API client to check timer status
	client, err := getAPIClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check if there's an active timer
	activeTimer, err := client.GetActiveTimesheet()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking timer status: %v\n", err)
		os.Exit(1)
	}

	if activeTimer == nil {
		fmt.Fprintln(os.Stderr, "No active timer. Start a timer first, then run 'kartoza-timesheet pow'")
		os.Exit(1)
	}

	// Create session directory with timestamp
	sessionTime := time.Now().Format("2006-01-02_15-04-05")
	sessionDir := filepath.Join(outputDir, sessionTime)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting Proof of Work capture\n")
	fmt.Printf("  Project: %s\n", activeTimer.ProjectName)
	if activeTimer.TaskName != "" {
		fmt.Printf("  Task: %s\n", activeTimer.TaskName)
	}
	fmt.Printf("  Screenshot interval: %d seconds\n", powInterval)
	fmt.Printf("  Output directory: %s\n", sessionDir)
	fmt.Printf("  Screenshot tool: %s\n", screenshotTool)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop capturing and generate video...")
	fmt.Println()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start capture loop
	ticker := time.NewTicker(time.Duration(powInterval) * time.Second)
	defer ticker.Stop()

	frameCount := 0
	stopCapture := false

	// Take first screenshot immediately
	frameCount++
	if err := captureScreenshot(screenshotTool, sessionDir, frameCount); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to capture frame %d: %v\n", frameCount, err)
	} else {
		fmt.Printf("  [%s] Captured frame %d\n", time.Now().Format("15:04:05"), frameCount)
	}

	for !stopCapture {
		select {
		case <-sigChan:
			fmt.Println("\nStopping capture...")
			stopCapture = true

		case <-ticker.C:
			// Check if timer is still active
			activeTimer, err = client.GetActiveTimesheet()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error checking timer: %v\n", err)
				continue
			}

			if activeTimer == nil {
				fmt.Println("\nTimer stopped, finishing capture...")
				stopCapture = true
				continue
			}

			// Capture screenshot
			frameCount++
			if err := captureScreenshot(screenshotTool, sessionDir, frameCount); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to capture frame %d: %v\n", frameCount, err)
			} else {
				fmt.Printf("  [%s] Captured frame %d\n", time.Now().Format("15:04:05"), frameCount)
			}
		}
	}

	// Generate video if we have frames
	if frameCount > 0 {
		fmt.Printf("\nCaptured %d frames. Generating timelapse video...\n", frameCount)
		videoPath := filepath.Join(outputDir, fmt.Sprintf("pow_%s.mp4", sessionTime))

		if err := generateVideo(sessionDir, videoPath, frameCount); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating video: %v\n", err)
			fmt.Println("Frames are preserved in:", sessionDir)
			os.Exit(1)
		}

		fmt.Printf("Video created: %s\n", videoPath)

		// Clean up frames unless --keep-frames is set
		if !powKeepFrames {
			fmt.Println("Cleaning up frames...")
			if err := os.RemoveAll(sessionDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to clean up frames: %v\n", err)
			}
		} else {
			fmt.Println("Frames preserved in:", sessionDir)
		}
	} else {
		fmt.Println("No frames captured.")
		os.RemoveAll(sessionDir)
	}
}

// detectScreenshotTool finds an available screenshot tool
func detectScreenshotTool() string {
	// Check for Wayland first (grim)
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("grim"); err == nil {
			return "grim"
		}
	}

	// Check for X11 tools
	if _, err := exec.LookPath("scrot"); err == nil {
		return "scrot"
	}

	// Fallback: check for gnome-screenshot
	if _, err := exec.LookPath("gnome-screenshot"); err == nil {
		return "gnome-screenshot"
	}

	// Check for ImageMagick import
	if _, err := exec.LookPath("import"); err == nil {
		return "import"
	}

	return ""
}

// captureScreenshot captures a screenshot using the specified tool
func captureScreenshot(tool, outputDir string, frameNum int) error {
	filename := filepath.Join(outputDir, fmt.Sprintf("frame_%05d.png", frameNum))

	var cmd *exec.Cmd

	switch tool {
	case "grim":
		// Wayland screenshot
		cmd = exec.Command("grim", filename)

	case "scrot":
		// X11 screenshot with scrot
		cmd = exec.Command("scrot", filename)

	case "gnome-screenshot":
		// GNOME screenshot
		cmd = exec.Command("gnome-screenshot", "-f", filename)

	case "import":
		// ImageMagick import (captures root window)
		cmd = exec.Command("import", "-window", "root", filename)

	default:
		return fmt.Errorf("unknown screenshot tool: %s", tool)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %v (output: %s)", tool, err, string(output))
	}

	return nil
}

// generateVideo creates a timelapse video from captured frames
func generateVideo(framesDir, outputPath string, frameCount int) error {
	// Use ffmpeg to create video from frames
	// -framerate: input framerate (frames per second in output)
	// -i: input pattern for numbered frames
	// -c:v libx264: use H.264 codec
	// -pix_fmt yuv420p: pixel format for compatibility
	// -crf 23: quality (lower = better, 18-28 is reasonable range)

	inputPattern := filepath.Join(framesDir, "frame_%05d.png")

	// Calculate a reasonable framerate based on frame count
	// Aim for videos between 10-60 seconds
	framerate := frameCount / 30 // ~30 second video
	if framerate < 1 {
		framerate = 1
	}
	if framerate > 30 {
		framerate = 30
	}

	cmd := exec.Command("ffmpeg",
		"-y", // Overwrite output file
		"-framerate", fmt.Sprintf("%d", framerate),
		"-i", inputPattern,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-crf", "23",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %v (output: %s)", err, string(output))
	}

	return nil
}
