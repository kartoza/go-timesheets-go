package tui

import (
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gdamore/tcell/v2"
	"tim-particles/pkg/display"
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

	// Clear screen and set black background
	screen.Clear()
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))

	// Create fireworks display with title
	config := display.DefaultConfig()
	config.ShowTitle = true
	config.Title = "Timesheets Submitted! Press any key to continue..."
	config.FallbackSpawnRate = 0.8 // Faster fireworks

	fw, err := display.NewWithConfig(screen, config)
	if err != nil {
		return err
	}
	defer fw.Stop()

	// Start the fireworks display
	fw.Start()

	// Set up timeout
	timeout := time.After(duration)

	// Poll for events with timeout
	eventChan := make(chan struct{})
	go func() {
		for {
			ev := screen.PollEvent()
			if ev != nil {
				switch ev.(type) {
				case *tcell.EventKey:
					eventChan <- struct{}{}
					return
				case *tcell.EventResize:
					// Handle resize - update bounds
					width, height := screen.Size()
					fw.SetBounds(0, 0, width, height)
					screen.Sync()
				}
			}
		}
	}()

	// Wait for timeout or keypress
	select {
	case <-timeout:
		// Duration elapsed
	case <-eventChan:
		// User pressed a key
	}

	return nil
}
