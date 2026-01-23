package widgets

import (
	"image/color"
	"math"
	"math/rand"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/office"
)

// IsometricOffice is an animated isometric office scene widget
type IsometricOffice struct {
	widget.BaseWidget

	width     float32
	height    float32
	workers   []*guiWorker
	furniture []guiFurniture
	animFrame int
	ticker    *time.Ticker
	stopChan  chan bool
	loading   bool
	loadError string
}

type guiWorker struct {
	x, y       float32
	targetX    float32
	targetY    float32
	deskX      float32 // Original desk position
	deskY      float32
	color      color.Color
	skinColor  color.Color
	state      int // 0=working, 1=walking, 2=idle
	frame      int
	speed      float32
	name       string
	role       string
	activity   string
}

type guiFurniture struct {
	ftype int // 0=desk, 1=chair, 2=computer, 3=plant, 4=coffee, 5=whiteboard, 6=watercooler
	x, y  float32
}

const (
	stateWorking = iota
	stateWalking
	stateIdle
)

// Worker colors (from TUI)
var workerColors = []color.Color{
	color.NRGBA{R: 0xFF, G: 0x57, B: 0x22, A: 0xFF}, // Deep Orange
	color.NRGBA{R: 0x21, G: 0x96, B: 0xF3, A: 0xFF}, // Blue
	color.NRGBA{R: 0x4C, G: 0xAF, B: 0x50, A: 0xFF}, // Green
	color.NRGBA{R: 0x9C, G: 0x27, B: 0xB0, A: 0xFF}, // Purple
	color.NRGBA{R: 0xE9, G: 0x1E, B: 0x63, A: 0xFF}, // Pink
	color.NRGBA{R: 0x00, G: 0xBC, B: 0xD4, A: 0xFF}, // Cyan
	color.NRGBA{R: 0xFF, G: 0xEB, B: 0x3B, A: 0xFF}, // Yellow
	color.NRGBA{R: 0x79, G: 0x55, B: 0x48, A: 0xFF}, // Brown
	color.NRGBA{R: 0x60, G: 0x7D, B: 0x8B, A: 0xFF}, // Blue Grey
	color.NRGBA{R: 0xF4, G: 0x43, B: 0x36, A: 0xFF}, // Red
	color.NRGBA{R: 0x3F, G: 0x51, B: 0xB5, A: 0xFF}, // Indigo
	color.NRGBA{R: 0x00, G: 0x96, B: 0x88, A: 0xFF}, // Teal
	color.NRGBA{R: 0xFF, G: 0x98, B: 0x00, A: 0xFF}, // Orange
	color.NRGBA{R: 0x67, G: 0x3A, B: 0xB7, A: 0xFF}, // Deep Purple
	color.NRGBA{R: 0x8B, G: 0xC3, B: 0x4A, A: 0xFF}, // Light Green
}

// NewIsometricOffice creates a new isometric office scene
func NewIsometricOffice() *IsometricOffice {
	o := &IsometricOffice{
		width:    800,
		height:   600,
		stopChan: make(chan bool),
		loading:  true,
	}
	o.ExtendBaseWidget(o)

	// Start fetching team members asynchronously
	go o.fetchTeamMembers()

	return o
}

func (o *IsometricOffice) fetchTeamMembers() {
	members, err := office.FetchTeamMembers()
	if err != nil {
		o.loadError = err.Error()
		o.loading = false
		// Use hardcoded fallback
		o.initSceneWithMembers(nil)
		return
	}

	o.initSceneWithMembers(members)
	o.loading = false
}

