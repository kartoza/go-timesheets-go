package office

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// TeamMember represents a Kartoza team member
type TeamMember struct {
	Name      string
	Role      string
	AvatarURL string
	Bio       string
	Color     int // Assigned color for the character
	// Position in the office
	X, Y     float64
	TargetX  float64
	TargetY  float64
	VX, VY   float64
	// Current activity
	Activity string
	// Animation state
	Frame    int
	Facing   Direction
	State    CharacterState
	IdleTime float64
}

// Direction for character facing
type Direction int

const (
	DirDown Direction = iota
	DirUp
	DirLeft
	DirRight
)

// CharacterState for animation
type CharacterState int

const (
	StateIdle CharacterState = iota
	StateWalking
	StateWorking
	StateTalking
)

// FetchTeamMembers fetches team data from kartoza.com
func FetchTeamMembers() ([]TeamMember, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://kartoza.com/the_team")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch team page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return parseTeamPage(string(body))
}

// parseTeamPage extracts team member info from HTML
func parseTeamPage(html string) ([]TeamMember, error) {
	var members []TeamMember

	// Pattern to find team member blocks
	// Looking for person-name and person-role classes
	namePattern := regexp.MustCompile(`<h4[^>]*class="[^"]*person-name[^"]*"[^>]*>([^<]+)</h4>`)
	rolePattern := regexp.MustCompile(`<p[^>]*class="[^"]*person-role[^"]*"[^>]*>([^<]+)</p>`)
	avatarPattern := regexp.MustCompile(`<img[^>]*class="[^"]*profile-image[^"]*"[^>]*src="([^"]+)"`)
	bioPattern := regexp.MustCompile(`<div[^>]*class="[^"]*line-clamp[^"]*"[^>]*>([^<]+)`)

	names := namePattern.FindAllStringSubmatch(html, -1)
	roles := rolePattern.FindAllStringSubmatch(html, -1)
	avatars := avatarPattern.FindAllStringSubmatch(html, -1)
	bios := bioPattern.FindAllStringSubmatch(html, -1)

	// Build members from matched data
	for i, name := range names {
		member := TeamMember{
			Name:  strings.TrimSpace(name[1]),
			Color: i % len(characterColors),
		}

		if i < len(roles) {
			member.Role = strings.TrimSpace(roles[i][1])
		}

		if i < len(avatars) {
			url := avatars[i][1]
			if !strings.HasPrefix(url, "http") {
				url = "https://kartoza.com" + url
			}
			member.AvatarURL = url
		}

		if i < len(bios) {
			member.Bio = strings.TrimSpace(bios[i][1])
		}

		members = append(members, member)
	}

	// If parsing failed, use hardcoded fallback
	if len(members) == 0 {
		members = getHardcodedTeam()
	}

	return members, nil
}

// getHardcodedTeam returns a fallback list of team members
func getHardcodedTeam() []TeamMember {
	team := []TeamMember{
		{Name: "Tim Sutton", Role: "Director", Activity: "Reviewing PRs"},
		{Name: "Gavin Fleming", Role: "Director", Activity: "Client meeting"},
		{Name: "Marike Kruger", Role: "General Manager", Activity: "Planning sprint"},
		{Name: "Irwan Fathurrahman", Role: "Lead Developer", Activity: "Coding"},
		{Name: "Zakki Muzakki", Role: "Software Developer", Activity: "Bug fixing"},
		{Name: "Seabilwe Tilodi", Role: "Head of Training", Activity: "Creating course"},
		{Name: "Dimas Tri Ciputra", Role: "Senior Software Developer", Activity: "Code review"},
		{Name: "Victoria Nyaga", Role: "GIS Product Developer", Activity: "QGIS plugin"},
		{Name: "Jeremy Prior", Role: "GIS Specialist", Activity: "Map styling"},
		{Name: "Leon Greyling", Role: "DevOps Manager", Activity: "Server maintenance"},
		{Name: "Elia Volschenk", Role: "Scrum Master", Activity: "Stand-up meeting"},
		{Name: "Danang Tri Massandy", Role: "Senior Developer", Activity: "API development"},
		{Name: "Jeff Osundwa", Role: "QGIS Developer", Activity: "Plugin testing"},
		{Name: "Lova Andriarimalala", Role: "Full Stack Developer", Activity: "Frontend work"},
		{Name: "Fawzy Aly", Role: "Scrum Master", Activity: "Sprint planning"},
	}

	for i := range team {
		team[i].Color = i % len(characterColors)
	}

	return team
}

// Character colors (shirt colors for 8-bit sprites)
var characterColors = []int{
	0xFF5722, // Deep Orange
	0x2196F3, // Blue
	0x4CAF50, // Green
	0x9C27B0, // Purple
	0xE91E63, // Pink
	0x00BCD4, // Cyan
	0xFFEB3B, // Yellow
	0x795548, // Brown
	0x607D8B, // Blue Grey
	0xF44336, // Red
	0x3F51B5, // Indigo
	0x009688, // Teal
	0xFF9800, // Orange
	0x673AB7, // Deep Purple
	0x8BC34A, // Light Green
}
