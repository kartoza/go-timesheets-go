package tui

import (
	"bytes"
	"embed"
	"fmt"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/blacktop/go-termimg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nfnt/resize"
)

//go:embed resources/kartoza-logo.png
var logoFS embed.FS

// SplashModel represents the splash screen state
type SplashModel struct {
	width          int
	height         int
	kittySupported bool
	logoImage      []byte // Store as PNG bytes for efficiency
	showDuration   time.Duration
	startTime      time.Time
	elapsed        time.Duration
	done           bool
	scale          float64 // Current scale factor (1.0 = full size, 0.1 = tiny)
	lastImageID    int     // Track image ID for Kitty protocol
}

// splashDoneMsg signals the splash screen is complete
type splashDoneMsg struct{}

// splashTickMsg for animation updates
type splashTickMsg time.Time

// NewSplashModel creates a new splash screen model
func NewSplashModel(duration time.Duration) *SplashModel {
	sm := &SplashModel{
		showDuration:   duration,
		kittySupported: detectKittySupport(),
		scale:          0.05, // Start tiny (expand animation)
		lastImageID:    1000,
	}

	// Load the embedded logo as raw bytes
	sm.loadLogo()

	return sm
}

// detectKittySupport checks if the terminal supports Kitty graphics protocol
func detectKittySupport() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	kittyWindowID := os.Getenv("KITTY_WINDOW_ID")

	// Check for Kitty terminal
	if kittyWindowID != "" {
		return true
	}

	// Check for xterm-kitty or kitty in TERM
	if strings.Contains(term, "kitty") {
		return true
	}

	// Check TERM_PROGRAM
	if termProgram == "kitty" {
		return true
	}

	// Try using go-termimg's detection
	protocol := termimg.DetectProtocol()
	return protocol == termimg.Kitty
}

// loadLogo loads the logo from embedded resources
func (sm *SplashModel) loadLogo() {
	data, err := logoFS.ReadFile("resources/kartoza-logo.png")
	if err != nil {
		return
	}
	sm.logoImage = data
}

// Init initializes the splash screen
func (sm *SplashModel) Init() tea.Cmd {
	sm.startTime = time.Now()
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return splashTickMsg(t)
	})
}

// Update handles messages for the splash screen
func (sm *SplashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		sm.width = msg.Width
		sm.height = msg.Height

	case tea.KeyMsg:
		// Allow skipping splash with any key
		sm.done = true
		return sm, tea.Quit

	case splashTickMsg:
		sm.elapsed = time.Since(sm.startTime)

		// Calculate progress (0.0 to 1.0)
		progress := float64(sm.elapsed) / float64(sm.showDuration)
		if progress >= 1.0 {
			sm.done = true
			return sm, tea.Quit
		}

		// Apply easing function for smooth expand animation
		// Using ease-in-out cubic: slow start, fast middle, slow end
		// Scale goes from 0.05 up to 1.0 (5% to 100% of original size)
		easedProgress := easeInOutCubic(progress)
		sm.scale = 0.05 + (easedProgress * 0.95) // 0.05 -> 1.0

		// Continue ticking for animation (30fps = ~33ms per frame)
		return sm, tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
			return splashTickMsg(t)
		})

	case splashDoneMsg:
		sm.done = true
		return sm, tea.Quit
	}

	return sm, nil
}

// View renders the splash screen
func (sm *SplashModel) View() string {
	if sm.width == 0 || sm.height == 0 {
		return ""
	}

	var content string

	if sm.kittySupported && len(sm.logoImage) > 0 {
		content = sm.renderWithKitty()
	} else {
		content = sm.renderTextSplash()
	}

	return content
}

