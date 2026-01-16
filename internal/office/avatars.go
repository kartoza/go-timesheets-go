package office

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF decoder
	_ "image/jpeg" // Register JPEG decoder
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blacktop/go-termimg"
	"github.com/nfnt/resize"
)

// AvatarManager handles downloading, caching, and rendering team member avatars
type AvatarManager struct {
	cacheDir      string
	avatars       map[string]image.Image
	mu            sync.RWMutex
	kittySupport  bool
	avatarSize    uint // Size in pixels for avatar images
}

// NewAvatarManager creates a new avatar manager
func NewAvatarManager() *AvatarManager {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".cache", "kartoza-timesheet", "avatars")
	os.MkdirAll(cacheDir, 0755)

	am := &AvatarManager{
		cacheDir:   cacheDir,
		avatars:    make(map[string]image.Image),
		avatarSize: 64, // Small avatar size for terminal display
	}

	// Detect Kitty support
	am.kittySupport = detectKittySupport()

	return am
}

// detectKittySupport checks if the terminal supports Kitty graphics protocol
func detectKittySupport() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	kittyWindowID := os.Getenv("KITTY_WINDOW_ID")

	// Check for Kitty terminal
	if kittyWindowID != "" {
		return true
	}

	// Check for xterm-kitty or kitty in TERM
	if strings.Contains(term, "kitty") {
		return true
	}

	// Check TERM_PROGRAM
	if termProgram == "kitty" {
		return true
	}

	// Try using go-termimg's detection
	protocol := termimg.DetectProtocol()
	return protocol == termimg.Kitty
}

// HasKittySupport returns whether Kitty graphics are supported
func (am *AvatarManager) HasKittySupport() bool {
	return am.kittySupport
}

// LoadAvatars downloads and caches avatars for all team members
func (am *AvatarManager) LoadAvatars(members []TeamMember) {
	var wg sync.WaitGroup

	for i := range members {
		if members[i].AvatarURL == "" {
			continue
		}

		wg.Add(1)
		go func(member *TeamMember) {
			defer wg.Done()
			am.loadAvatar(member)
		}(&members[i])
	}

	wg.Wait()
}

// loadAvatar downloads and caches a single avatar
func (am *AvatarManager) loadAvatar(member *TeamMember) {
	if member.AvatarURL == "" {
		return
	}

	// Generate cache key from URL
	hash := md5.Sum([]byte(member.AvatarURL))
	cacheKey := hex.EncodeToString(hash[:])
	cachePath := filepath.Join(am.cacheDir, cacheKey+".png")

	// Check if cached
	if img := am.loadFromCache(cachePath); img != nil {
		am.mu.Lock()
		am.avatars[member.Name] = img
		am.mu.Unlock()
		return
	}

	// Download avatar
	img, err := am.downloadAvatar(member.AvatarURL)
	if err != nil {
		return
	}

	// Resize to avatar size
	img = resize.Resize(am.avatarSize, am.avatarSize, img, resize.Lanczos3)

	// Save to cache
	am.saveToCache(cachePath, img)

	am.mu.Lock()
	am.avatars[member.Name] = img
	am.mu.Unlock()
}

// loadFromCache loads an image from the cache
func (am *AvatarManager) loadFromCache(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil
	}

	return img
}

// saveToCache saves an image to the cache
func (am *AvatarManager) saveToCache(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	png.Encode(f, img)
}

// downloadAvatar downloads an image from a URL
func (am *AvatarManager) downloadAvatar(url string) (image.Image, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download: %s", resp.Status)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// GetAvatar returns the cached avatar image for a team member
func (am *AvatarManager) GetAvatar(name string) image.Image {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.avatars[name]
}

// RenderAvatar renders an avatar at the specified terminal position
// Returns the ANSI escape sequences to display the image
// imageID should be unique per avatar to prevent Kitty from reusing cached images
func (am *AvatarManager) RenderAvatar(name string, x, y, imageID int) string {
	if !am.kittySupport {
		return ""
	}

	img := am.GetAvatar(name)
	if img == nil {
		return ""
	}

	// Encode image to PNG bytes
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}

	// Use go-termimg to render with Kitty protocol
	ti, err := termimg.From(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return ""
	}

	// Configure for small display (roughly 3x3 terminal cells)
	// Use unique ImageNum to prevent caching issues between different avatars
	ti.Protocol(termimg.Kitty).
		Width(3).
		Height(3).
		Scale(termimg.ScaleFit).
		ImageNum(imageID)

	// Get the rendered output
	rendered, err := ti.Render()
	if err != nil {
		return ""
	}

	// Position cursor and render
	// ANSI escape: \033[<row>;<col>H moves cursor to row, col (1-indexed)
	return fmt.Sprintf("\033[%d;%dH%s", y+1, x+1, rendered)
}

// RenderAvatarToWriter renders an avatar directly to a writer at current cursor position
func (am *AvatarManager) RenderAvatarToWriter(w io.Writer, name string) error {
	if !am.kittySupport {
		return fmt.Errorf("Kitty graphics not supported")
	}

	img := am.GetAvatar(name)
	if img == nil {
		return fmt.Errorf("avatar not found for %s", name)
	}

	// Encode image to PNG bytes
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}

	// Use go-termimg to render
	ti, err := termimg.From(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}

	ti.Protocol(termimg.Kitty).
		Width(3).
		Height(3).
		Scale(termimg.ScaleFit)

	// Render and write to writer
	rendered, err := ti.Render()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(rendered))
	return err
}

// ClearCache removes all cached avatars
func (am *AvatarManager) ClearCache() error {
	return os.RemoveAll(am.cacheDir)
}

// LoadedCount returns the number of avatars currently loaded
func (am *AvatarManager) LoadedCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.avatars)
}

// GetCacheDir returns the cache directory path
func (am *AvatarManager) GetCacheDir() string {
	return am.cacheDir
}