func (o *IsometricOffice) initSceneWithMembers(members []office.TeamMember) {
	skinColor := color.NRGBA{R: 0xFF, G: 0xDA, B: 0xB9, A: 0xFF} // Peach

	// If no members fetched, use fallback
	if len(members) == 0 {
		members = []office.TeamMember{
			{Name: "Tim Sutton", Role: "Director", Activity: "Reviewing PRs"},
			{Name: "Gavin Fleming", Role: "Director", Activity: "Client meeting"},
			{Name: "Marike Kruger", Role: "General Manager", Activity: "Planning sprint"},
			{Name: "Irwan Fathurrahman", Role: "Lead Developer", Activity: "Coding"},
			{Name: "Zakki Muzakki", Role: "Software Developer", Activity: "Bug fixing"},
			{Name: "Seabilwe Tilodi", Role: "Head of Training", Activity: "Creating course"},
			{Name: "Dimas Tri Ciputra", Role: "Senior Developer", Activity: "Code review"},
			{Name: "Victoria Nyaga", Role: "GIS Developer", Activity: "QGIS plugin"},
			{Name: "Jeremy Prior", Role: "GIS Specialist", Activity: "Map styling"},
			{Name: "Leon Greyling", Role: "DevOps Manager", Activity: "Server maintenance"},
			{Name: "Elia Volschenk", Role: "Scrum Master", Activity: "Stand-up meeting"},
			{Name: "Danang Tri Massandy", Role: "Senior Developer", Activity: "API development"},
		}
	}

	// Clear existing data
	o.workers = nil
	o.furniture = nil

	// We'll set up workers and furniture when we know the actual size
	// For now, store the member data and set up in the first render
	for i, member := range members {
		// Get first name for display
		firstName := member.Name
		if idx := strings.Index(member.Name, " "); idx > 0 {
			firstName = member.Name[:idx]
		}

		activity := member.Activity
		if activity == "" {
			activity = getRandomActivity(member.Role)
		}

		o.workers = append(o.workers, &guiWorker{
			color:     workerColors[i%len(workerColors)],
			skinColor: skinColor,
			state:     stateWorking,
			speed:     1.0 + rand.Float32()*0.5,
			name:      firstName,
			role:      member.Role,
			activity:  activity,
		})
	}
}