// renderWithKitty renders the splash screen with the Kitty graphics protocol
// Shows ONLY the logo centered on screen - center of image at center of terminal
func (sm *SplashModel) renderWithKitty() string {
	// Decode the PNG to get dimensions
	img, err := png.Decode(bytes.NewReader(sm.logoImage))
	if err != nil {
		return sm.renderTextSplash()
	}

	// Calculate base logo size in terminal cells (about 1/3 screen width at full scale)
	baseWidthCells := sm.width / 3
	if baseWidthCells < 20 {
		baseWidthCells = 20
	}
	if baseWidthCells > 60 {
		baseWidthCells = 60
	}

	// Apply scale factor for shrink animation
	logoWidthCells := int(float64(baseWidthCells) * sm.scale)
	if logoWidthCells < 2 {
		logoWidthCells = 2
	}

	// Calculate logo height based on aspect ratio
	// Terminal cells are typically ~2:1 (height:width ratio in pixels)
	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dx()) / float64(bounds.Dy())
	// Account for terminal cell aspect ratio (~2:1)
	logoHeightCells := int(float64(logoWidthCells) / aspectRatio / 2.0)
	if logoHeightCells < 1 {
		logoHeightCells = 1
	}

	// Resize the logo based on current scale
	pixelWidth := uint(logoWidthCells * 8)
	pixelHeight := uint(float64(pixelWidth) / aspectRatio)
	if pixelWidth < 8 {
		pixelWidth = 8
	}
	if pixelHeight < 8 {
		pixelHeight = 8
	}
	resizedLogo := resize.Resize(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedLogo); err != nil {
		return sm.renderTextSplash()
	}

	// Use go-termimg to render with Kitty protocol
	ti, err := termimg.From(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return sm.renderTextSplash()
	}

	// Use unique image ID for each frame to force re-render
	sm.lastImageID++

	// Configure for display
	ti.Protocol(termimg.Kitty).
		Width(logoWidthCells).
		Height(logoHeightCells).
		Scale(termimg.ScaleFit).
		ImageNum(sm.lastImageID)

	rendered, err := ti.Render()
	if err != nil {
		return sm.renderTextSplash()
	}

	// Calculate position to center the image
	// Center of image should be at center of terminal
	centerX := (sm.width - logoWidthCells) / 2
	centerY := (sm.height - logoHeightCells) / 2

	// Build output: clear previous image, then render new one at center
	var output strings.Builder
	// Delete all previous images
	output.WriteString("\033_Ga=d\033\\")
	// Position cursor and render image
	output.WriteString(fmt.Sprintf("\033[%d;%dH%s", centerY+1, centerX+1, rendered))

	return output.String()
}

// renderTextSplash renders a text-based splash screen (fallback)
// Shows ONLY the ASCII logo centered on screen - center of logo at center of terminal
func (sm *SplashModel) renderTextSplash() string {
	// ASCII art logo - just KARTOZA (no leading newline for accurate height calculation)
	asciiLogo := `    ██╗  ██╗ █████╗ ██████╗ ████████╗ ██████╗ ███████╗ █████╗
    ██║ ██╔╝██╔══██╗██╔══██╗╚══██╔══╝██╔═══██╗╚══███╔╝██╔══██╗
    █████╔╝ ███████║██████╔╝   ██║   ██║   ██║  ███╔╝ ███████║
    ██╔═██╗ ██╔══██║██╔══██╗   ██║   ██║   ██║ ███╔╝  ██╔══██║
    ██║  ██╗██║  ██║██║  ██║   ██║   ╚██████╔╝███████╗██║  ██║
    ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚══════╝╚═╝  ╚═╝`

	// Calculate logo dimensions
	lines := strings.Split(asciiLogo, "\n")
	logoHeight := len(lines)
	logoWidth := 0
	for _, line := range lines {
		if len(line) > logoWidth {
			logoWidth = len(line)
		}
	}

	logoStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true)

	styledLogo := logoStyle.Render(asciiLogo)

	// Calculate position to center the logo
	// Center of logo should be at center of terminal
	centerX := (sm.width - logoWidth) / 2
	centerY := (sm.height - logoHeight) / 2

	// Build output with cursor positioning for each line
	var output strings.Builder
	styledLines := strings.Split(styledLogo, "\n")
	for i, line := range styledLines {
		// Position cursor for this line
		row := centerY + i + 1 // 1-indexed
		col := centerX + 1     // 1-indexed
		if col < 1 {
			col = 1
		}
		if row < 1 {
			row = 1
		}
		output.WriteString(fmt.Sprintf("\033[%d;%dH%s", row, col, line))
	}

	return output.String()
}

// IsDone returns whether the splash screen is complete
func (sm *SplashModel) IsDone() bool {
	return sm.done
}

// ShowSplashScreen displays the splash screen as a standalone program
// It runs for the specified duration, then exits so the main app can start
func ShowSplashScreen(duration time.Duration) error {
	splash := NewSplashModel(duration)

	p := tea.NewProgram(splash, tea.WithAltScreen())

	_, err := p.Run()
	return err
}

// SplashWrapper wraps the main app with a splash screen
type SplashWrapper struct {
	splash        *SplashModel
	mainApp       tea.Model
	showingSplash bool
}

// NewSplashWrapper creates a new splash wrapper around the main app
func NewSplashWrapper(mainApp tea.Model, splashDuration time.Duration) *SplashWrapper {
	return &SplashWrapper{
		splash:        NewSplashModel(splashDuration),
		mainApp:       mainApp,
		showingSplash: true,
	}
}

// Init initializes the splash wrapper
func (sw *SplashWrapper) Init() tea.Cmd {
	return sw.splash.Init()
}

