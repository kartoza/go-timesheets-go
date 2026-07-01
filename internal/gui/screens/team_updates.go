package screens

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
)

// TeamUpdatesScreen renders the microblog feed: compose box at the top,
// paginated list of posts below. Own posts show Edit + Delete affordances;
// every post has a like toggle.
type TeamUpdatesScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window

	// Compose area
	composeEntry   *widget.Entry
	composeCounter *canvas.Text
	postButton     *widget.Button

	// Feed area
	feedContainer *fyne.Container
	loadingBar    *widget.ProgressBarInfinite
	loadMoreBtn   *widget.Button
	statusLabel   *canvas.Text

	// Data
	posts       []api.MicroblogPost
	currentPage int
	hasMore     bool

	// Callbacks
	OnBack func()
}

// contentMaxLen matches the backend serializer's max_length=666.
const contentMaxLen = 666

// NewTeamUpdatesScreen builds the widget tree; call Refresh() after mounting
// to fetch the first page of posts.
func NewTeamUpdatesScreen(apiClient *api.Client, window fyne.Window) *TeamUpdatesScreen {
	s := &TeamUpdatesScreen{
		apiClient:   apiClient,
		window:      window,
		currentPage: 1,
	}
	s.build()
	return s
}

func (s *TeamUpdatesScreen) build() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}

	// Header
	title := canvas.NewText("Team Updates", goldColor)
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	backBtn := widget.NewButton("< Back", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	header := container.NewBorder(nil, nil, backBtn, nil, container.NewCenter(title))

	// Compose box
	s.composeEntry = widget.NewMultiLineEntry()
	s.composeEntry.SetPlaceHolder("Share an update with the team… (up to 666 chars)")
	s.composeEntry.SetMinRowsVisible(3)

	s.composeCounter = canvas.NewText(fmt.Sprintf("0 / %d", contentMaxLen), grayColor)
	s.composeCounter.TextSize = 11
	s.composeCounter.Alignment = fyne.TextAlignTrailing

	s.composeEntry.OnChanged = func(v string) {
		n := len(v)
		if n > contentMaxLen {
			// Truncate rather than reject silently, so the counter stays honest.
			s.composeEntry.SetText(v[:contentMaxLen])
			n = contentMaxLen
		}
		s.composeCounter.Text = fmt.Sprintf("%d / %d", n, contentMaxLen)
		if n > contentMaxLen-40 {
			s.composeCounter.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
		} else {
			s.composeCounter.Color = grayColor
		}
		s.composeCounter.Refresh()
	}

	s.postButton = widget.NewButton("Post", s.onPostClicked)
	s.postButton.Importance = widget.HighImportance

	composeControls := container.NewBorder(nil, nil, s.composeCounter, s.postButton, layout.NewSpacer())

	composeSection := container.NewVBox(
		widget.NewLabel("New update"),
		s.composeEntry,
		composeControls,
		widget.NewSeparator(),
	)

	// Feed container (populated by Refresh)
	s.feedContainer = container.NewVBox()

	s.loadingBar = widget.NewProgressBarInfinite()
	s.loadingBar.Hide()

	s.loadMoreBtn = widget.NewButton("Load older…", s.onLoadMoreClicked)
	s.loadMoreBtn.Hide()

	s.statusLabel = canvas.NewText("", grayColor)
	s.statusLabel.TextSize = 12
	s.statusLabel.Alignment = fyne.TextAlignCenter

	feedScroll := container.NewVScroll(container.NewVBox(
		s.feedContainer,
		container.NewCenter(s.loadMoreBtn),
	))

	s.Container = container.NewBorder(
		container.NewVBox(header, widget.NewSeparator(), composeSection, s.loadingBar),
		container.NewVBox(widget.NewSeparator(), container.NewCenter(s.statusLabel)),
		nil, nil,
		feedScroll,
	)
}

// Refresh clears the current feed and reloads page 1.
func (s *TeamUpdatesScreen) Refresh() {
	s.currentPage = 1
	s.posts = nil
	s.rebuildFeed()
	s.loadPage(1, true)
}

func (s *TeamUpdatesScreen) onLoadMoreClicked() {
	s.loadPage(s.currentPage+1, false)
}

