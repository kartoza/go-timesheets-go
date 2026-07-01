package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
)

// TeamUpdatesModel is the Bubble Tea model for the Team Updates (microblog)
// screen. Layout: header, feed of posts (arrow-key selectable), compose box
// at the bottom. Keys:
//
//	↑ / ↓ / j / k   navigate feed
//	c / n           focus compose box
//	Esc             leave compose / go back
//	Ctrl+Enter      submit compose
//	l / space       like / unlike selected post
//	e               edit selected (own posts)
//	d               delete selected (own posts)
//	m               load more (older) posts
//	r               refresh
//	q               back to main menu
type TeamUpdatesModel struct {
	apiClient   *api.Client
	width       int
	height      int
	headerState *HeaderState

	posts       []api.MicroblogPost
	currentPage int
	hasMore     bool
	cursor      int // index into posts

	compose        textarea.Model
	composeFocused bool
	editingPostID  int // 0 when composing a new post; >0 when editing an existing one

	loading    bool
	statusMsg  string
	err        error
	confirmDel bool // showing delete confirmation for cursor position
}

// NewTeamUpdatesModel constructs the model. Call Init() (via tea's normal
// lifecycle) to load the first page.
func NewTeamUpdatesModel(apiClient *api.Client) *TeamUpdatesModel {
	ta := textarea.New()
	ta.Placeholder = "Share an update with the team… (Ctrl+Enter to post, Esc to leave)"
	ta.CharLimit = 666
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return &TeamUpdatesModel{
		apiClient:   apiClient,
		compose:     ta,
		currentPage: 1,
	}
}

// SetHeaderState wires the shared header (username / active-timer indicator).
func (m *TeamUpdatesModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// Init loads the first page.
func (m *TeamUpdatesModel) Init() tea.Cmd {
	m.loading = true
	return m.loadPage(1, true)
}

// Update handles all messages.
func (m *TeamUpdatesModel) Update(msg tea.Msg) (*TeamUpdatesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compose.SetWidth(min(msg.Width-6, 80))
		return m, nil

	case teamUpdatesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Failed to load: " + msg.err.Error()
			return m, nil
		}
		m.err = nil
		if msg.replace {
			m.posts = msg.resp.Results
			m.cursor = 0
		} else {
			m.posts = append(m.posts, msg.resp.Results...)
		}
		m.currentPage = msg.page
		m.hasMore = msg.resp.Next
		if len(m.posts) == 0 {
			m.statusMsg = "No updates yet — press c to compose the first."
		} else {
			m.statusMsg = fmt.Sprintf("Showing %d of %d", len(m.posts), msg.resp.Count)
		}
		return m, nil

	case teamUpdatesPostedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = "Post failed: " + msg.err.Error()
			return m, nil
		}
		m.compose.SetValue("")
		m.composeFocused = false
		m.compose.Blur()
		if msg.updated {
			// Edit → replace in place
			for i := range m.posts {
				if m.posts[i].ID == msg.post.ID {
					m.posts[i] = msg.post
					break
				}
			}
			m.statusMsg = "Updated."
			m.editingPostID = 0
		} else {
			// New post → prepend
			m.posts = append([]api.MicroblogPost{msg.post}, m.posts...)
			m.cursor = 0
			m.statusMsg = "Posted."
		}
		return m, nil

	case teamUpdatesLikedMsg:
		if msg.err != nil {
			m.statusMsg = "Like failed: " + msg.err.Error()
			return m, nil
		}
		for i := range m.posts {
			if m.posts[i].ID == msg.postID {
				m.posts[i].Liked = msg.liked
				m.posts[i].LikesCount = msg.count
				break
			}
		}
		return m, nil

	case teamUpdatesDeletedMsg:
		if msg.err != nil {
			m.statusMsg = "Delete failed: " + msg.err.Error()
			return m, nil
		}
		out := m.posts[:0]
		for _, p := range m.posts {
			if p.ID != msg.postID {
				out = append(out, p)
			}
		}
		m.posts = out
		if m.cursor >= len(m.posts) && m.cursor > 0 {
			m.cursor = len(m.posts) - 1
		}
		m.statusMsg = "Deleted."
		return m, nil

	case tea.KeyMsg:
		// While the delete-confirm modal is up, only y/n matter.
		if m.confirmDel {
			switch msg.String() {
			case "y", "Y":
				m.confirmDel = false
				if m.cursor >= 0 && m.cursor < len(m.posts) {
					id := m.posts[m.cursor].ID
					return m, m.doDelete(id)
				}
				return m, nil
			case "n", "N", "esc":
				m.confirmDel = false
				return m, nil
			}
			return m, nil
		}

		if m.composeFocused {
			switch msg.String() {
			case "esc":
				m.composeFocused = false
				m.compose.Blur()
				m.editingPostID = 0
				return m, nil
			case "ctrl+j", "ctrl+m", "ctrl+enter":
				return m, m.submitCompose()
			}
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return backToMenuMsg{} }
		case "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.posts)-1 {
				m.cursor++
			}
			return m, nil
		case "c", "n":
			m.composeFocused = true
			m.editingPostID = 0
			m.compose.Focus()
			return m, textarea.Blink
		case "l", " ":
			if m.cursor >= 0 && m.cursor < len(m.posts) {
				return m, m.doLike(m.posts[m.cursor].ID)
			}
		case "e":
			if m.cursor >= 0 && m.cursor < len(m.posts) && m.posts[m.cursor].IsOwner {
				p := m.posts[m.cursor]
				m.editingPostID = p.ID
				m.compose.SetValue(p.Content)
				m.compose.Focus()
				m.composeFocused = true
				return m, textarea.Blink
			}
		case "d":
			if m.cursor >= 0 && m.cursor < len(m.posts) && m.posts[m.cursor].IsOwner {
				m.confirmDel = true
				return m, nil
			}
		case "m":
			if m.hasMore {
				m.loading = true
				m.statusMsg = "Loading older…"
				return m, m.loadPage(m.currentPage+1, false)
			}
		case "r":
			m.loading = true
			m.statusMsg = "Refreshing…"
			return m, m.loadPage(1, true)
		}
	}
	return m, nil
}

