package office

import "github.com/gdamore/tcell/v2"

// Sprite represents an 8-bit style character sprite
// Each sprite is 3 wide x 3 tall for terminal display
type Sprite struct {
	Frames [][]string // Animation frames
	Colors [][]int    // Color indices per cell (-1 = use character color)
}

// GetCharacterSprite returns sprite frames for a character based on state and direction
func GetCharacterSprite(state CharacterState, dir Direction, frame int) ([]string, []int) {
	switch state {
	case StateWalking:
		return getWalkingSprite(dir, frame)
	case StateWorking:
		return getWorkingSprite(frame)
	case StateTalking:
		return getTalkingSprite(frame)
	default:
		return getIdleSprite(dir, frame)
	}
}

// 8-bit style character sprites using Unicode block characters
// Each character is 3x3 cells

func getIdleSprite(dir Direction, frame int) ([]string, []int) {
	// Idle animation - slight breathing effect
	switch dir {
	case DirDown:
		if frame%2 == 0 {
			return []string{
				" ◕ ",
				"╔█╗",
				"║ ║",
			}, []int{0, -1, 0}
		}
		return []string{
			" ◕ ",
			"╔█╗",
			"╚═╝",
		}, []int{0, -1, 0}
	case DirUp:
		return []string{
			" ◒ ",
			"╔█╗",
			"║ ║",
		}, []int{0, -1, 0}
	case DirLeft:
		return []string{
			"◐  ",
			"█╗ ",
			"║╚ ",
		}, []int{0, -1, 0}
	case DirRight:
		return []string{
			"  ◑",
			" ╔█",
			" ╝║",
		}, []int{0, -1, 0}
	}
	return []string{" ◕ ", "╔█╗", "║ ║"}, []int{0, -1, 0}
}

func getWalkingSprite(dir Direction, frame int) ([]string, []int) {
	// Walking animation - 2 frame cycle
	switch dir {
	case DirDown:
		if frame%2 == 0 {
			return []string{
				" ◕ ",
				"╔█╗",
				"╝ ╚",
			}, []int{0, -1, 0}
		}
		return []string{
			" ◕ ",
			"╔█╗",
			"╚ ╝",
		}, []int{0, -1, 0}
	case DirUp:
		if frame%2 == 0 {
			return []string{
				" ◒ ",
				"╔█╗",
				"╝ ╚",
			}, []int{0, -1, 0}
		}
		return []string{
			" ◒ ",
			"╔█╗",
			"╚ ╝",
		}, []int{0, -1, 0}
	case DirLeft:
		if frame%2 == 0 {
			return []string{
				"◐  ",
				"█╗ ",
				"╝╚ ",
			}, []int{0, -1, 0}
		}
		return []string{
			"◐  ",
			"█╗ ",
			"╚╝ ",
		}, []int{0, -1, 0}
	case DirRight:
		if frame%2 == 0 {
			return []string{
				"  ◑",
				" ╔█",
				" ╝╚",
			}, []int{0, -1, 0}
		}
		return []string{
			"  ◑",
			" ╔█",
			" ╚╝",
		}, []int{0, -1, 0}
	}
	return getIdleSprite(dir, frame)
}

func getWorkingSprite(frame int) ([]string, []int) {
	// Working at desk - typing animation
	if frame%4 < 2 {
		return []string{
			" ◕ ",
			"╔█╗",
			"╠╦╣",
		}, []int{0, -1, 0}
	}
	return []string{
		" ◔ ",
		"╔█╗",
		"╠╩╣",
	}, []int{0, -1, 0}
}

func getTalkingSprite(frame int) ([]string, []int) {
	// Talking animation
	if frame%3 == 0 {
		return []string{
			" ◕ ",
			"╔█╗",
			"╚═╝",
		}, []int{0, -1, 0}
	} else if frame%3 == 1 {
		return []string{
			" ◔ ",
			"║█║",
			"╚═╝",
		}, []int{0, -1, 0}
	}
	return []string{
		" ◕ ",
		"║█║",
		"╚═╝",
	}, []int{0, -1, 0}
}

// Office furniture sprites

// DeskSprite - a simple desk (4x2)
var DeskSprite = []string{
	"┌──┐",
	"└┬┬┘",
}

// ChairSprite - office chair (2x1)
var ChairSprite = []string{
	"╔╗",
}

// ComputerSprite - monitor on desk (3x2)
var ComputerSprite = []string{
	"┌─┐",
	"╘═╛",
}

// PlantSprite - decorative plant (2x2)
var PlantSprite = []string{
	"🌿",
	"┃ ",
}