func (o *IsometricOffice) setupLayout() {
	if len(o.workers) == 0 {
		return
	}

	// Calculate desk layout based on available space
	margin := float32(40)
	deskWidth := float32(60)
	deskHeight := float32(50)
	deskSpacingX := float32(100)
	deskSpacingY := float32(80)

	availableWidth := o.width - 2*margin
	_ = o.height - 2*margin - 60 // Reserve space for header/footer (used for bounds checking)

	desksPerRow := int(availableWidth / deskSpacingX)
	if desksPerRow < 1 {
		desksPerRow = 1
	}
	if desksPerRow > 6 {
		desksPerRow = 6 // Max 6 desks per row
	}

	// Clear furniture
	o.furniture = nil

	// Position workers at desks
	for i, w := range o.workers {
		row := i / desksPerRow
		col := i % desksPerRow

		// Center the row
		rowWidth := float32(min(desksPerRow, len(o.workers)-row*desksPerRow)) * deskSpacingX
		startX := margin + (availableWidth-rowWidth)/2

		x := startX + float32(col)*deskSpacingX + deskWidth/2
		y := margin + 60 + float32(row)*deskSpacingY + deskHeight/2

		// Clamp to screen bounds
		if y > o.height-100 {
			y = o.height - 100
		}

		w.deskX = x
		w.deskY = y + 25
		w.x = x
		w.y = y + 25
		w.targetX = x
		w.targetY = y + 25

		// Add desk
		o.furniture = append(o.furniture, guiFurniture{ftype: 0, x: x, y: y})
		// Add computer on desk
		o.furniture = append(o.furniture, guiFurniture{ftype: 2, x: x, y: y - 15})
		// Add chair
		o.furniture = append(o.furniture, guiFurniture{ftype: 1, x: x, y: y + 30})
	}

	// Add decorative furniture along edges
	// Plants in corners
	o.furniture = append(o.furniture, guiFurniture{ftype: 3, x: 25, y: 70})
	o.furniture = append(o.furniture, guiFurniture{ftype: 3, x: o.width - 25, y: 70})
	o.furniture = append(o.furniture, guiFurniture{ftype: 3, x: 25, y: o.height - 50})
	o.furniture = append(o.furniture, guiFurniture{ftype: 3, x: o.width - 25, y: o.height - 50})

	// Coffee machine and water cooler
	o.furniture = append(o.furniture, guiFurniture{ftype: 4, x: 30, y: o.height / 2})
	o.furniture = append(o.furniture, guiFurniture{ftype: 6, x: o.width - 30, y: o.height / 2})

	// Whiteboard at top
	o.furniture = append(o.furniture, guiFurniture{ftype: 5, x: o.width / 2, y: 55})
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
		},
		"DevOps": {
			"Server maintenance",
			"CI/CD pipeline",
			"Monitoring alerts",
			"Deployment",
		},
		"GIS": {
			"Map styling",
			"Data analysis",
			"QGIS plugin",
			"Spatial queries",
		},
		"Manager": {
			"Sprint planning",
			"Stand-up meeting",
			"Resource allocation",
			"Status update",
		},
		"default": {
			"Deep work",
			"Email",
			"Meeting",
			"Research",
		},
	}

	roleLower := strings.ToLower(role)
	var category string
	if strings.Contains(roleLower, "director") {
		category = "Director"
	} else if strings.Contains(roleLower, "developer") || strings.Contains(roleLower, "software") {
		category = "Developer"
	} else if strings.Contains(roleLower, "devops") || strings.Contains(roleLower, "ops") {
		category = "DevOps"
	} else if strings.Contains(roleLower, "gis") || strings.Contains(roleLower, "qgis") {
		category = "GIS"
	} else if strings.Contains(roleLower, "manager") || strings.Contains(roleLower, "scrum") {
		category = "Manager"
	} else {
		category = "default"
	}

	acts := activities[category]
	return acts[rand.Intn(len(acts))]
}

// StartAnimation starts the animation loop
func (o *IsometricOffice) StartAnimation() {
	if o.ticker != nil {
		return
	}
	o.ticker = time.NewTicker(100 * time.Millisecond)
	go func() {
		for {
			select {
			case <-o.ticker.C:
				o.update()
				fyne.Do(func() {
					o.Refresh()
				})
			case <-o.stopChan:
				return
			}
		}
	}()
}

// StopAnimation stops the animation loop
func (o *IsometricOffice) StopAnimation() {
	if o.ticker != nil {
		o.ticker.Stop()
		o.ticker = nil
	}
	select {
	case o.stopChan <- true:
	default:
	}
}

func (o *IsometricOffice) update() {
	o.animFrame++

	for _, w := range o.workers {
		w.frame++

		switch w.state {
		case stateWorking:
			// Occasionally stand up and walk around
			if rand.Float32() < 0.003 {
				w.state = stateWalking
				// Pick random destination within bounds
				w.targetX = 50 + rand.Float32()*(o.width-100)
				w.targetY = 80 + rand.Float32()*(o.height-160)
			}
		case stateWalking:
			dx := w.targetX - w.x
			dy := w.targetY - w.y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

			if dist < 5 {
				w.state = stateIdle
			} else {
				w.x += (dx / dist) * w.speed * 2
				w.y += (dy / dist) * w.speed * 2
			}
		case stateIdle:
			if rand.Float32() < 0.02 {
				// Go back to desk
				w.state = stateWalking
				w.targetX = w.deskX
				w.targetY = w.deskY
			}
		}

		// Return to working state when back at desk
		if w.state == stateWalking {
			dx := w.targetX - w.x
			dy := w.targetY - w.y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if dist < 5 && w.targetX == w.deskX && w.targetY == w.deskY {
				w.state = stateWorking
			}
		}

		// Keep within bounds
		if w.x < 20 {
			w.x = 20
		}
		if w.x > o.width-20 {
			w.x = o.width - 20
		}
		if w.y < 60 {
			w.y = 60
		}
		if w.y > o.height-40 {
			w.y = o.height - 40
		}
	}
}