// View renders the screen.
func (m *TeamUpdatesModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	// Header
	var header string
	if m.headerState != nil {
		header = RenderHeader("Team Updates", m.headerState)
	} else {
		header = RenderHeader("Team Updates", &HeaderState{})
	}

	// Feed
	feed := m.renderFeed()

	// Compose
	compose := m.renderCompose()

	// Status / help
	status := m.renderStatusAndHelp()

	// Confirm modal overlay
	main := lipgloss.JoinVertical(lipgloss.Left, header, "", feed, "", compose)
	if m.confirmDel {
		modal := m.renderDeleteConfirm()
		main = lipgloss.JoinVertical(lipgloss.Left, header, "", modal)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		main,
		status,
	)
}

func (m *TeamUpdatesModel) renderFeed() string {
	if len(m.posts) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true)
		if m.loading {
			return empty.Render("Loading updates…")
		}
		return empty.Render("No updates yet — press c to compose the first.")
	}

	// Show a window of posts around the cursor. Approximate 4 rows per post.
	rowsPerPost := 4
	availableRows := m.height - 12
	if availableRows < rowsPerPost {
		availableRows = rowsPerPost
	}
	windowSize := availableRows / rowsPerPost
	if windowSize < 1 {
		windowSize = 1
	}
	start := max(0, m.cursor-windowSize/2)
	end := min(len(m.posts), start+windowSize)
	if end-start < windowSize {
		start = max(0, end-windowSize)
	}

	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, m.renderPostCard(i))
	}
	if m.hasMore && end == len(m.posts) {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true).
			Render("… press m to load older"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *TeamUpdatesModel) renderPostCard(idx int) string {
	p := m.posts[idx]
	selected := idx == m.cursor

	authorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	handleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0"))
	whenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0"))
	contentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	likeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))
	pinStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#569FC6"))
	ownerHintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0")).Italic(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))

	prefix := "  "
	if selected {
		prefix = cursorStyle.Render("▶ ")
	}

	pinned := ""
	if p.IsPinned {
		pinned = pinStyle.Render("★ pinned  ")
	}
	header := fmt.Sprintf("%s%s%s  %s  %s",
		prefix,
		pinned,
		authorStyle.Render(p.AuthorName),
		handleStyle.Render("@"+p.AuthorHandle),
		whenStyle.Render(teamUpdatesFormatCreatedAt(p.CreatedAt)),
	)

	// Content — wrap long lines to a reasonable width.
	wrapW := min(m.width-6, 100)
	content := contentStyle.Render(wordWrap(p.Content, wrapW))

	// Tags
	var tags string
	if len(p.Tags) > 0 {
		names := make([]string, 0, len(p.Tags))
		for _, t := range p.Tags {
			names = append(names, "#"+t.Name)
		}
		tags = tagStyle.Render(strings.Join(names, "  "))
	}

	// Like line
	likeMark := "♡"
	if p.Liked {
		likeMark = "♥"
	}
	likeLine := likeStyle.Render(fmt.Sprintf("%s %d likes", likeMark, p.LikesCount))
	if p.IsOwner {
		likeLine += "  " + ownerHintStyle.Render("(e edit • d delete)")
	}

	parts := []string{header, content}
	if tags != "" {
		parts = append(parts, tags)
	}
	parts = append(parts, likeLine, "")
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *TeamUpdatesModel) renderCompose() string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036")).Bold(true)

	label := "Compose"
	if m.editingPostID > 0 {
		label = "Edit"
	}
	if m.composeFocused {
		label += "  ✎"
	} else {
		label += "  (press c)"
	}

	counterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0"))
	counter := counterStyle.Render(fmt.Sprintf("%d / %d", len(m.compose.Value()), m.compose.CharLimit))

	return lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label)+"   "+counter,
		m.compose.View(),
	)
}

