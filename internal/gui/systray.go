package gui

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"math"
	"time"

	"fyne.io/systray"
)

//go:embed icon.png
var iconData []byte

// TimerInfo contains details about the running timer for tooltip display
type TimerInfo struct {
	ProjectName  string
	TaskName     string
	ActivityName string
	StartTime    time.Time
}

// SystrayManager handles the system tray icon and menu
type SystrayManager struct {
	app *App

	// Menu items
	mShow   *systray.MenuItem
	mStatus *systray.MenuItem
	mQuit   *systray.MenuItem

	// Channels for communication
	showChan chan struct{}
	quitChan chan struct{}

	// Icon rotation
	rotationTicker *time.Ticker
	stopRotation   chan struct{}
	isRotating     bool
	rotationAngle  float64
	baseIcon       image.Image
	rotatedIcons   [][]byte // Pre-rendered rotated icons

	// Timer info for tooltip
	timerInfo     *TimerInfo
	tooltipTicker *time.Ticker
	stopTooltip   chan struct{}
}

// NewSystrayManager creates a new system tray manager
func NewSystrayManager(app *App) *SystrayManager {
	s := &SystrayManager{
		app:          app,
		showChan:     make(chan struct{}, 1),
		quitChan:     make(chan struct{}, 1),
		stopRotation: make(chan struct{}),
		stopTooltip:  make(chan struct{}),
	}
	s.loadAndPrepareIcons()
	return s
}

// loadAndPrepareIcons loads the base icon and pre-renders rotated versions
func (s *SystrayManager) loadAndPrepareIcons() {
	// Decode the base icon
	img, err := png.Decode(bytes.NewReader(iconData))
	if err != nil {
		return
	}
	s.baseIcon = img

	// Pre-render 12 rotated versions (30 degrees each = 360/12)
	s.rotatedIcons = make([][]byte, 12)
	for i := 0; i < 12; i++ {
		angle := float64(i) * 30.0 * math.Pi / 180.0
		rotated := rotateImage(img, angle)
		var buf bytes.Buffer
		if err := png.Encode(&buf, rotated); err == nil {
			s.rotatedIcons[i] = buf.Bytes()
		}
	}
}

// rotateImage rotates an image by the given angle (in radians)
func rotateImage(src image.Image, angle float64) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Create a new RGBA image
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	// Center of rotation
	cx, cy := float64(w)/2, float64(h)/2

	// Precompute sin and cos
	sin, cos := math.Sin(-angle), math.Cos(-angle)

	// For each pixel in the destination
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Translate to center
			dx := float64(x) - cx
			dy := float64(y) - cy

			// Rotate (inverse rotation to find source pixel)
			srcX := dx*cos - dy*sin + cx
			srcY := dx*sin + dy*cos + cy

			// Check bounds and copy pixel
			sx, sy := int(srcX), int(srcY)
			if sx >= 0 && sx < w && sy >= 0 && sy < h {
				dst.Set(x, y, src.At(sx, sy))
			}
		}
	}

	return dst
}

// ShowChan returns the channel that signals when to show the window
func (s *SystrayManager) ShowChan() <-chan struct{} {
	return s.showChan
}

// QuitChan returns the channel that signals when to quit
func (s *SystrayManager) QuitChan() <-chan struct{} {
	return s.quitChan
}

// OnReady is called when the systray is ready
func (s *SystrayManager) OnReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Kartoza Timesheets")
	systray.SetTooltip("Kartoza Timesheets - Time tracking made simple")

	// Set up left-click handler to show window
	systray.SetOnTapped(func() {
		select {
		case s.showChan <- struct{}{}:
		default:
		}
	})

	// Add menu items (shown on right-click)
	s.mShow = systray.AddMenuItem("Show Window", "Show the main window")
	systray.AddSeparator()
	s.mStatus = systray.AddMenuItem("No active timer", "Current timer status")
	s.mStatus.Disable()
	systray.AddSeparator()
	s.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks in a goroutine
	go s.handleClicks()
}

// OnExit is called when the systray is exiting
func (s *SystrayManager) OnExit() {
	s.StopIconRotation()
}

// handleClicks handles menu item clicks
func (s *SystrayManager) handleClicks() {
	for {
		select {
		case <-s.mShow.ClickedCh:
			select {
			case s.showChan <- struct{}{}:
			default:
			}
		case <-s.mQuit.ClickedCh:
			select {
			case s.quitChan <- struct{}{}:
			default:
			}
			return
		}
	}
}

// UpdateStatus updates the status shown in the tray menu
func (s *SystrayManager) UpdateStatus(status string) {
	if s.mStatus != nil {
		s.mStatus.SetTitle(status)
	}
}

