package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/cobra"
	"tim-particles/pkg/audio"
	"tim-particles/pkg/fireworks"
	"tim-particles/pkg/particles"
)

var celebrationDuration int

// celebrationCmd represents the celebration command
var celebrationCmd = &cobra.Command{
	Use:   "celebration",
	Short: "Launch a fireworks celebration",
	Long: `Launch a fullscreen fireworks celebration display.

This command displays an impressive fireworks show in your terminal.
Press any key to exit, or wait for the duration to complete.

Examples:
  kartoza-timesheet celebration
  kartoza-timesheet celebration --duration 30`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCelebration(time.Duration(celebrationDuration) * time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(celebrationCmd)

	celebrationCmd.Flags().IntVarP(&celebrationDuration, "duration", "t", 60, "Duration in seconds (0 for unlimited)")
}

// CelebrationShow manages the terminal-based fireworks display
type CelebrationShow struct {
	screen    tcell.Screen
	show      *fireworks.Show
	width     int
	height    int
	running   bool
	audio     *audio.Analyzer
	duration  time.Duration
	startTime time.Time
}

// Kartoza brand colors - warm reds, oranges and golds (as RGB values)
// Using tcell.ColorIsRGB | tcell.ColorValid | RGB to ensure proper color handling
var celebrationColors = []tcell.Color{
	tcell.NewRGBColor(233, 84, 32),   // Kartoza Orange #E95420
	tcell.NewRGBColor(255, 0, 0),     // Red
	tcell.NewRGBColor(255, 69, 0),    // OrangeRed
	tcell.NewRGBColor(255, 140, 0),   // DarkOrange
	tcell.NewRGBColor(255, 165, 0),   // Orange
	tcell.NewRGBColor(220, 20, 60),   // Crimson
	tcell.NewRGBColor(178, 34, 34),   // FireBrick
	tcell.NewRGBColor(205, 92, 92),   // IndianRed
	tcell.NewRGBColor(139, 0, 0),     // DarkRed
	tcell.NewRGBColor(255, 215, 0),   // Gold
	tcell.NewRGBColor(255, 255, 0),   // Yellow
	tcell.NewRGBColor(240, 128, 128), // LightCoral
	tcell.NewRGBColor(250, 128, 114), // Salmon
	tcell.NewRGBColor(255, 99, 71),   // Tomato
	tcell.NewRGBColor(255, 127, 80),  // Coral
	tcell.NewRGBColor(255, 160, 122), // LightSalmon
}

func newCelebrationShow(duration time.Duration) (*CelebrationShow, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	if err := screen.Init(); err != nil {
		return nil, err
	}

	width, height := screen.Size()
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))
	screen.Clear()

	audioAnalyzer := audio.NewAnalyzer()
	audioAnalyzer.Start()

	// Create fireworks show - use indices as "colors" since we look them up when rendering
	show := fireworks.NewShow(width, height)
	indexColors := make([]particles.Color, len(celebrationColors))
	for i := range celebrationColors {
		indexColors[i] = particles.Color(i) // Store index, not actual color
	}
	show.Colors = indexColors

	return &CelebrationShow{
		screen:    screen,
		show:      show,
		width:     width,
		height:    height,
		running:   true,
		audio:     audioAnalyzer,
		duration:  duration,
		startTime: time.Now(),
	}, nil
}