func (m *TeamUpdatesModel) renderStatusAndHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0")).Italic(true)
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#569FC6"))

	help := "↑/↓: nav • c: compose • l: like • e: edit • d: delete • m: more • r: refresh • Esc/q: back"
	if m.composeFocused {
		help = "Ctrl+Enter: post • Esc: leave compose"
	}
	line := helpStyle.Render(help)
	if m.statusMsg != "" {
		line = statusStyle.Render(m.statusMsg) + "  •  " + line
	}
	return line
}

func (m *TeamUpdatesModel) renderDeleteConfirm() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E74C3C")).
		Padding(1, 2)
	msg := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true).Render("Delete update?"),
		"",
		"This cannot be undone.",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0")).Italic(true).Render("y: confirm • n / Esc: cancel"),
	)
	return box.Render(msg)
}

// --- commands ---

func (m *TeamUpdatesModel) loadPage(page int, replace bool) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.apiClient.ListMicroblogPosts(page)
		if err != nil {
			return teamUpdatesLoadedMsg{err: err}
		}
		return teamUpdatesLoadedMsg{resp: *resp, page: page, replace: replace}
	}
}

func (m *TeamUpdatesModel) submitCompose() tea.Cmd {
	content := strings.TrimSpace(m.compose.Value())
	if content == "" {
		m.statusMsg = "Nothing to post."
		return nil
	}
	editID := m.editingPostID
	m.loading = true
	m.statusMsg = "Posting…"
	return func() tea.Msg {
		if editID > 0 {
			post, err := m.apiClient.UpdateMicroblogPost(editID, api.MicroblogWriteRequest{Content: content})
			if err != nil {
				return teamUpdatesPostedMsg{err: err}
			}
			return teamUpdatesPostedMsg{post: *post, updated: true}
		}
		post, err := m.apiClient.CreateMicroblogPost(api.MicroblogWriteRequest{Content: content})
		if err != nil {
			return teamUpdatesPostedMsg{err: err}
		}
		return teamUpdatesPostedMsg{post: *post}
	}
}

func (m *TeamUpdatesModel) doLike(id int) tea.Cmd {
	return func() tea.Msg {
		liked, count, err := m.apiClient.LikeMicroblogPost(id)
		return teamUpdatesLikedMsg{postID: id, liked: liked, count: count, err: err}
	}
}

func (m *TeamUpdatesModel) doDelete(id int) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.DeleteMicroblogPost(id)
		return teamUpdatesDeletedMsg{postID: id, err: err}
	}
}

// --- messages ---

type teamUpdatesLoadedMsg struct {
	resp    api.MicroblogPostsResponse
	page    int
	replace bool
	err     error
}

type teamUpdatesPostedMsg struct {
	post    api.MicroblogPost
	updated bool // true for edits, false for new posts
	err     error
}

type teamUpdatesLikedMsg struct {
	postID int
	liked  bool
	count  int
	err    error
}

type teamUpdatesDeletedMsg struct {
	postID int
	err    error
}

// launchTeamUpdatesMsg is sent by the main menu when the user picks the
// Team Updates action; handled by AppWithAuth to construct the model.
type launchTeamUpdatesMsg struct{}

// --- helpers ---

func teamUpdatesFormatCreatedAt(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Local().Format("Mon Jan 02")
	}
}

// wordWrap folds s at whitespace so no line exceeds width columns. Simple
// implementation — assumes single-byte characters, which is fine for our
// mostly-ASCII use.
func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var out strings.Builder
	for _, para := range strings.Split(s, "\n") {
		line := 0
		for i, word := range strings.Fields(para) {
			if i > 0 {
				if line+1+len(word) > width {
					out.WriteByte('\n')
					line = 0
				} else {
					out.WriteByte(' ')
					line++
				}
			}
			out.WriteString(word)
			line += len(word)
		}
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n")
}
