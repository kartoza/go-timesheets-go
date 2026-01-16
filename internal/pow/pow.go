// Package pow provides Proof of Work screenshot capture functionality
package pow

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// powDebugLog is used to log POW activity
var powDebugLog *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/kartoza-timesheet-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		powDebugLog = log.New(io.Discard, "", 0)
	} else {
		powDebugLog = log.New(f, "POW: ", log.LstdFlags|log.Lshortfile)
	}
}

// Capturer handles screenshot capture and video generation for proof of work
type Capturer struct {
	mu             sync.Mutex
	enabled        bool
	interval       time.Duration
	outputDir      string
	screenshotTool string

	// Current session state
	sessionActive bool
	sessionDir    string
	sessionStart  time.Time
	frameCount    int
	stopChan      chan struct{}
	wg            sync.WaitGroup

	// Metadata for video naming
	projectName string
	taskName    string
	timesheetID int
}

// Config holds configuration for the POW capturer
type Config struct {
	Enabled   bool
	Interval  time.Duration // Screenshot interval (default: 60s)
	OutputDir string        // Base output directory
}

// DefaultConfig returns the default POW configuration
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		Enabled:   false,
		Interval:  60 * time.Second,
		OutputDir: filepath.Join(homeDir, ".local", "share", "kartoza-timesheet", "pow"),
	}
}

// New creates a new POW capturer
func New(cfg Config) (*Capturer, error) {
	if !cfg.Enabled {
		return &Capturer{enabled: false}, nil
	}

	// Detect screenshot tool
	tool := detectScreenshotTool()
	if tool == "" {
		powDebugLog.Printf("No screenshot tool found, POW disabled")
		return &Capturer{enabled: false}, nil
	}

	// Check for ffmpeg
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		powDebugLog.Printf("ffmpeg not found, POW disabled")
		return &Capturer{enabled: false}, nil
	}

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create POW output directory: %w", err)
	}

	powDebugLog.Printf("POW capturer initialized: tool=%s, interval=%v, outputDir=%s",
		tool, cfg.Interval, cfg.OutputDir)

	return &Capturer{
		enabled:        true,
		interval:       cfg.Interval,
		outputDir:      cfg.OutputDir,
		screenshotTool: tool,
	}, nil
}

// IsEnabled returns whether POW capture is enabled
func (c *Capturer) IsEnabled() bool {
	return c.enabled
}

// SetEnabled enables or disables POW capture
// Returns true if the state was changed, false if requirements are not met
func (c *Capturer) SetEnabled(enabled bool) bool {
	if enabled {
		// Check requirements before enabling
		tool := detectScreenshotTool()
		if tool == "" {
			powDebugLog.Printf("Cannot enable POW: no screenshot tool found")
			return false
		}
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			powDebugLog.Printf("Cannot enable POW: ffmpeg not found")
			return false
		}

		// Create output directory if needed
		cfg := DefaultConfig()
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			powDebugLog.Printf("Cannot enable POW: failed to create output dir: %v", err)
			return false
		}

		c.mu.Lock()
		c.enabled = true
		c.screenshotTool = tool
		c.outputDir = cfg.OutputDir
		c.interval = cfg.Interval
		c.mu.Unlock()

		powDebugLog.Printf("POW enabled: tool=%s", tool)
		return true
	}

	c.mu.Lock()
	c.enabled = false
	c.mu.Unlock()
	powDebugLog.Printf("POW disabled")
	return true
}

// Toggle toggles the POW enabled state
// Returns the new enabled state and whether the operation succeeded
func (c *Capturer) Toggle() (enabled bool, ok bool) {
	if c.enabled {
		c.SetEnabled(false)
		return false, true
	}
	ok = c.SetEnabled(true)
	return c.enabled, ok
}

// StartSession begins capturing screenshots for a new timesheet session
func (c *Capturer) StartSession(timesheetID int, projectName, taskName string) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionActive {
		powDebugLog.Printf("Session already active, ignoring start request")
		return nil
	}

	// Create session directory
	c.sessionStart = time.Now()
	sessionTime := c.sessionStart.Format("2006-01-02_15-04-05")
	c.sessionDir = filepath.Join(c.outputDir, sessionTime)

	if err := os.MkdirAll(c.sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	c.projectName = projectName
	c.taskName = taskName
	c.timesheetID = timesheetID
	c.frameCount = 0
	c.sessionActive = true
	c.stopChan = make(chan struct{})

	powDebugLog.Printf("Starting POW session: id=%d, project=%s, task=%s, dir=%s",
		timesheetID, projectName, taskName, c.sessionDir)

	// Start capture goroutine
	c.wg.Add(1)
	go c.captureLoop()

	return nil
}