func (s *TeamUpdatesScreen) loadPage(page int, replace bool) {
	s.loadingBar.Show()
	s.setStatus("Loading updates…")

	util.RunAsync(
		func() (*api.MicroblogPostsResponse, error) {
			return s.apiClient.ListMicroblogPosts(page)
		},
		func(resp *api.MicroblogPostsResponse, err error) {
			s.loadingBar.Hide()
			if err != nil {
				s.setStatus("Failed to load: " + err.Error())
				return
			}
			if replace {
				s.posts = resp.Results
			} else {
				s.posts = append(s.posts, resp.Results...)
			}
			s.currentPage = page
			s.hasMore = resp.Next
			s.rebuildFeed()
			if resp.Count == 0 {
				s.setStatus("No updates yet — be the first to post.")
			} else {
				s.setStatus(fmt.Sprintf("Showing %d of %d", len(s.posts), resp.Count))
			}
		},
	)
}

// rebuildFeed regenerates the feed cards from s.posts. Must be called on the
// UI goroutine (util.RunAsync's callback already ensures this).
func (s *TeamUpdatesScreen) rebuildFeed() {
	s.feedContainer.Objects = nil
	for i := range s.posts {
		post := s.posts[i] // capture by index so closures bind to the right entry
		s.feedContainer.Add(s.buildPostCard(&post))
	}
	if s.hasMore {
		s.loadMoreBtn.Show()
	} else {
		s.loadMoreBtn.Hide()
	}
	s.feedContainer.Refresh()
}

// buildPostCard renders a single post as a bordered card.
func (s *TeamUpdatesScreen) buildPostCard(post *api.MicroblogPost) fyne.CanvasObject {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	blueColor := color.NRGBA{R: 0x56, G: 0x9F, B: 0xC6, A: 0xFF}

	// Header line: author + timestamp + pinned marker
	authorText := canvas.NewText(post.AuthorName, color.White)
	authorText.TextSize = 13
	authorText.TextStyle = fyne.TextStyle{Bold: true}

	handleText := canvas.NewText("@"+post.AuthorHandle, grayColor)
	handleText.TextSize = 11

	whenText := canvas.NewText(formatCreatedAt(post.CreatedAt), grayColor)
	whenText.TextSize = 11

	headerItems := []fyne.CanvasObject{authorText, handleText, layout.NewSpacer(), whenText}
	if post.IsPinned {
		pinIcon := canvas.NewText("★ pinned", goldColor)
		pinIcon.TextSize = 11
		headerItems = append([]fyne.CanvasObject{pinIcon}, headerItems...)
	}
	headerRow := container.NewHBox(headerItems...)

	// Content
	contentLabel := widget.NewLabel(post.Content)
	contentLabel.Wrapping = fyne.TextWrapWord

	// Tags
	var tagsRow fyne.CanvasObject
	if len(post.Tags) > 0 {
		tagLabels := make([]string, 0, len(post.Tags))
		for _, t := range post.Tags {
			tagLabels = append(tagLabels, "#"+t.Name)
		}
		tagsText := canvas.NewText(strings.Join(tagLabels, "  "), blueColor)
		tagsText.TextSize = 11
		tagsRow = tagsText
	}

	// Actions: like, edit (own), delete (own)
	likeLabel := fmt.Sprintf("♡ %d", post.LikesCount)
	if post.Liked {
		likeLabel = fmt.Sprintf("♥ %d", post.LikesCount)
	}
	likeBtn := widget.NewButton(likeLabel, func() {
		s.onLikeClicked(post.ID)
	})
	if post.Liked {
		likeBtn.Importance = widget.HighImportance
	}

	actionItems := []fyne.CanvasObject{likeBtn}
	if post.IsOwner {
		editBtn := widget.NewButton("Edit", func() { s.onEditClicked(post) })
		deleteBtn := widget.NewButton("Delete", func() { s.onDeleteClicked(post) })
		deleteBtn.Importance = widget.DangerImportance
		actionItems = append(actionItems, editBtn, deleteBtn)
	}
	actionRow := container.NewHBox(actionItems...)

	// Assemble
	inner := container.NewVBox(headerRow, contentLabel)
	if tagsRow != nil {
		inner.Add(tagsRow)
	}
	inner.Add(actionRow)

	border := canvas.NewRectangle(color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF})
	border.StrokeColor = color.NRGBA{R: 0x3D, G: 0x3E, B: 0x40, A: 0xFF}
	border.StrokeWidth = 1
	border.CornerRadius = 6

	return container.NewStack(border, container.NewPadded(inner))
}