// Update handles messages for the splash wrapper
func (sw *SplashWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sw.showingSplash {
		// Handle window size for both splash and main app
		if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
			sw.splash.width = wsMsg.Width
			sw.splash.height = wsMsg.Height
		}

		model, cmd := sw.splash.Update(msg)
		if m, ok := model.(*SplashModel); ok {
			sw.splash = m
		}

		if sw.splash.IsDone() {
			sw.showingSplash = false
			// Initialize the main app
			return sw, sw.mainApp.Init()
		}

		return sw, cmd
	}

	// Pass to main app
	var cmd tea.Cmd
	sw.mainApp, cmd = sw.mainApp.Update(msg)
	return sw, cmd
}

// View renders the current view
func (sw *SplashWrapper) View() string {
	if sw.showingSplash {
		return sw.splash.View()
	}
	return sw.mainApp.View()
}

// easeInOutCubic provides smooth acceleration and deceleration
// t is progress from 0.0 to 1.0, returns eased value from 0.0 to 1.0
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
}

// Alternative easing functions for different effects:

// easeOutExpo - fast start, slow end (like a bouncing ball coming to rest)
func easeOutExpo(t float64) float64 {
	if t == 1 {
		return 1
	}
	return 1 - mathPow(2, -10*t)
}

// easeInExpo - slow start, fast end
func easeInExpo(t float64) float64 {
	if t == 0 {
		return 0
	}
	return mathPow(2, 10*(t-1))
}

// mathPow is a simple power function for the easing calculations
func mathPow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	// Handle fractional exponents approximately
	if exp != float64(int(exp)) {
		// Use repeated multiplication approximation
		frac := exp - float64(int(exp))
		result *= (1 + frac*(base-1))
	}
	return result
}

// ExitSplashModel represents the exit splash screen state (reverse of entry splash)
// It starts small and expands to full size
type ExitSplashModel struct {
	width          int
	height         int
	kittySupported bool
	logoImage      []byte
	showDuration   time.Duration
	startTime      time.Time
	elapsed        time.Duration
	done           bool
	scale          float64 // Current scale factor (0.05 = tiny, 1.0 = full size)
	lastImageID    int
}

// exitSplashDoneMsg signals the exit splash screen is complete
type exitSplashDoneMsg struct{}

// exitSplashTickMsg for animation updates
type exitSplashTickMsg time.Time

// NewExitSplashModel creates a new exit splash screen model
func NewExitSplashModel(duration time.Duration) *ExitSplashModel {
	esm := &ExitSplashModel{
		showDuration:   duration,
		kittySupported: detectKittySupport(),
		scale:          1.0, // Start at full size (shrink animation)
		lastImageID:    2000,
	}

	// Load the embedded logo as raw bytes
	esm.loadLogo()

	return esm
}

// loadLogo loads the logo from embedded resources
func (esm *ExitSplashModel) loadLogo() {
	data, err := logoFS.ReadFile("resources/kartoza-logo.png")
	if err != nil {
		return
	}
	esm.logoImage = data
}

// Init initializes the exit splash screen
func (esm *ExitSplashModel) Init() tea.Cmd {
	esm.startTime = time.Now()
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return exitSplashTickMsg(t)
	})
}

// Update handles messages for the exit splash screen
func (esm *ExitSplashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		esm.width = msg.Width
		esm.height = msg.Height

	case tea.KeyMsg:
		// Allow skipping exit splash with any key
		esm.done = true
		return esm, tea.Quit

	case exitSplashTickMsg:
		esm.elapsed = time.Since(esm.startTime)

		// Calculate progress (0.0 to 1.0)
		progress := float64(esm.elapsed) / float64(esm.showDuration)
		if progress >= 1.0 {
			esm.done = true
			return esm, tea.Quit
		}

		// Apply easing function for smooth shrink animation
		// Using ease-in-out cubic: slow start, fast middle, slow end
		// Scale goes from 1.0 down to 0.05 (100% to 5% of original size)
		easedProgress := easeInOutCubic(progress)
		esm.scale = 1.0 - (easedProgress * 0.95) // 1.0 -> 0.05

		// Continue ticking for animation (30fps = ~33ms per frame)
		return esm, tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
			return exitSplashTickMsg(t)
		})

	case exitSplashDoneMsg:
		esm.done = true
		return esm, tea.Quit
	}

	return esm, nil
}

// View renders the exit splash screen
func (esm *ExitSplashModel) View() string {
	if esm.width == 0 || esm.height == 0 {
		return ""
	}

	var content string

	if esm.kittySupported && len(esm.logoImage) > 0 {
		content = esm.renderWithKitty()
	} else {
		content = esm.renderTextSplash()
	}

	return content
}

