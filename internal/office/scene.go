package office

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Office represents the office scene
type Office struct {
	Width      int
	Height     int
	Members    []TeamMember
	Furniture  []Furniture
	Screen     tcell.Screen
	Running    bool
	ShowBubbles bool
	lastUpdate time.Time
}

// Furniture represents an office furniture item
type Furniture struct {
	Type   FurnitureType
	X, Y   int
	Width  int
	Height int
}

// FurnitureType enum
type FurnitureType int

const (
	FurnDesk FurnitureType = iota
	FurnChair
	FurnComputer
	FurnPlant
	FurnCoffeeMachine
	FurnWhiteboard
	FurnWaterCooler
	FurnWall
	FurnDoor
)

// NewOffice creates a new office scene
func NewOffice(screen tcell.Screen, members []TeamMember) *Office {
	width, height := screen.Size()

	office := &Office{
		Width:       width,
		Height:      height,
		Members:     members,
		Screen:      screen,
		Running:     true,
		ShowBubbles: true,
		lastUpdate:  time.Now(),
	}

	office.setupFurniture()
	office.placeMembers()

	return office
}

// setupFurniture creates the office layout
func (o *Office) setupFurniture() {
	// Create desk clusters for team members
	// Office layout: rows of desks with aisles

	// Top wall
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWall, X: 0, Y: 0, Width: o.Width, Height: 1,
	})

	// Bottom wall
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWall, X: 0, Y: o.Height - 1, Width: o.Width, Height: 1,
	})

	// Left wall
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWall, X: 0, Y: 0, Width: 1, Height: o.Height,
	})

	// Right wall
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWall, X: o.Width - 1, Y: 0, Width: 1, Height: o.Height,
	})

	// Door
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnDoor, X: o.Width / 2, Y: o.Height - 1, Width: 4, Height: 1,
	})

	// Create desk rows
	deskStartX := 5
	deskStartY := 6
	deskSpacingX := 12
	deskSpacingY := 8

	desksPerRow := (o.Width - 10) / deskSpacingX
	if desksPerRow < 1 {
		desksPerRow = 1
	}

	numDesks := len(o.Members)
	if numDesks > 20 {
		numDesks = 20 // Cap at 20 desks
	}

	for i := 0; i < numDesks; i++ {
		row := i / desksPerRow
		col := i % desksPerRow

		x := deskStartX + col*deskSpacingX
		y := deskStartY + row*deskSpacingY

		if y+4 >= o.Height-2 {
			break
		}

		// Add desk
		o.Furniture = append(o.Furniture, Furniture{
			Type: FurnDesk, X: x, Y: y, Width: 6, Height: 2,
		})

		// Add computer on desk
		o.Furniture = append(o.Furniture, Furniture{
			Type: FurnComputer, X: x + 1, Y: y - 1, Width: 3, Height: 2,
		})

		// Add chair
		o.Furniture = append(o.Furniture, Furniture{
			Type: FurnChair, X: x + 2, Y: y + 2, Width: 2, Height: 1,
		})
	}

	// Add decorative items

	// Whiteboard near top
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWhiteboard, X: o.Width - 12, Y: 2, Width: 8, Height: 3,
	})

	// Coffee machine
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnCoffeeMachine, X: 3, Y: 2, Width: 3, Height: 2,
	})

	// Water cooler
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnWaterCooler, X: 3, Y: o.Height/2, Width: 2, Height: 3,
	})

	// Plants in corners
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnPlant, X: 2, Y: o.Height - 4, Width: 2, Height: 2,
	})
	o.Furniture = append(o.Furniture, Furniture{
		Type: FurnPlant, X: o.Width - 4, Y: o.Height - 4, Width: 2, Height: 2,
	})
}

// placeMembers positions team members at their desks
func (o *Office) placeMembers() {
	deskStartX := 5
	deskStartY := 6
	deskSpacingX := 12
	deskSpacingY := 8
	desksPerRow := (o.Width - 10) / deskSpacingX
	if desksPerRow < 1 {
		desksPerRow = 1
	}

	for i := range o.Members {
		if i >= 20 {
			// Place extra members in the aisle area
			o.Members[i].X = float64(rand.Intn(o.Width-10) + 5)
			o.Members[i].Y = float64(rand.Intn(o.Height-10) + 5)
			o.Members[i].State = StateWalking
			continue
		}

		row := i / desksPerRow
		col := i % desksPerRow

		x := float64(deskStartX + col*deskSpacingX + 2)
		y := float64(deskStartY + row*deskSpacingY + 3)

		if y+4 >= float64(o.Height-2) {
			// Place in aisle if no more desk space
			x = float64(rand.Intn(o.Width-10) + 5)
			y = float64(rand.Intn(o.Height-10) + 5)
			o.Members[i].State = StateWalking
		} else {
			o.Members[i].State = StateWorking
		}

		o.Members[i].X = x
		o.Members[i].Y = y
		o.Members[i].TargetX = x
		o.Members[i].TargetY = y
		o.Members[i].Facing = DirDown

		// Assign random activities if not set
		if o.Members[i].Activity == "" {
			o.Members[i].Activity = getRandomActivity(o.Members[i].Role)
		}
	}
}

