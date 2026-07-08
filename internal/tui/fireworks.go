package tui

import (
	"io"
	"math/rand"
	"time"

	"tim-particles/pkg/audio"
	"tim-particles/pkg/fireworks"
	"tim-particles/pkg/particles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gdamore/tcell/v2"
)

// fireworksCmd is a custom exec.Cmd that runs fireworks
type fireworksCmd struct {
	duration time.Duration
}

// Run implements the tea.ExecCommand interface
func (f *fireworksCmd) Run() error {
	return runFireworksDisplay(f.duration)
}

// SetStdin, SetStdout, SetStderr are required by tea.ExecCommand
func (f *fireworksCmd) SetStdin(_ io.Reader)  {}
func (f *fireworksCmd) SetStdout(_ io.Writer) {}
func (f *fireworksCmd) SetStderr(_ io.Writer) {}

// FireworksCelebrationCmd returns a tea.Cmd that runs a fireworks celebration
// for the specified duration. Use this after a successful submit.
func FireworksCelebrationCmd(duration time.Duration) tea.Cmd {
	return tea.Exec(&fireworksCmd{duration: duration}, func(err error) tea.Msg {
		return fireworksCompleteMsg{err: err}
	})
}

// fireworksCompleteMsg is sent when the fireworks display completes
type fireworksCompleteMsg struct {
	err error
}

// Kartoza brand colors - warm reds, oranges and golds (as RGB values)
var fireworkColors = []tcell.Color{
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

// runFireworksDisplay runs a fullscreen fireworks display for the specified duration.
func runFireworksDisplay(duration time.Duration) error {
	// Create a new tcell screen
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}

	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()

	width, height := screen.Size()
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))
	screen.Clear()

	// Create audio analyzer for reactive fireworks
	audioAnalyzer := audio.NewAnalyzer()
	_ = audioAnalyzer.Start()
	defer audioAnalyzer.Stop()

	// Create fireworks show - use indices as "colors" since we look them up when rendering
	show := fireworks.NewShow(width, height)
	indexColors := make([]particles.Color, len(fireworkColors))
	for i := range fireworkColors {
		indexColors[i] = particles.Color(i) // Store index, not actual color
	}
	show.Colors = indexColors

	running := true
	startTime := time.Now()

	// Handle input in separate goroutine
	go func() {
		for running {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				running = false
				return
			case *tcell.EventResize:
				width, height = screen.Size()
				show.SetBounds(width, height)
				screen.Sync()
			default:
				_ = ev
			}
		}
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	fireworkTimer := time.Now()
	fireworkInterval := 0.3 + rand.Float64()*0.5

	lastUpdate := time.Now()

	for running {
		// Check duration timeout
		if duration > 0 && time.Since(startTime) > duration {
			running = false
			break
		}

		select {
		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(lastUpdate).Seconds()
			lastUpdate = now

			// Get audio data
			audioData := audioAnalyzer.GetData()
			hasAudio := audioAnalyzer.IsEnabled()

			// Determine if we should spawn fireworks
			shouldSpawn, spawnCount := show.ShouldSpawn(audioData, hasAudio)

			// Check particle threshold
			particleCount := show.ActiveParticleCount()
			threshold := show.DynamicParticleThreshold(audioData, hasAudio)

			canSpawn := particleCount < threshold

			// Override for high confidence beats
			if audioData.IsBeat && audioData.BeatConfidence > 0.6 {
				canSpawn = true
			}

			if canSpawn {
				if shouldSpawn && hasAudio {
					for i := 0; i < spawnCount; i++ {
						show.CreateFirework(audioData)
					}
				} else if now.Sub(fireworkTimer).Seconds() > fireworkInterval {
					show.CreateFirework(audioData)
					fireworkTimer = now
					fireworkInterval = 0.3 + rand.Float64()*0.5
				}
			}

			// Update physics
			show.Update(dt)

			// Render - clear screen first!
			screen.Clear()

			// Draw title
			title := "🎉 Timesheets Submitted! Press any key to continue... 🎉"
			style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
			startX := (width - len(title)) / 2
			for i, ch := range title {
				screen.SetContent(startX+i, 0, ch, nil, style)
			}

			// Draw music note indicator when audio syncing
			if audioAnalyzer.IsEnabled() {
				if audioData.Volume > 0.05 || audioData.Energy > 0.05 {
					noteStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)
					noteX := startX + len(title) + 2
					screen.SetContent(noteX, 0, '♪', nil, noteStyle)
				}
			}

			// Draw launching rockets - use color index to look up from our color array
			for _, fw := range show.GetLaunchingRockets() {
				x := int(fw.RocketX)
				y := int(fw.RocketY)

				if x >= 0 && x < width && y >= 1 && y < height {
					colorIdx := int(fw.Color) % len(fireworkColors)
					if colorIdx < 0 {
						colorIdx = 0
					}
					rocketColor := fireworkColors[colorIdx]
					rocketStyle := tcell.StyleDefault.Foreground(rocketColor).Background(tcell.ColorBlack)
					screen.SetContent(x, y, '●', nil, rocketStyle)
				}
			}

			// Draw all particles - use color index to look up from our color array
			for _, p := range show.GetAllParticles() {
				x := int(p.X)
				y := int(p.Y)

				if x < 0 || x >= width || y < 1 || y >= height {
					continue
				}

				// Use color index (stored in p.Color) to look up actual tcell color
				colorIdx := int(p.Color) % len(fireworkColors)
				if colorIdx < 0 {
					colorIdx = 0
				}
				particleColor := fireworkColors[colorIdx]

				pStyle := tcell.StyleDefault.Foreground(particleColor).Background(tcell.ColorBlack)

				if p.Life < 0.2 {
					pStyle = tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
				} else if p.Life < 0.5 {
					pStyle = pStyle.Dim(true)
				}

				screen.SetContent(x, y, p.Char, nil, pStyle)
			}

			screen.Show()
		}
	}

	return nil
}
