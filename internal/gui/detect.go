package gui

import (
	"os"
	"runtime"
)

// CanUseGUI checks if a graphical environment is available
func CanUseGUI() bool {
	switch runtime.GOOS {
	case "linux":
		// Check for X11 or Wayland display
		if os.Getenv("DISPLAY") != "" {
			return true
		}
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			return true
		}
		// Also check XDG_SESSION_TYPE
		sessionType := os.Getenv("XDG_SESSION_TYPE")
		if sessionType == "x11" || sessionType == "wayland" {
			return true
		}
		return false
	case "darwin":
		// macOS always has GUI unless in SSH without X forwarding
		if os.Getenv("SSH_TTY") != "" && os.Getenv("DISPLAY") == "" {
			return false
		}
		return true
	case "windows":
		// Windows always has GUI available
		return true
	default:
		return false
	}
}