// getRandomActivity returns a plausible activity based on role
func getRandomActivity(role string) string {
	activities := map[string][]string{
		"Director": {
			"Strategic planning",
			"Client call",
			"Reviewing proposals",
			"Budget review",
			"Team meeting",
		},
		"Developer": {
			"Writing code",
			"Code review",
			"Fixing bugs",
			"Testing features",
			"Documentation",
			"Refactoring",
		},
		"DevOps": {
			"Server maintenance",
			"CI/CD pipeline",
			"Monitoring alerts",
			"Deployment",
			"Security audit",
		},
		"GIS": {
			"Map styling",
			"Data analysis",
			"QGIS plugin",
			"Spatial queries",
			"Cartography",
		},
		"Manager": {
			"Sprint planning",
			"Stand-up meeting",
			"Resource allocation",
			"Status update",
			"Team sync",
		},
		"Training": {
			"Course creation",
			"Student support",
			"Material review",
			"Workshop prep",
		},
		"default": {
			"Deep work",
			"Email",
			"Meeting",
			"Research",
			"Documentation",
		},
	}

	// Match role to activity category
	var category string
	roleLower := role
	if contains(roleLower, "director") {
		category = "Director"
	} else if contains(roleLower, "developer") || contains(roleLower, "software") {
		category = "Developer"
	} else if contains(roleLower, "devops") || contains(roleLower, "ops") {
		category = "DevOps"
	} else if contains(roleLower, "gis") || contains(roleLower, "qgis") {
		category = "GIS"
	} else if contains(roleLower, "manager") || contains(roleLower, "scrum") {
		category = "Manager"
	} else if contains(roleLower, "training") {
		category = "Training"
	} else {
		category = "default"
	}

	acts := activities[category]
	return acts[rand.Intn(len(acts))]
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
		// Case insensitive check
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			pc := substr[j]
			// Simple lowercase conversion for ASCII
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 32
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Update updates the office simulation
func (o *Office) Update(dt float64) {
	for i := range o.Members {
		o.updateMember(&o.Members[i], dt)
	}
}

// updateMember updates a single team member's state
func (o *Office) updateMember(m *TeamMember, dt float64) {
	// Update animation frame
	m.Frame++

	switch m.State {
	case StateWorking:
		// Occasionally stand up and walk around
		m.IdleTime += dt
		if m.IdleTime > 10+rand.Float64()*20 {
			m.State = StateWalking
			m.IdleTime = 0
			// Pick a random destination
			m.TargetX = float64(rand.Intn(o.Width-10) + 5)
			m.TargetY = float64(rand.Intn(o.Height-10) + 5)
		}

	case StateWalking:
		// Move towards target
		dx := m.TargetX - m.X
		dy := m.TargetY - m.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 1 {
			// Reached destination
			m.State = StateIdle
			m.IdleTime = 0
		} else {
			// Move towards target
			speed := 3.0 * dt
			m.X += (dx / dist) * speed
			m.Y += (dy / dist) * speed

			// Update facing direction
			if math.Abs(dx) > math.Abs(dy) {
				if dx > 0 {
					m.Facing = DirRight
				} else {
					m.Facing = DirLeft
				}
			} else {
				if dy > 0 {
					m.Facing = DirDown
				} else {
					m.Facing = DirUp
				}
			}
		}

	case StateIdle:
		m.IdleTime += dt
		if m.IdleTime > 3+rand.Float64()*5 {
			// Either go back to desk or walk somewhere
			if rand.Float32() < 0.7 {
				m.State = StateWorking
				// Return to original desk position (simplified)
			} else {
				m.State = StateWalking
				m.TargetX = float64(rand.Intn(o.Width-10) + 5)
				m.TargetY = float64(rand.Intn(o.Height-10) + 5)
			}
			m.IdleTime = 0
		}

	case StateTalking:
		m.IdleTime += dt
		if m.IdleTime > 5 {
			m.State = StateIdle
			m.IdleTime = 0
		}
	}

	// Boundary checks
	if m.X < 2 {
		m.X = 2
	}
	if m.X > float64(o.Width-4) {
		m.X = float64(o.Width - 4)
	}
	if m.Y < 4 {
		m.Y = 4
	}
	if m.Y > float64(o.Height-5) {
		m.Y = float64(o.Height - 5)
	}
}

// Render draws the entire office scene
func (o *Office) Render() {
	o.Screen.Clear()

	// Draw floor
	o.drawFloor()

	// Draw furniture
	for _, f := range o.Furniture {
		o.drawFurniture(f)
	}

	// Draw team members (sorted by Y for proper layering)
	o.drawMembers()

	// Draw title
	o.drawTitle()

	// Draw help text
	o.drawHelp()

	o.Screen.Show()
}

func (o *Office) drawFloor() {
	floorStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack)

	for y := 1; y < o.Height-1; y++ {
		for x := 1; x < o.Width-1; x++ {
			// Checkerboard pattern
			if (x+y)%2 == 0 {
				o.Screen.SetContent(x, y, '·', nil, floorStyle)
			} else {
				o.Screen.SetContent(x, y, ' ', nil, floorStyle)
			}
		}
	}
}