// CreateRenderer creates the renderer for the office scene
func (o *IsometricOffice) CreateRenderer() fyne.WidgetRenderer {
	return &isometricOfficeRenderer{office: o, lastWidth: -1, lastHeight: -1}
}

type isometricOfficeRenderer struct {
	office     *IsometricOffice
	lastWidth  float32
	lastHeight float32
}

func (r *isometricOfficeRenderer) Layout(size fyne.Size) {
	r.office.width = size.Width
	r.office.height = size.Height

	// Recalculate layout if size changed significantly
	if math.Abs(float64(size.Width-r.lastWidth)) > 10 || math.Abs(float64(size.Height-r.lastHeight)) > 10 {
		r.lastWidth = size.Width
		r.lastHeight = size.Height
		r.office.setupLayout()
	}
}

func (r *isometricOfficeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *isometricOfficeRenderer) Refresh() {
}

func (r *isometricOfficeRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	o := r.office

	// Background - office floor
	floor := canvas.NewRectangle(color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF})
	floor.Resize(fyne.NewSize(o.width, o.height))
	objects = append(objects, floor)

	// Draw floor grid pattern
	gridColor := color.NRGBA{R: 0x35, G: 0x35, B: 0x35, A: 0xFF}
	gridSpacing := float32(40)
	for x := float32(0); x < o.width; x += gridSpacing {
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(x, 0)
		line.Position2 = fyne.NewPos(x, o.height)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}
	for y := float32(0); y < o.height; y += gridSpacing {
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(0, y)
		line.Position2 = fyne.NewPos(o.width, y)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}

	// Draw walls
	wallColor := color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}
	wallThickness := float32(5)

	topWall := canvas.NewRectangle(wallColor)
	topWall.Move(fyne.NewPos(0, 0))
	topWall.Resize(fyne.NewSize(o.width, wallThickness))
	objects = append(objects, topWall)

	leftWall := canvas.NewRectangle(wallColor)
	leftWall.Move(fyne.NewPos(0, 0))
	leftWall.Resize(fyne.NewSize(wallThickness, o.height))
	objects = append(objects, leftWall)

	rightWall := canvas.NewRectangle(wallColor)
	rightWall.Move(fyne.NewPos(o.width-wallThickness, 0))
	rightWall.Resize(fyne.NewSize(wallThickness, o.height))
	objects = append(objects, rightWall)

	bottomWall := canvas.NewRectangle(wallColor)
	bottomWall.Move(fyne.NewPos(0, o.height-wallThickness))
	bottomWall.Resize(fyne.NewSize(o.width, wallThickness))
	objects = append(objects, bottomWall)

	// Draw door
	doorColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	door := canvas.NewRectangle(doorColor)
	door.Move(fyne.NewPos(o.width/2-30, o.height-wallThickness))
	door.Resize(fyne.NewSize(60, wallThickness))
	objects = append(objects, door)

	// Loading indicator
	if o.loading {
		loadingText := canvas.NewText("Loading team members from kartoza.com...", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
		loadingText.TextSize = 16
		loadingText.Move(fyne.NewPos(o.width/2-150, o.height/2))
		objects = append(objects, loadingText)
		return objects
	}

	// Draw furniture
	for _, f := range o.furniture {
		objects = append(objects, r.drawFurniture(f)...)
	}

	// Draw workers (sorted by Y for layering)
	sortedWorkers := make([]*guiWorker, len(o.workers))
	copy(sortedWorkers, o.workers)
	for i := 0; i < len(sortedWorkers)-1; i++ {
		for j := 0; j < len(sortedWorkers)-i-1; j++ {
			if sortedWorkers[j].y > sortedWorkers[j+1].y {
				sortedWorkers[j], sortedWorkers[j+1] = sortedWorkers[j+1], sortedWorkers[j]
			}
		}
	}

	for _, w := range sortedWorkers {
		objects = append(objects, r.drawWorker(w)...)
	}

	// Title bar at top
	titleBg := canvas.NewRectangle(color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xE0})
	titleBg.Move(fyne.NewPos(0, 0))
	titleBg.Resize(fyne.NewSize(o.width, 35))
	objects = append(objects, titleBg)

	title := canvas.NewText("Kartoza Virtual Office", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Move(fyne.NewPos(15, 8))
	objects = append(objects, title)

	// Team count
	teamCount := canvas.NewText(
		strings.Replace("Team: X members", "X", string(rune('0'+len(o.workers)/10))+string(rune('0'+len(o.workers)%10)), 1),
		color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF},
	)
	if len(o.workers) < 10 {
		teamCount.Text = "Team: " + string(rune('0'+len(o.workers))) + " members"
	} else {
		teamCount.Text = "Team: " + string(rune('0'+len(o.workers)/10)) + string(rune('0'+len(o.workers)%10)) + " members"
	}
	teamCount.TextSize = 12
	teamCount.Move(fyne.NewPos(o.width-120, 12))
	objects = append(objects, teamCount)

	return objects
}