// renderWithKitty renders the exit splash screen with the Kitty graphics protocol
func (esm *ExitSplashModel) renderWithKitty() string {
	// Decode the PNG to get dimensions
	img, err := png.Decode(bytes.NewReader(esm.logoImage))
	if err != nil {
		return esm.renderTextSplash()
	}

	// Calculate base logo size in terminal cells (about 1/3 screen width at full scale)
	baseWidthCells := esm.width / 3
	if baseWidthCells < 20 {
		baseWidthCells = 20
	}
	if baseWidthCells > 60 {
		baseWidthCells = 60
	}

	// Apply scale factor for expand animation
	logoWidthCells := int(float64(baseWidthCells) * esm.scale)
	if logoWidthCells < 2 {
		logoWidthCells = 2
	}

	// Calculate logo height based on aspect ratio
	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dx()) / float64(bounds.Dy())
	logoHeightCells := int(float64(logoWidthCells) / aspectRatio / 2.0)
	if logoHeightCells < 1 {
		logoHeightCells = 1
	}

	// Resize the logo based on current scale
	pixelWidth := uint(logoWidthCells * 8)
	pixelHeight := uint(float64(pixelWidth) / aspectRatio)
	if pixelWidth < 8 {
		pixelWidth = 8
	}
	if pixelHeight < 8 {
		pixelHeight = 8
	}
	resizedLogo := resize.Resize(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedLogo); err != nil {
		return esm.renderTextSplash()
	}

	// Use go-termimg to render with Kitty protocol
	ti, err := termimg.From(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return esm.renderTextSplash()
	}

	// Use unique image ID for each frame
	esm.lastImageID++

	// Configure for display
	ti.Protocol(termimg.Kitty).
		Width(logoWidthCells).
		Height(logoHeightCells).
		Scale(termimg.ScaleFit).
		ImageNum(esm.lastImageID)

	rendered, err := ti.Render()
	if err != nil {
		return esm.renderTextSplash()
	}

	// Calculate position to center the image
	centerX := (esm.width - logoWidthCells) / 2
	centerY := (esm.height - logoHeightCells) / 2

	// Build output: clear previous image, then render new one at center
	var output strings.Builder
	// Delete all previous images
	output.WriteString("\033_Ga=d\033\\")
	// Position cursor and render image
	output.WriteString(fmt.Sprintf("\033[%d;%dH%s", centerY+1, centerX+1, rendered))

	return output.String()
}

// renderTextSplash renders a text-based exit splash screen (fallback)
func (esm *ExitSplashModel) renderTextSplash() string {
	// ASCII art logo - just KARTOZA
	asciiLogo := `    ██╗  ██╗ █████╗ ██████╗ ████████╗ ██████╗ ███████╗ █████╗
    ██║ ██╔╝██╔══██╗██╔══██╗╚══██╔══╝██╔═══██╗╚══███╔╝██╔══██╗
    █████╔╝ ███████║██████╔╝   ██║   ██║   ██║  ███╔╝ ███████║
    ██╔═██╗ ██╔══██║██╔══██╗   ██║   ██║   ██║ ███╔╝  ██╔══██║
    ██║  ██╗██║  ██║██║  ██║   ██║   ╚██████╔╝███████╗██║  ██║
    ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚══════╝╚═╝  ╚═╝`

	// Calculate logo dimensions
	lines := strings.Split(asciiLogo, "\n")
	logoHeight := len(lines)
	logoWidth := 0
	for _, line := range lines {
		if len(line) > logoWidth {
			logoWidth = len(line)
		}
	}

	logoStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true)

	styledLogo := logoStyle.Render(asciiLogo)

	// Calculate position to center the logo
	centerX := (esm.width - logoWidth) / 2
	centerY := (esm.height - logoHeight) / 2

	// Build output with cursor positioning for each line
	var output strings.Builder
	styledLines := strings.Split(styledLogo, "\n")
	for i, line := range styledLines {
		row := centerY + i + 1
		col := centerX + 1
		if col < 1 {
			col = 1
		}
		if row < 1 {
			row = 1
		}
		output.WriteString(fmt.Sprintf("\033[%d;%dH%s", row, col, line))
	}

	return output.String()
}

// IsDone returns whether the exit splash screen is complete
func (esm *ExitSplashModel) IsDone() bool {
	return esm.done
}

// ShowExitSplashScreen displays the exit splash screen as a standalone program
func ShowExitSplashScreen(duration time.Duration) error {
	splash := NewExitSplashModel(duration)

	p := tea.NewProgram(splash, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