func (s *TeamUpdatesScreen) onPostClicked() {
	content := strings.TrimSpace(s.composeEntry.Text)
	if content == "" {
		s.setStatus("Nothing to post.")
		return
	}
	req := api.MicroblogWriteRequest{Content: content}

	s.postButton.Disable()
	s.setStatus("Posting…")

	util.RunAsync(
		func() (*api.MicroblogPost, error) {
			return s.apiClient.CreateMicroblogPost(req)
		},
		func(post *api.MicroblogPost, err error) {
			s.postButton.Enable()
			if err != nil {
				s.setStatus("Post failed: " + err.Error())
				return
			}
			// Clear the compose box and prepend the new post to the feed
			// without a full refetch — quicker + preserves scroll position.
			s.composeEntry.SetText("")
			if post != nil {
				s.posts = append([]api.MicroblogPost{*post}, s.posts...)
				s.rebuildFeed()
			}
			s.setStatus("Posted.")
		},
	)
}

func (s *TeamUpdatesScreen) onLikeClicked(id int) {
	util.RunAsync(
		func() (likeResult, error) {
			liked, count, err := s.apiClient.LikeMicroblogPost(id)
			return likeResult{liked: liked, count: count}, err
		},
		func(r likeResult, err error) {
			if err != nil {
				s.setStatus("Like failed: " + err.Error())
				return
			}
			// Update local state so we don't have to refetch the whole feed.
			for i := range s.posts {
				if s.posts[i].ID == id {
					s.posts[i].Liked = r.liked
					s.posts[i].LikesCount = r.count
					break
				}
			}
			s.rebuildFeed()
		},
	)
}

// likeResult is a tiny helper so util.RunAsync's single-value generic works
// for the two-value LikeMicroblogPost return.
type likeResult struct {
	liked bool
	count int
}

func (s *TeamUpdatesScreen) onEditClicked(post *api.MicroblogPost) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(post.Content)
	entry.SetMinRowsVisible(4)

	counter := canvas.NewText(fmt.Sprintf("%d / %d", len(post.Content), contentMaxLen),
		color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	counter.TextSize = 11
	counter.Alignment = fyne.TextAlignTrailing
	entry.OnChanged = func(v string) {
		if len(v) > contentMaxLen {
			entry.SetText(v[:contentMaxLen])
		}
		counter.Text = fmt.Sprintf("%d / %d", len(entry.Text), contentMaxLen)
		counter.Refresh()
	}

	content := container.NewBorder(nil, counter, nil, nil, entry)

	dialog.ShowCustomConfirm("Edit update", "Save", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		newContent := strings.TrimSpace(entry.Text)
		if newContent == "" {
			s.setStatus("Content cannot be empty.")
			return
		}
		s.setStatus("Saving…")
		id := post.ID
		util.RunAsync(
			func() (*api.MicroblogPost, error) {
				return s.apiClient.UpdateMicroblogPost(id, api.MicroblogWriteRequest{Content: newContent})
			},
			func(updated *api.MicroblogPost, err error) {
				if err != nil {
					s.setStatus("Update failed: " + err.Error())
					return
				}
				if updated != nil {
					for i := range s.posts {
						if s.posts[i].ID == id {
							s.posts[i] = *updated
							break
						}
					}
					s.rebuildFeed()
				}
				s.setStatus("Updated.")
			},
		)
	}, s.window)
}

func (s *TeamUpdatesScreen) onDeleteClicked(post *api.MicroblogPost) {
	dialog.ShowConfirm("Delete update",
		"Delete this update? This cannot be undone.",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			id := post.ID
			s.setStatus("Deleting…")
			util.RunAsync(
				func() (bool, error) {
					return true, s.apiClient.DeleteMicroblogPost(id)
				},
				func(_ bool, err error) {
					if err != nil {
						s.setStatus("Delete failed: " + err.Error())
						return
					}
					// Remove from local state
					out := s.posts[:0]
					for _, p := range s.posts {
						if p.ID != id {
							out = append(out, p)
						}
					}
					s.posts = out
					s.rebuildFeed()
					s.setStatus("Deleted.")
				},
			)
		}, s.window)
}

func (s *TeamUpdatesScreen) setStatus(msg string) {
	s.statusLabel.Text = msg
	s.statusLabel.Refresh()
}

// formatCreatedAt renders a backend ISO-8601 timestamp as a friendly relative
// string ("2m ago", "3h ago", "5d ago") for recent posts; falls back to the
// full local date for older ones.
func formatCreatedAt(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		// Try without offset (backend may emit fractional seconds)
		t, err = time.Parse("2006-01-02T15:04:05.999999Z", iso)
		if err != nil {
			return iso
		}
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