func (r *isometricOfficeRenderer) drawFurniture(f guiFurniture) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}

	switch f.ftype {
	case 0: // Desk
		deskColor := color.NRGBA{R: 0x8B, G: 0x5A, B: 0x2B, A: 0xFF}
		deskTop := canvas.NewRectangle(deskColor)
		deskTop.Move(fyne.NewPos(f.x-25, f.y))
		deskTop.Resize(fyne.NewSize(50, 20))
		objects = append(objects, deskTop)

		legColor := color.NRGBA{R: 0x6B, G: 0x4A, B: 0x1B, A: 0xFF}
		leg1 := canvas.NewRectangle(legColor)
		leg1.Move(fyne.NewPos(f.x-23, f.y+18))
		leg1.Resize(fyne.NewSize(4, 12))
		objects = append(objects, leg1)

		leg2 := canvas.NewRectangle(legColor)
		leg2.Move(fyne.NewPos(f.x+19, f.y+18))
		leg2.Resize(fyne.NewSize(4, 12))
		objects = append(objects, leg2)

	case 1: // Chair
		chairColor := color.NRGBA{R: 0x1A, G: 0x23, B: 0x7E, A: 0xFF}
		seat := canvas.NewRectangle(chairColor)
		seat.Move(fyne.NewPos(f.x-8, f.y))
		seat.Resize(fyne.NewSize(16, 10))
		objects = append(objects, seat)

		back := canvas.NewRectangle(chairColor)
		back.Move(fyne.NewPos(f.x-6, f.y-12))
		back.Resize(fyne.NewSize(12, 14))
		objects = append(objects, back)

	case 2: // Computer monitor
		monitorColor := color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xFF}
		monitor := canvas.NewRectangle(monitorColor)
		monitor.Move(fyne.NewPos(f.x-10, f.y))
		monitor.Resize(fyne.NewSize(20, 15))
		objects = append(objects, monitor)

		screenColor := color.NRGBA{R: 0x64, G: 0xB5, B: 0xF6, A: 0xFF}
		if r.office.animFrame%20 < 10 {
			screenColor = color.NRGBA{R: 0x42, G: 0xA5, B: 0xF5, A: 0xFF}
		}
		screen := canvas.NewRectangle(screenColor)
		screen.Move(fyne.NewPos(f.x-8, f.y+2))
		screen.Resize(fyne.NewSize(16, 11))
		objects = append(objects, screen)

		standColor := color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xFF}
		stand := canvas.NewRectangle(standColor)
		stand.Move(fyne.NewPos(f.x-3, f.y+15))
		stand.Resize(fyne.NewSize(6, 5))
		objects = append(objects, stand)

	case 3: // Plant
		potColor := color.NRGBA{R: 0x8B, G: 0x5A, B: 0x2B, A: 0xFF}
		pot := canvas.NewRectangle(potColor)
		pot.Move(fyne.NewPos(f.x-8, f.y+10))
		pot.Resize(fyne.NewSize(16, 12))
		objects = append(objects, pot)

		leafColor := color.NRGBA{R: 0x4C, G: 0xAF, B: 0x50, A: 0xFF}
		leaf1 := canvas.NewCircle(leafColor)
		leaf1.Move(fyne.NewPos(f.x-10, f.y-5))
		leaf1.Resize(fyne.NewSize(12, 12))
		objects = append(objects, leaf1)

		leaf2 := canvas.NewCircle(leafColor)
		leaf2.Move(fyne.NewPos(f.x-2, f.y-8))
		leaf2.Resize(fyne.NewSize(14, 14))
		objects = append(objects, leaf2)

		leaf3 := canvas.NewCircle(leafColor)
		leaf3.Move(fyne.NewPos(f.x+2, f.y-3))
		leaf3.Resize(fyne.NewSize(10, 10))
		objects = append(objects, leaf3)

	case 4: // Coffee machine
		machineColor := color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xFF}
		machine := canvas.NewRectangle(machineColor)
		machine.Move(fyne.NewPos(f.x-10, f.y))
		machine.Resize(fyne.NewSize(20, 30))
		objects = append(objects, machine)

		dispenserColor := color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xFF}
		dispenser := canvas.NewRectangle(dispenserColor)
		dispenser.Move(fyne.NewPos(f.x-6, f.y+20))
		dispenser.Resize(fyne.NewSize(12, 8))
		objects = append(objects, dispenser)

		coffeeColor := color.NRGBA{R: 0x6F, G: 0x42, B: 0x35, A: 0xFF}
		coffee := canvas.NewCircle(coffeeColor)
		coffee.Move(fyne.NewPos(f.x-5, f.y+5))
		coffee.Resize(fyne.NewSize(10, 10))
		objects = append(objects, coffee)

	case 5: // Whiteboard
		boardColor := color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF}
		board := canvas.NewRectangle(boardColor)
		board.Move(fyne.NewPos(f.x-50, f.y))
		board.Resize(fyne.NewSize(100, 35))
		objects = append(objects, board)

		frameColor := color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xFF}
		frameTop := canvas.NewRectangle(frameColor)
		frameTop.Move(fyne.NewPos(f.x-52, f.y-2))
		frameTop.Resize(fyne.NewSize(104, 4))
		objects = append(objects, frameTop)

		frameBottom := canvas.NewRectangle(frameColor)
		frameBottom.Move(fyne.NewPos(f.x-52, f.y+33))
		frameBottom.Resize(fyne.NewSize(104, 4))
		objects = append(objects, frameBottom)

		lineColor := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
		for i := 0; i < 3; i++ {
			line := canvas.NewLine(lineColor)
			line.Position1 = fyne.NewPos(f.x-45, f.y+8+float32(i)*8)
			line.Position2 = fyne.NewPos(f.x+45-float32(i)*15, f.y+8+float32(i)*8)
			line.StrokeWidth = 2
			objects = append(objects, line)
		}

	case 6: // Water cooler
		coolerColor := color.NRGBA{R: 0x64, G: 0xB5, B: 0xF6, A: 0xFF}
		water := canvas.NewRectangle(coolerColor)
		water.Move(fyne.NewPos(f.x-8, f.y))
		water.Resize(fyne.NewSize(16, 20))
		objects = append(objects, water)

		baseColor := color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xFF}
		base := canvas.NewRectangle(baseColor)
		base.Move(fyne.NewPos(f.x-10, f.y+18))
		base.Resize(fyne.NewSize(20, 15))
		objects = append(objects, base)
	}

	return objects
}