// CoffeeMachineSprite - coffee machine (3x2)
var CoffeeMachineSprite = []string{
	"╔═╗",
	"╚☕╝",
}

// WhiteboardSprite - whiteboard (6x3)
var WhiteboardSprite = []string{
	"┌────┐",
	"│████│",
	"└────┘",
}

// WaterCoolerSprite - water cooler (2x3)
var WaterCoolerSprite = []string{
	"╔╗",
	"║║",
	"╚╝",
}

// DrawSprite draws a sprite at the given position
func DrawSprite(screen tcell.Screen, x, y int, sprite []string, color tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack)

	for row, line := range sprite {
		col := 0
		for _, ch := range line {
			screen.SetContent(x+col, y+row, ch, nil, style)
			col++
		}
	}
}

// DrawCharacter draws a character sprite with their assigned color
func DrawCharacter(screen tcell.Screen, member *TeamMember, frame int) {
	spriteLines, _ := GetCharacterSprite(member.State, member.Facing, frame)

	// Head color (skin tone)
	skinColor := tcell.NewRGBColor(255, 218, 185) // Peach

	// Body color from member's assigned color
	bodyColor := tcell.NewRGBColor(
		int32((member.Color>>16)&0xFF),
		int32((member.Color>>8)&0xFF),
		int32(member.Color&0xFF),
	)

	x := int(member.X)
	y := int(member.Y)

	for row, line := range spriteLines {
		col := 0
		for _, ch := range line {
			var style tcell.Style
			if row == 0 {
				// Head row - use skin color
				style = tcell.StyleDefault.Foreground(skinColor).Background(tcell.ColorBlack)
			} else {
				// Body rows - use character color
				style = tcell.StyleDefault.Foreground(bodyColor).Background(tcell.ColorBlack)
			}
			if ch != ' ' {
				screen.SetContent(x+col, y+row, ch, nil, style)
			}
			col++
		}
	}
}

// DrawSpeechBubble draws a speech bubble above a character
func DrawSpeechBubble(screen tcell.Screen, x, y int, text string, maxWidth int) {
	if text == "" {
		return
	}

	// Truncate text if too long
	if len(text) > maxWidth-4 {
		text = text[:maxWidth-7] + "..."
	}

	bubbleWidth := len(text) + 4
	bubbleX := x - bubbleWidth/2 + 1
	bubbleY := y - 3

	// Keep bubble on screen
	if bubbleX < 0 {
		bubbleX = 0
	}

	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)

	// Top border
	screen.SetContent(bubbleX, bubbleY, '╭', nil, borderStyle)
	for i := 1; i < bubbleWidth-1; i++ {
		screen.SetContent(bubbleX+i, bubbleY, '─', nil, borderStyle)
	}
	screen.SetContent(bubbleX+bubbleWidth-1, bubbleY, '╮', nil, borderStyle)

	// Middle with text
	screen.SetContent(bubbleX, bubbleY+1, '│', nil, borderStyle)
	screen.SetContent(bubbleX+1, bubbleY+1, ' ', nil, style)
	col := 2
	for _, ch := range text {
		screen.SetContent(bubbleX+col, bubbleY+1, ch, nil, style)
		col++
	}
	screen.SetContent(bubbleX+bubbleWidth-2, bubbleY+1, ' ', nil, style)
	screen.SetContent(bubbleX+bubbleWidth-1, bubbleY+1, '│', nil, borderStyle)

	// Bottom border with pointer
	screen.SetContent(bubbleX, bubbleY+2, '╰', nil, borderStyle)
	for i := 1; i < bubbleWidth-1; i++ {
		if i == bubbleWidth/2 {
			screen.SetContent(bubbleX+i, bubbleY+2, '┴', nil, borderStyle)
		} else {
			screen.SetContent(bubbleX+i, bubbleY+2, '─', nil, borderStyle)
		}
	}
	screen.SetContent(bubbleX+bubbleWidth-1, bubbleY+2, '╯', nil, borderStyle)

	// Pointer
	screen.SetContent(bubbleX+bubbleWidth/2, bubbleY+3, '╽', nil, borderStyle)
}

// DrawNameTag draws a small name tag below a character
func DrawNameTag(screen tcell.Screen, x, y int, name string) {
	// Get first name only
	parts := splitName(name)
	firstName := parts[0]

	tagX := x - len(firstName)/2 + 1
	tagY := y + 3

	style := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)

	col := 0
	for _, ch := range firstName {
		screen.SetContent(tagX+col, tagY, ch, nil, style)
		col++
	}
}

func splitName(name string) []string {
	parts := []string{}
	current := ""
	for _, ch := range name {
		if ch == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	if len(parts) == 0 {
		return []string{name}
	}
	return parts
}