func (o *Office) drawFurniture(f Furniture) {
	switch f.Type {
	case FurnWall:
		o.drawWall(f)
	case FurnDoor:
		o.drawDoor(f)
	case FurnDesk:
		o.drawDesk(f)
	case FurnChair:
		o.drawChair(f)
	case FurnComputer:
		o.drawComputer(f)
	case FurnPlant:
		o.drawPlant(f)
	case FurnCoffeeMachine:
		o.drawCoffeeMachine(f)
	case FurnWhiteboard:
		o.drawWhiteboard(f)
	case FurnWaterCooler:
		o.drawWaterCooler(f)
	}
}

func (o *Office) drawWall(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)

	if f.Height == 1 {
		// Horizontal wall
		for x := f.X; x < f.X+f.Width; x++ {
			o.Screen.SetContent(x, f.Y, '═', nil, style)
		}
	} else {
		// Vertical wall
		for y := f.Y; y < f.Y+f.Height; y++ {
			o.Screen.SetContent(f.X, y, '║', nil, style)
		}
	}
}

func (o *Office) drawDoor(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
	for x := f.X; x < f.X+f.Width; x++ {
		o.Screen.SetContent(x, f.Y, '░', nil, style)
	}
}

func (o *Office) drawDesk(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.NewRGBColor(139, 90, 43)).Background(tcell.ColorBlack) // Brown

	// Top of desk
	o.Screen.SetContent(f.X, f.Y, '┌', nil, style)
	for x := f.X + 1; x < f.X+f.Width-1; x++ {
		o.Screen.SetContent(x, f.Y, '─', nil, style)
	}
	o.Screen.SetContent(f.X+f.Width-1, f.Y, '┐', nil, style)

	// Bottom of desk
	o.Screen.SetContent(f.X, f.Y+1, '└', nil, style)
	for x := f.X + 1; x < f.X+f.Width-1; x++ {
		if x == f.X+1 || x == f.X+f.Width-2 {
			o.Screen.SetContent(x, f.Y+1, '┬', nil, style)
		} else {
			o.Screen.SetContent(x, f.Y+1, '─', nil, style)
		}
	}
	o.Screen.SetContent(f.X+f.Width-1, f.Y+1, '┘', nil, style)
}

func (o *Office) drawChair(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorDarkBlue).Background(tcell.ColorBlack)
	o.Screen.SetContent(f.X, f.Y, '╔', nil, style)
	o.Screen.SetContent(f.X+1, f.Y, '╗', nil, style)
}

func (o *Office) drawComputer(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorBlack)
	screenStyle := tcell.StyleDefault.Foreground(tcell.ColorLightBlue).Background(tcell.ColorBlack)

	o.Screen.SetContent(f.X, f.Y, '┌', nil, style)
	o.Screen.SetContent(f.X+1, f.Y, '█', nil, screenStyle)
	o.Screen.SetContent(f.X+2, f.Y, '┐', nil, style)
	o.Screen.SetContent(f.X, f.Y+1, '╘', nil, style)
	o.Screen.SetContent(f.X+1, f.Y+1, '═', nil, style)
	o.Screen.SetContent(f.X+2, f.Y+1, '╛', nil, style)
}

func (o *Office) drawPlant(f Furniture) {
	leafStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)
	potStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(139, 90, 43)).Background(tcell.ColorBlack)

	o.Screen.SetContent(f.X, f.Y, '🌿', nil, leafStyle)
	o.Screen.SetContent(f.X, f.Y+1, '┃', nil, potStyle)
}

