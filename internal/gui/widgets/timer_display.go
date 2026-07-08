package widgets

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TimerDisplay is a custom widget showing elapsed time in LCD-style format.
//
// Three visual states:
//   - IsActive=true: running timer (green, blinking colon, live time)
//   - IsPaused=true (IsActive=false): paused timer (amber, frozen time,
//     "PAUSED — <project>" header)
//   - both false: idle (grey, "No active timer")
type TimerDisplay struct {
	widget.BaseWidget

	Hours       int
	Minutes     int
	IsActive    bool
	IsPaused    bool
	ProjectName string
	TaskName    string
	BlinkOn     bool

	timeLabel    *canvas.Text
	projectLabel *canvas.Text
	taskLabel    *canvas.Text
	statusDot    *canvas.Circle
	ticker       *time.Ticker
	stopChan     chan struct{}
}

// NewTimerDisplay creates a new timer display widget
func NewTimerDisplay() *TimerDisplay {
	t := &TimerDisplay{
		BlinkOn:  true,
		stopChan: make(chan struct{}),
	}
	t.ExtendBaseWidget(t)
	return t
}

// SetTime updates the displayed time
func (t *TimerDisplay) SetTime(hours, minutes int) {
	t.Hours = hours
	t.Minutes = minutes
	t.Refresh()
}

// SetActive sets whether the timer is running. Clears any paused state.
func (t *TimerDisplay) SetActive(active bool, projectName, taskName string) {
	t.IsActive = active
	if active {
		t.IsPaused = false
	}
	t.ProjectName = projectName
	t.TaskName = taskName
	t.Refresh()
}

// SetPaused shows the timer in paused state: frozen time, amber colour, and
// a "PAUSED — <project>" header so the user remembers what they were doing.
// Pass projectName/taskName to identify the paused work.
func (t *TimerDisplay) SetPaused(paused bool, projectName, taskName string) {
	t.IsPaused = paused
	if paused {
		t.IsActive = false
	}
	t.ProjectName = projectName
	t.TaskName = taskName
	t.Refresh()
}

// StartBlinking starts the blink animation for active timers
func (t *TimerDisplay) StartBlinking() {
	if t.ticker != nil {
		return
	}
	t.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.BlinkOn = !t.BlinkOn
				// Use fyne.Do for thread-safe UI update
				fyne.Do(func() {
					t.Refresh()
				})
			case <-t.stopChan:
				return
			}
		}
	}()
}

// StopBlinking stops the blink animation
func (t *TimerDisplay) StopBlinking() {
	if t.ticker != nil {
		t.ticker.Stop()
		t.ticker = nil
	}
	select {
	case t.stopChan <- struct{}{}:
	default:
	}
	t.BlinkOn = true
	t.Refresh()
}

// CreateRenderer creates the renderer for the timer display
func (t *TimerDisplay) CreateRenderer() fyne.WidgetRenderer {
	// Time display (large LCD-style)
	t.timeLabel = canvas.NewText("00:00", color.White)
	t.timeLabel.TextSize = 48
	t.timeLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.timeLabel.Alignment = fyne.TextAlignCenter

	// Project name
	t.projectLabel = canvas.NewText("No active timer", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	t.projectLabel.TextSize = 14
	t.projectLabel.Alignment = fyne.TextAlignCenter

	// Task name
	t.taskLabel = canvas.NewText("", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	t.taskLabel.TextSize = 12
	t.taskLabel.Alignment = fyne.TextAlignCenter

	// Status indicator dot
	t.statusDot = canvas.NewCircle(color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})

	return &timerDisplayRenderer{
		display:      t,
		timeLabel:    t.timeLabel,
		projectLabel: t.projectLabel,
		taskLabel:    t.taskLabel,
		statusDot:    t.statusDot,
	}
}

type timerDisplayRenderer struct {
	display      *TimerDisplay
	timeLabel    *canvas.Text
	projectLabel *canvas.Text
	taskLabel    *canvas.Text
	statusDot    *canvas.Circle
}

func (r *timerDisplayRenderer) Layout(size fyne.Size) {
	// Status dot in top-right
	dotSize := float32(12)
	r.statusDot.Resize(fyne.NewSize(dotSize, dotSize))
	r.statusDot.Move(fyne.NewPos(size.Width-dotSize-10, 10))

	// Time label centered at top
	r.timeLabel.Resize(fyne.NewSize(size.Width, 60))
	r.timeLabel.Move(fyne.NewPos(0, 20))

	// Project label below time
	r.projectLabel.Resize(fyne.NewSize(size.Width, 20))
	r.projectLabel.Move(fyne.NewPos(0, 85))

	// Task label below project
	r.taskLabel.Resize(fyne.NewSize(size.Width, 18))
	r.taskLabel.Move(fyne.NewPos(0, 108))
}

func (r *timerDisplayRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 140)
}

func (r *timerDisplayRenderer) Refresh() {
	d := r.display

	// Update time display with blinking colon (only blinks when actively running)
	colon := ":"
	if d.IsActive && !d.BlinkOn {
		colon = " "
	}
	r.timeLabel.Text = fmt.Sprintf("%02d%s%02d", d.Hours, colon, d.Minutes)

	// Three-state colour and label scheme.
	switch {
	case d.IsActive:
		green := color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}
		r.timeLabel.Color = green
		r.statusDot.FillColor = green
		r.projectLabel.Text = d.ProjectName
		r.projectLabel.Color = color.White
		r.taskLabel.Text = d.TaskName
	case d.IsPaused:
		amber := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
		r.timeLabel.Color = amber
		r.statusDot.FillColor = amber
		r.projectLabel.Text = "PAUSED — " + d.ProjectName
		r.projectLabel.Color = amber
		r.taskLabel.Text = d.TaskName
	default:
		grey := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
		r.timeLabel.Color = grey
		r.statusDot.FillColor = grey
		r.projectLabel.Text = "No active timer"
		r.projectLabel.Color = grey
		r.taskLabel.Text = ""
	}

	r.timeLabel.Refresh()
	r.projectLabel.Refresh()
	r.taskLabel.Refresh()
	r.statusDot.Refresh()
}

func (r *timerDisplayRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.timeLabel, r.projectLabel, r.taskLabel, r.statusDot}
}

func (r *timerDisplayRenderer) Destroy() {
	r.display.StopBlinking()
}

// TimerBox creates a complete timer box with border. Pass any number of
// buttons (Start/Stop, Pause, etc.); they are laid out centred in a horizontal
// row below the timer display.
func TimerBox(timerDisplay *TimerDisplay, buttons ...*widget.Button) fyne.CanvasObject {
	buttonObjs := make([]fyne.CanvasObject, 0, len(buttons))
	for _, b := range buttons {
		if b != nil {
			buttonObjs = append(buttonObjs, b)
		}
	}
	buttonRow := container.NewHBox(buttonObjs...)

	content := container.NewVBox(
		timerDisplay,
		widget.NewSeparator(),
		container.NewCenter(buttonRow),
	)

	// Create bordered container
	border := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	border.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	border.StrokeWidth = 2
	border.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	border.CornerRadius = 8

	return container.NewStack(border, container.NewPadded(content))
}