// StopSession stops the current capture session and generates the video
func (c *Capturer) StopSession() (string, error) {
	if !c.enabled {
		return "", nil
	}

	c.mu.Lock()
	if !c.sessionActive {
		c.mu.Unlock()
		return "", nil
	}

	// Signal stop
	close(c.stopChan)
	sessionDir := c.sessionDir
	frameCount := c.frameCount
	projectName := c.projectName
	sessionStart := c.sessionStart

	c.sessionActive = false
	c.mu.Unlock()

	// Wait for capture loop to finish
	c.wg.Wait()

	powDebugLog.Printf("POW session stopped: frames=%d", frameCount)

	if frameCount == 0 {
		// No frames captured, clean up
		os.RemoveAll(sessionDir)
		return "", nil
	}

	// Generate video
	videoName := fmt.Sprintf("pow_%s_%s.mp4",
		sanitizeFilename(projectName),
		sessionStart.Format("2006-01-02_15-04-05"))
	videoPath := filepath.Join(c.outputDir, videoName)

	powDebugLog.Printf("Generating video: %s (from %d frames)", videoPath, frameCount)

	if err := generateVideo(sessionDir, videoPath, frameCount); err != nil {
		powDebugLog.Printf("Failed to generate video: %v", err)
		return "", fmt.Errorf("failed to generate video: %w", err)
	}

	// Clean up frames
	powDebugLog.Printf("Cleaning up frames in %s", sessionDir)
	os.RemoveAll(sessionDir)

	powDebugLog.Printf("Video created: %s", videoPath)
	return videoPath, nil
}

// captureLoop runs the screenshot capture loop
func (c *Capturer) captureLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Capture first frame immediately
	c.captureFrame()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.captureFrame()
		}
	}
}

// captureFrame captures a single screenshot
func (c *Capturer) captureFrame() {
	c.mu.Lock()
	c.frameCount++
	frameNum := c.frameCount
	sessionDir := c.sessionDir
	tool := c.screenshotTool
	c.mu.Unlock()

	filename := filepath.Join(sessionDir, fmt.Sprintf("frame_%05d.png", frameNum))

	var cmd *exec.Cmd
	switch tool {
	case "grim":
		cmd = exec.Command("grim", filename)
	case "scrot":
		cmd = exec.Command("scrot", filename)
	case "gnome-screenshot":
		cmd = exec.Command("gnome-screenshot", "-f", filename)
	case "import":
		cmd = exec.Command("import", "-window", "root", filename)
	case "screencapture":
		// macOS built-in screenshot tool
		// -x: no sound, -C: capture cursor
		cmd = exec.Command("screencapture", "-x", "-C", filename)
	case "powershell":
		// Windows PowerShell screenshot using .NET
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bitmap = New-Object System.Drawing.Bitmap($screen.Width, $screen.Height)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
$bitmap.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()
`, filename)
		cmd = exec.Command("powershell", "-NoProfile", "-Command", psScript)
	default:
		powDebugLog.Printf("Unknown screenshot tool: %s", tool)
		return
	}

	if err := cmd.Run(); err != nil {
		powDebugLog.Printf("Failed to capture frame %d: %v", frameNum, err)
	} else {
		powDebugLog.Printf("Captured frame %d", frameNum)
	}
}

// detectScreenshotTool finds an available screenshot tool
func detectScreenshotTool() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS - use screencapture (built-in)
		if _, err := exec.LookPath("screencapture"); err == nil {
			return "screencapture"
		}

	case "windows":
		// Windows - use PowerShell with .NET (always available)
		if _, err := exec.LookPath("powershell"); err == nil {
			return "powershell"
		}

	default:
		// Linux - check for various tools
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
	}

	return ""
}

// generateVideo creates a timelapse video from captured frames
func generateVideo(framesDir, outputPath string, frameCount int) error {
	inputPattern := filepath.Join(framesDir, "frame_%05d.png")

	// Calculate framerate - aim for ~30 second videos
	framerate := frameCount / 30
	if framerate < 1 {
		framerate = 1
	}
	if framerate > 30 {
		framerate = 30
	}

	powDebugLog.Printf("Running ffmpeg: framerate=%d, input=%s, output=%s",
		framerate, inputPattern, outputPath)

	cmd := exec.Command("ffmpeg",
		"-y",
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

// sanitizeFilename removes or replaces characters not safe for filenames
func sanitizeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "session"
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}

// CheckRequirements checks if POW capture requirements are met
func CheckRequirements() (screenshotTool string, hasFfmpeg bool) {
	screenshotTool = detectScreenshotTool()
	_, err := exec.LookPath("ffmpeg")
	hasFfmpeg = err == nil
	return
}