func (o *Office) drawCoffeeMachine(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorBlack)

	o.Screen.SetContent(f.X, f.Y, '╔', nil, style)
	o.Screen.SetContent(f.X+1, f.Y, '═', nil, style)
	o.Screen.SetContent(f.X+2, f.Y, '╗', nil, style)
	o.Screen.SetContent(f.X, f.Y+1, '╚', nil, style)
	o.Screen.SetContent(f.X+1, f.Y+1, '☕', nil, style)
	o.Screen.SetContent(f.X+2, f.Y+1, '╝', nil, style)
}

func (o *Office) drawWhiteboard(f Furniture) {
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
	boardStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

	// Top border
	o.Screen.SetContent(f.X, f.Y, '┌', nil, borderStyle)
	for x := f.X + 1; x < f.X+f.Width-1; x++ {
		o.Screen.SetContent(x, f.Y, '─', nil, borderStyle)
	}
	o.Screen.SetContent(f.X+f.Width-1, f.Y, '┐', nil, borderStyle)

	// Middle with board
	o.Screen.SetContent(f.X, f.Y+1, '│', nil, borderStyle)
	for x := f.X + 1; x < f.X+f.Width-1; x++ {
		o.Screen.SetContent(x, f.Y+1, '█', nil, boardStyle)
	}
	o.Screen.SetContent(f.X+f.Width-1, f.Y+1, '│', nil, borderStyle)

	// Bottom border
	o.Screen.SetContent(f.X, f.Y+2, '└', nil, borderStyle)
	for x := f.X + 1; x < f.X+f.Width-1; x++ {
		o.Screen.SetContent(x, f.Y+2, '─', nil, borderStyle)
	}
	o.Screen.SetContent(f.X+f.Width-1, f.Y+2, '┘', nil, borderStyle)
}

func (o *Office) drawWaterCooler(f Furniture) {
	style := tcell.StyleDefault.Foreground(tcell.ColorLightBlue).Background(tcell.ColorBlack)
	baseStyle := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)

	o.Screen.SetContent(f.X, f.Y, '╔', nil, style)
	o.Screen.SetContent(f.X+1, f.Y, '╗', nil, style)
	o.Screen.SetContent(f.X, f.Y+1, '║', nil, style)
	o.Screen.SetContent(f.X+1, f.Y+1, '║', nil, style)
	o.Screen.SetContent(f.X, f.Y+2, '╚', nil, baseStyle)
	o.Screen.SetContent(f.X+1, f.Y+2, '╝', nil, baseStyle)
}

func (o *Office) drawMembers() {
	// Sort members by Y position for proper layering (simple bubble sort)
	sorted := make([]TeamMember, len(o.Members))
	copy(sorted, o.Members)

	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Y > sorted[j+1].Y {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	for i := range sorted {
		DrawCharacter(o.Screen, &sorted[i], sorted[i].Frame)
		DrawNameTag(o.Screen, int(sorted[i].X), int(sorted[i].Y), sorted[i].Name)

		// Draw speech bubble if enabled and member is working or talking
		if o.ShowBubbles && (sorted[i].State == StateWorking || sorted[i].State == StateTalking) {
			DrawSpeechBubble(o.Screen, int(sorted[i].X), int(sorted[i].Y), sorted[i].Activity, 25)
		}
	}
}

func (o *Office) drawTitle() {
	title := "🏢 Kartoza Office - Press 'q' to quit, 'b' to toggle bubbles"
	style := tcell.StyleDefault.Foreground(tcell.NewRGBColor(233, 84, 32)).Background(tcell.ColorBlack) // Kartoza Orange

	startX := (o.Width - len(title)) / 2
	if startX < 0 {
		startX = 0
	}

	col := 0
	for _, ch := range title {
		if startX+col < o.Width {
			o.Screen.SetContent(startX+col, 0, ch, nil, style)
		}
		col++
	}
}

func (o *Office) drawHelp() {
	help := fmt.Sprintf("Team: %d members", len(o.Members))
	style := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)

	for i, ch := range help {
		o.Screen.SetContent(2+i, o.Height-1, ch, nil, style)
	}
}

// SetBounds updates office dimensions on resize
func (o *Office) SetBounds(width, height int) {
	o.Width = width
	o.Height = height
	// Recreate furniture for new size
	o.Furniture = nil
	o.setupFurniture()
}

// ToggleBubbles toggles speech bubble visibility
func (o *Office) ToggleBubbles() {
	o.ShowBubbles = !o.ShowBubbles
}

// Stop stops the office simulation
func (o *Office) Stop() {
	o.Running = false
}