func (r *isometricOfficeRenderer) drawWorker(w *guiWorker) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}

	// Shadow
	shadowColor := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x40}
	shadow := canvas.NewCircle(shadowColor)
	shadow.Move(fyne.NewPos(w.x-8, w.y+15))
	shadow.Resize(fyne.NewSize(16, 6))
	objects = append(objects, shadow)

	// Body
	body := canvas.NewRectangle(w.color)
	body.Move(fyne.NewPos(w.x-6, w.y))
	body.Resize(fyne.NewSize(12, 18))
	objects = append(objects, body)

	// Head
	head := canvas.NewCircle(w.skinColor)
	head.Move(fyne.NewPos(w.x-7, w.y-14))
	head.Resize(fyne.NewSize(14, 14))
	objects = append(objects, head)

	// Arms
	if w.state == stateWorking {
		armOffset := float32(0)
		if w.frame%10 < 5 {
			armOffset = 2
		}
		leftArm := canvas.NewRectangle(w.color)
		leftArm.Move(fyne.NewPos(w.x-10, w.y+2+armOffset))
		leftArm.Resize(fyne.NewSize(4, 8))
		objects = append(objects, leftArm)

		rightArm := canvas.NewRectangle(w.color)
		rightArm.Move(fyne.NewPos(w.x+6, w.y+2-armOffset))
		rightArm.Resize(fyne.NewSize(4, 8))
		objects = append(objects, rightArm)
	} else {
		armSwing := float32(math.Sin(float64(w.frame)/3)) * 3
		leftArm := canvas.NewRectangle(w.color)
		leftArm.Move(fyne.NewPos(w.x-10, w.y+2+armSwing))
		leftArm.Resize(fyne.NewSize(4, 10))
		objects = append(objects, leftArm)

		rightArm := canvas.NewRectangle(w.color)
		rightArm.Move(fyne.NewPos(w.x+6, w.y+2-armSwing))
		rightArm.Resize(fyne.NewSize(4, 10))
		objects = append(objects, rightArm)
	}

	// Legs
	legSwing := float32(0)
	if w.state == stateWalking {
		legSwing = float32(math.Sin(float64(w.frame)/2)) * 4
	}
	leftLeg := canvas.NewRectangle(color.NRGBA{R: 0x33, G: 0x33, B: 0x55, A: 0xFF})
	leftLeg.Move(fyne.NewPos(w.x-5, w.y+16+legSwing))
	leftLeg.Resize(fyne.NewSize(4, 10))
	objects = append(objects, leftLeg)

	rightLeg := canvas.NewRectangle(color.NRGBA{R: 0x33, G: 0x33, B: 0x55, A: 0xFF})
	rightLeg.Move(fyne.NewPos(w.x+1, w.y+16-legSwing))
	rightLeg.Resize(fyne.NewSize(4, 10))
	objects = append(objects, rightLeg)

	// Name tag
	nameTag := canvas.NewText(w.name, color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	nameTag.TextSize = 10
	nameTag.Move(fyne.NewPos(w.x-15, w.y+28))
	objects = append(objects, nameTag)

	// Activity bubble when working
	if w.state == stateWorking && w.activity != "" {
		// Speech bubble background
		bubbleWidth := float32(len(w.activity)*6 + 10)
		bubble := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xDD})
		bubble.Move(fyne.NewPos(w.x+10, w.y-25))
		bubble.Resize(fyne.NewSize(bubbleWidth, 18))
		objects = append(objects, bubble)

		// Activity text
		activityText := canvas.NewText(w.activity, color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF})
		activityText.TextSize = 9
		activityText.Move(fyne.NewPos(w.x+15, w.y-23))
		objects = append(objects, activityText)
	}

	return objects
}

func (r *isometricOfficeRenderer) Destroy() {
	r.office.StopAnimation()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