// StartIconRotation starts the icon rotation animation.
//
// The goroutine captures the ticker channel, stop channel, and icon slice
// into locals before launching so that StopIconRotation can safely nil out
// the struct fields without racing this goroutine — previously the goroutine
// read s.rotationTicker.C through the struct field on every select
// iteration, which crashed with a nil-pointer panic if Stop ran between two
// iterations (pause/resume cycles made this trigger reliably).
func (s *SystrayManager) StartIconRotation() {
	if s.isRotating || len(s.rotatedIcons) == 0 {
		return
	}

	s.isRotating = true
	s.rotationTicker = time.NewTicker(250 * time.Millisecond) // Rotate every 250ms

	// A fresh stop channel per Start, closed by Stop. Closing (vs sending)
	// guarantees the goroutine receives the signal even if it's mid-iteration
	// when Stop fires — the previous non-blocking send could be dropped.
	s.stopRotation = make(chan struct{})

	tickerC := s.rotationTicker.C
	stopCh := s.stopRotation
	icons := s.rotatedIcons

	go func() {
		iconIndex := 0
		for {
			select {
			case <-tickerC:
				// Set the next rotated icon
				if iconIndex < len(icons) && icons[iconIndex] != nil {
					systray.SetIcon(icons[iconIndex])
				}
				iconIndex = (iconIndex + 1) % len(icons)
			case <-stopCh:
				return
			}
		}
	}()
}

// StopIconRotation stops the icon rotation and resets to base icon.
// Safe to call repeatedly; the close happens at most once because we guard
// on isRotating.
func (s *SystrayManager) StopIconRotation() {
	if !s.isRotating {
		return
	}

	s.isRotating = false
	if s.rotationTicker != nil {
		s.rotationTicker.Stop()
		s.rotationTicker = nil
	}

	// Close (not send) so the running goroutine always exits, even if it
	// was busy in systray.SetIcon when this fired.
	if s.stopRotation != nil {
		close(s.stopRotation)
		s.stopRotation = nil
	}

	// Reset to base icon
	systray.SetIcon(iconData)
}

// SetTimerRunning updates the tray to indicate a timer is running
func (s *SystrayManager) SetTimerRunning(projectName, taskName, activityName string, startTime time.Time) {
	// Store timer info
	s.timerInfo = &TimerInfo{
		ProjectName:  projectName,
		TaskName:     taskName,
		ActivityName: activityName,
		StartTime:    startTime,
	}

	// Update menu status
	if taskName != "" {
		s.UpdateStatus("⏱ " + projectName + " - " + taskName)
	} else {
		s.UpdateStatus("⏱ " + projectName)
	}

	// Update tooltip immediately
	s.updateTooltip()

	// Start tooltip update ticker (every 30 seconds)
	s.startTooltipUpdater()

	// Start icon rotation
	s.StartIconRotation()
}

// updateTooltip updates the systray tooltip with current timer info
func (s *SystrayManager) updateTooltip() {
	if s.timerInfo == nil {
		systray.SetTooltip("Kartoza Timesheets - Time tracking made simple")
		return
	}

	elapsed := time.Since(s.timerInfo.StartTime)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60

	// Build tooltip lines
	tooltip := fmt.Sprintf("Project: %s", s.timerInfo.ProjectName)
	if s.timerInfo.TaskName != "" {
		tooltip += fmt.Sprintf("\nTask: %s", s.timerInfo.TaskName)
	}
	if s.timerInfo.ActivityName != "" {
		tooltip += fmt.Sprintf("\nActivity: %s", s.timerInfo.ActivityName)
	}
	tooltip += fmt.Sprintf("\nStarted: %s", s.timerInfo.StartTime.Local().Format("15:04"))
	tooltip += fmt.Sprintf("\nElapsed: %dh %02dm", hours, minutes)

	systray.SetTooltip(tooltip)
}

// startTooltipUpdater starts the periodic tooltip update.
// Captures the ticker/stop channels into locals so stopTooltipUpdater can
// safely nil the struct fields without racing this goroutine (same pattern
// as StartIconRotation).
func (s *SystrayManager) startTooltipUpdater() {
	if s.tooltipTicker != nil {
		return
	}

	s.tooltipTicker = time.NewTicker(30 * time.Second)
	s.stopTooltip = make(chan struct{})

	tickerC := s.tooltipTicker.C
	stopCh := s.stopTooltip

	go func() {
		for {
			select {
			case <-tickerC:
				s.updateTooltip()
			case <-stopCh:
				return
			}
		}
	}()
}

// stopTooltipUpdater stops the periodic tooltip update.
func (s *SystrayManager) stopTooltipUpdater() {
	if s.tooltipTicker != nil {
		s.tooltipTicker.Stop()
		s.tooltipTicker = nil
	}
	if s.stopTooltip != nil {
		close(s.stopTooltip)
		s.stopTooltip = nil
	}
}

// SetTimerStopped updates the tray to indicate no timer is running
func (s *SystrayManager) SetTimerStopped() {
	s.timerInfo = nil
	s.UpdateStatus("No active timer")
	systray.SetTooltip("Kartoza Timesheets - Time tracking made simple")

	// Stop tooltip updater
	s.stopTooltipUpdater()

	// Stop icon rotation
	s.StopIconRotation()
}
