package office

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Run starts the office scene
func Run() error {
	// Fetch team members
	fmt.Println("Fetching Kartoza team data...")
	members, err := FetchTeamMembers()
	if err != nil {
		fmt.Printf("Warning: Could not fetch team data: %v\n", err)
		fmt.Println("Using fallback team data...")
		members = getHardcodedTeam()
	}
	fmt.Printf("Loaded %d team members\n", len(members))

	// Create screen
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create screen: %w", err)
	}

	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to init screen: %w", err)
	}
	defer screen.Fini()

	// Setup screen
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))
	screen.Clear()

	// Create office
	office := NewOffice(screen, members)

	// Start event handling
	eventChan := make(chan tcell.Event)
	go func() {
		for office.Running {
			eventChan <- screen.PollEvent()
		}
	}()

	// Main loop
	ticker := time.NewTicker(50 * time.Millisecond) // 20 FPS
	defer ticker.Stop()

	lastUpdate := time.Now()

	for office.Running {
		select {
		case ev := <-eventChan:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape:
					office.Running = false
				case tcell.KeyRune:
					switch ev.Rune() {
					case 'q', 'Q':
						office.Running = false
					case 'b', 'B':
						office.ToggleBubbles()
					}
				}
			case *tcell.EventResize:
				width, height := screen.Size()
				office.SetBounds(width, height)
				screen.Sync()
			}

		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(lastUpdate).Seconds()
			lastUpdate = now

			office.Update(dt)
			office.Render()
		}
	}

	return nil
}
