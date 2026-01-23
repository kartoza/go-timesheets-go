package widgets

import (
	"image/color"
	"math"
	"math/rand"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Kartoza brand colors for fireworks
var fireworkColors = []color.NRGBA{
	{R: 0xE9, G: 0x54, B: 0x20, A: 0xFF}, // Kartoza Orange
	{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}, // Gold
	{R: 0xFF, G: 0xD7, B: 0x00, A: 0xFF}, // Yellow
	{R: 0xFF, G: 0x45, B: 0x00, A: 0xFF}, // Red-Orange
	{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}, // Blue
	{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}, // Green
	{R: 0xFF, G: 0x69, B: 0xB4, A: 0xFF}, // Pink
	{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // White
}

// Particle represents a single firework particle
type Particle struct {
	X, Y     float64
	VX, VY   float64
	Life     float64
	MaxLife  float64
	Color    color.NRGBA
	Size     float64
	circle   *canvas.Circle
}

// FireworksDisplay shows an animated fireworks celebration
type FireworksDisplay struct {
	widget.BaseWidget

	particles  []*Particle
	ticker     *time.Ticker
	stopChan   chan struct{}
	width      float64
	height     float64
	container  *fyne.Container
	running    bool
	message    string
	onComplete func()
	popup      *widget.PopUp
}

// NewFireworksDisplay creates a new fireworks celebration display
func NewFireworksDisplay(message string, onComplete func()) *FireworksDisplay {
	f := &FireworksDisplay{
		particles:  make([]*Particle, 0, 200),
		stopChan:   make(chan struct{}),
		message:    message,
		onComplete: onComplete,
	}
	f.ExtendBaseWidget(f)
	return f
}

// Start begins the fireworks animation
func (f *FireworksDisplay) Start(duration time.Duration) {
	if f.running {
		return
	}
	f.running = true

	// Create initial burst
	f.spawnFirework()

	// Animation ticker
	f.ticker = time.NewTicker(33 * time.Millisecond) // ~30 FPS
	go func() {
		spawnTicker := time.NewTicker(400 * time.Millisecond)
		endTimer := time.NewTimer(duration)

		defer spawnTicker.Stop()
		defer endTimer.Stop()

		for {
			select {
			case <-f.ticker.C:
				fyne.Do(func() {
					f.update()
					f.Refresh()
				})
			case <-spawnTicker.C:
				fyne.Do(func() {
					f.spawnFirework()
				})
			case <-endTimer.C:
				f.Stop()
				if f.onComplete != nil {
					fyne.Do(func() {
						f.onComplete()
					})
				}
				return
			case <-f.stopChan:
				return
			}
		}
	}()
}

// Stop stops the fireworks animation
func (f *FireworksDisplay) Stop() {
	if !f.running {
		return
	}
	f.running = false
	if f.ticker != nil {
		f.ticker.Stop()
		f.ticker = nil
	}
	select {
	case f.stopChan <- struct{}{}:
	default:
	}
}

// spawnFirework creates a new firework explosion
func (f *FireworksDisplay) spawnFirework() {
	if f.width <= 0 || f.height <= 0 {
		return
	}

	// Random explosion center
	centerX := rand.Float64()*f.width*0.6 + f.width*0.2
	centerY := rand.Float64()*f.height*0.4 + f.height*0.1

	// Random color for this burst
	baseColor := fireworkColors[rand.Intn(len(fireworkColors))]

	// Create particles in a circular burst
	numParticles := 20 + rand.Intn(30)
	for i := 0; i < numParticles; i++ {
		angle := rand.Float64() * 2 * math.Pi
		speed := 2 + rand.Float64()*4
		life := 1.0 + rand.Float64()*0.5

		p := &Particle{
			X:       centerX,
			Y:       centerY,
			VX:      math.Cos(angle) * speed,
			VY:      math.Sin(angle) * speed,
			Life:    life,
			MaxLife: life,
			Color:   baseColor,
			Size:    3 + rand.Float64()*4,
		}

		// Vary the color slightly
		if rand.Float32() < 0.3 {
			p.Color = fireworkColors[rand.Intn(len(fireworkColors))]
		}

		f.particles = append(f.particles, p)
	}
}

// update updates all particles
func (f *FireworksDisplay) update() {
	dt := 0.1 // Time step
	gravity := 0.15

	// Update existing particles
	aliveParticles := make([]*Particle, 0, len(f.particles))
	for _, p := range f.particles {
		p.Life -= dt * 0.5
		if p.Life <= 0 {
			continue
		}

		// Apply physics
		p.VY += gravity // Gravity
		p.X += p.VX
		p.Y += p.VY

		// Drag
		p.VX *= 0.98
		p.VY *= 0.98

		// Keep in bounds
		if p.X >= 0 && p.X <= f.width && p.Y >= 0 && p.Y <= f.height {
			aliveParticles = append(aliveParticles, p)
		}
	}
	f.particles = aliveParticles
}

// CreateRenderer creates the renderer for the fireworks display
func (f *FireworksDisplay) CreateRenderer() fyne.WidgetRenderer {
	// Background
	bg := canvas.NewRectangle(color.NRGBA{R: 0x0A, G: 0x0A, B: 0x1A, A: 0xE0})

	// Celebration message
	msgText := canvas.NewText(f.message, color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	msgText.TextSize = 28
	msgText.TextStyle = fyne.TextStyle{Bold: true}
	msgText.Alignment = fyne.TextAlignCenter

	// Subtitle
	subText := canvas.NewText("🎉 Click anywhere to close 🎉", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	subText.TextSize = 14
	subText.Alignment = fyne.TextAlignCenter

	f.container = container.NewWithoutLayout()

	return &fireworksRenderer{
		display:   f,
		bg:        bg,
		msgText:   msgText,
		subText:   subText,
		container: f.container,
	}
}

type fireworksRenderer struct {
	display   *FireworksDisplay
	bg        *canvas.Rectangle
	msgText   *canvas.Text
	subText   *canvas.Text
	container *fyne.Container
}

func (r *fireworksRenderer) Layout(size fyne.Size) {
	r.display.width = float64(size.Width)
	r.display.height = float64(size.Height)

	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	// Center message
	r.msgText.Resize(fyne.NewSize(size.Width, 40))
	r.msgText.Move(fyne.NewPos(0, size.Height/2-40))

	r.subText.Resize(fyne.NewSize(size.Width, 20))
	r.subText.Move(fyne.NewPos(0, size.Height/2+10))

	r.container.Resize(size)
	r.container.Move(fyne.NewPos(0, 0))
}

func (r *fireworksRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *fireworksRenderer) Refresh() {
	// Clear old particle circles
	r.container.RemoveAll()

	// Draw particles
	for _, p := range r.display.particles {
		alpha := uint8(255 * (p.Life / p.MaxLife))
		c := p.Color
		c.A = alpha

		circle := canvas.NewCircle(c)
		size := float32(p.Size * (p.Life / p.MaxLife))
		circle.Resize(fyne.NewSize(size, size))
		circle.Move(fyne.NewPos(float32(p.X)-size/2, float32(p.Y)-size/2))

		r.container.Add(circle)
	}

	r.container.Refresh()
}

func (r *fireworksRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.container, r.msgText, r.subText}
}

func (r *fireworksRenderer) Destroy() {
	r.display.Stop()
}

// ShowFireworksCelebration shows a fireworks celebration dialog
func ShowFireworksCelebration(window fyne.Window, message string, duration time.Duration) {
	fireworks := NewFireworksDisplay(message, nil)

	// Create a popup
	popup := widget.NewModalPopUp(fireworks, window.Canvas())
	popup.Resize(fyne.NewSize(500, 400))

	// Set callback to close popup
	fireworks.onComplete = func() {
		popup.Hide()
	}

	// Store popup reference so Tapped can close it
	fireworks.popup = popup

	popup.Show()
	fireworks.Start(duration)
}

// Tappable interface - close fireworks on tap
func (f *FireworksDisplay) Tapped(*fyne.PointEvent) {
	f.Stop()
	if f.onComplete != nil {
		f.onComplete()
	}
}

func (f *FireworksDisplay) TappedSecondary(*fyne.PointEvent) {}