func (cs *CelebrationShow) render() {
	cs.screen.Clear()

	// Draw title
	title := "🎉 Kartoza Celebration! Press any key to exit... 🎉"
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	startX := (cs.width - len(title)) / 2
	for i, ch := range title {
		cs.screen.SetContent(startX+i, 0, ch, nil, style)
	}

	// Draw music note indicator when audio syncing
	if cs.audio.IsEnabled() {
		audioData := cs.audio.GetData()
		if audioData.Volume > 0.05 || audioData.Energy > 0.05 {
			noteStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)
			noteX := startX + len(title) + 2
			cs.screen.SetContent(noteX, 0, '♪', nil, noteStyle)
		}
	}

	// Draw launching rockets - use color index to look up from our color array
	for _, fw := range cs.show.GetLaunchingRockets() {
		x := int(fw.RocketX)
		y := int(fw.RocketY)

		if x >= 0 && x < cs.width && y >= 1 && y < cs.height {
			colorIdx := int(fw.Color) % len(celebrationColors)
			if colorIdx < 0 {
				colorIdx = 0
			}
			rocketColor := celebrationColors[colorIdx]
			style := tcell.StyleDefault.Foreground(rocketColor).Background(tcell.ColorBlack)
			cs.screen.SetContent(x, y, '●', nil, style)
		}
	}

	// Draw all particles - use color index to look up from our color array
	for _, p := range cs.show.GetAllParticles() {
		x := int(p.X)
		y := int(p.Y)

		if x < 0 || x >= cs.width || y < 1 || y >= cs.height {
			continue
		}

		// Use color index (stored in p.Color) to look up actual tcell color
		colorIdx := int(p.Color) % len(celebrationColors)
		if colorIdx < 0 {
			colorIdx = 0
		}
		particleColor := celebrationColors[colorIdx]

		style := tcell.StyleDefault.Foreground(particleColor).Background(tcell.ColorBlack)

		if p.Life < 0.2 {
			style = tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
		} else if p.Life < 0.5 {
			style = style.Dim(true)
		}

		cs.screen.SetContent(x, y, p.Char, nil, style)
	}

	cs.screen.Show()
}

func (cs *CelebrationShow) handleInput() {
	for cs.running {
		ev := cs.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			cs.running = false
			return
		case *tcell.EventResize:
			cs.width, cs.height = cs.screen.Size()
			cs.show.SetBounds(cs.width, cs.height)
			cs.screen.Sync()
		default:
			_ = ev
		}
	}
}

func (cs *CelebrationShow) Run() {
	defer cs.screen.Fini()
	defer cs.audio.Stop()

	// Handle input in separate goroutine
	go cs.handleInput()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	fireworkTimer := time.Now()
	fireworkInterval := 0.3 + rand.Float64()*0.5 // Faster spawning for celebration

	lastUpdate := time.Now()

	for cs.running {
		// Check duration timeout
		if cs.duration > 0 && time.Since(cs.startTime) > cs.duration {
			cs.running = false
			break
		}

		select {
		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(lastUpdate).Seconds()
			lastUpdate = now

			// Get audio data
			audioData := cs.audio.GetData()
			hasAudio := cs.audio.IsEnabled()

			// Determine if we should spawn fireworks
			shouldSpawn, spawnCount := cs.show.ShouldSpawn(audioData, hasAudio)

			// Check particle threshold
			particleCount := cs.show.ActiveParticleCount()
			threshold := cs.show.DynamicParticleThreshold(audioData, hasAudio)

			canSpawn := particleCount < threshold

			// Override for high confidence beats
			if audioData.IsBeat && audioData.BeatConfidence > 0.6 {
				canSpawn = true
			}

			if canSpawn {
				if shouldSpawn && hasAudio {
					for i := 0; i < spawnCount; i++ {
						cs.show.CreateFirework(audioData)
					}
				} else if now.Sub(fireworkTimer).Seconds() > fireworkInterval {
					// Faster random spawning for celebration
					cs.show.CreateFirework(audioData)
					fireworkTimer = now
					fireworkInterval = 0.3 + rand.Float64()*0.5
				}
			}

			// Update physics
			cs.show.Update(dt)

			// Render
			cs.render()
		}
	}
}

// runCelebration runs the fireworks celebration display
func runCelebration(duration time.Duration) error {
	show, err := newCelebrationShow(duration)
	if err != nil {
		return err
	}

	show.Run()
	return nil
}
