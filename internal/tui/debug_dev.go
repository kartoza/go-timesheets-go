//go:build !release

package tui

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Debug logging - enabled in development builds
var tuiDebugLog *log.Logger
var menuDebugLog *log.Logger

// DebugEnabled indicates if debug mode is active (controlled by --debug/-d flag)
// In dev builds, this defaults to false and can be enabled via command line
var DebugEnabled = false

// SetDebugMode sets the debug mode flag (only effective in dev builds)
func SetDebugMode(enabled bool) {
	DebugEnabled = enabled
}

func init() {
	f, err := os.OpenFile("/tmp/kartoza-timesheet-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		tuiDebugLog = log.New(io.Discard, "", 0)
		menuDebugLog = log.New(io.Discard, "", 0)
	} else {
		tuiDebugLog = log.New(f, "TUI: ", log.LstdFlags|log.Lshortfile)
		menuDebugLog = log.New(f, "MENU: ", log.LstdFlags|log.Lshortfile)
	}
}

// LaunchMonitor launches expvarmon in a new terminal window
func LaunchMonitor() error {
	// expvarmon command showing API metrics and memory stats
	// Metrics: API calls, errors, cache ratio, memory allocation
	expvarmonCmd := "expvarmon -ports=6060 -vars=\"api.requests.total,api.requests.errors,api.requests.inflight,duration:api.requests.duration_ms,api.cache_hit_ratio,mem:memstats.Alloc,mem:memstats.HeapAlloc\" -i 1s"

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		// Try common Linux terminal emulators (kitty preferred)
		terminals := []struct {
			name string
			args []string
		}{
			{"kitty", []string{"bash", "-c", expvarmonCmd + "; read -p 'Press Enter to close...'"}},
			{"gnome-terminal", []string{"--", "bash", "-c", expvarmonCmd + "; read -p 'Press Enter to close...'"}},
			{"konsole", []string{"-e", "bash", "-c", expvarmonCmd + "; read -p 'Press Enter to close...'"}},
			{"xfce4-terminal", []string{"-e", "bash -c \"" + expvarmonCmd + "; read -p 'Press Enter to close...'\""}},
			{"alacritty", []string{"-e", "bash", "-c", expvarmonCmd + "; read -p 'Press Enter to close...'"}},
			{"xterm", []string{"-e", "bash", "-c", expvarmonCmd + "; read -p 'Press Enter to close...'"}},
		}

		for _, term := range terminals {
			if path, err := exec.LookPath(term.name); err == nil {
				cmd = exec.Command(path, term.args...)
				break
			}
		}

		if cmd == nil {
			// Fallback: try x-terminal-emulator (Debian/Ubuntu)
			if path, err := exec.LookPath("x-terminal-emulator"); err == nil {
				cmd = exec.Command(path, "-e", "bash", "-c", expvarmonCmd+"; read -p 'Press Enter to close...'")
			}
		}

	case "darwin":
		// macOS - use osascript to open Terminal.app
		script := `tell application "Terminal" to do script "` + expvarmonCmd + `"`
		cmd = exec.Command("osascript", "-e", script)

	case "windows":
		// Windows - use cmd.exe to start a new window
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", expvarmonCmd)
	}

	if cmd == nil {
		return exec.ErrNotFound
	}

	return cmd.Start()
}

// LaunchAPILog launches a terminal window showing the scrolling API request log
func LaunchAPILog() error {
	// Get log file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(homeDir, ".config/.kartoza-timesheets/logs")
	logFile := filepath.Join(logDir, fmt.Sprintf("api-requests-%s.log", time.Now().Format("2006-01-02")))

	// tail -f command to follow the log
	tailCmd := fmt.Sprintf("tail -f %s", logFile)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		// Try common Linux terminal emulators (kitty preferred)
		terminals := []struct {
			name string
			args []string
		}{
			{"kitty", []string{"--title", "API Request Log", "bash", "-c", tailCmd + "; read -p 'Press Enter to close...'"}},
			{"gnome-terminal", []string{"--title=API Request Log", "--", "bash", "-c", tailCmd + "; read -p 'Press Enter to close...'"}},
			{"konsole", []string{"-e", "bash", "-c", tailCmd + "; read -p 'Press Enter to close...'"}},
			{"xfce4-terminal", []string{"--title=API Request Log", "-e", "bash -c \"" + tailCmd + "; read -p 'Press Enter to close...'\""}},
			{"alacritty", []string{"--title", "API Request Log", "-e", "bash", "-c", tailCmd + "; read -p 'Press Enter to close...'"}},
			{"xterm", []string{"-title", "API Request Log", "-e", "bash", "-c", tailCmd + "; read -p 'Press Enter to close...'"}},
		}

		for _, term := range terminals {
			if path, err := exec.LookPath(term.name); err == nil {
				cmd = exec.Command(path, term.args...)
				break
			}
		}

		if cmd == nil {
			// Fallback: try x-terminal-emulator (Debian/Ubuntu)
			if path, err := exec.LookPath("x-terminal-emulator"); err == nil {
				cmd = exec.Command(path, "-e", "bash", "-c", tailCmd+"; read -p 'Press Enter to close...'")
			}
		}

	case "darwin":
		// macOS - use osascript to open Terminal.app
		script := `tell application "Terminal" to do script "` + tailCmd + `"`
		cmd = exec.Command("osascript", "-e", script)

	case "windows":
		// Windows - use PowerShell Get-Content with -Wait
		psCmd := fmt.Sprintf("Get-Content -Path '%s' -Wait", logFile)
		cmd = exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", psCmd)
	}

	if cmd == nil {
		return exec.ErrNotFound
	}

	return cmd.Start()
}
