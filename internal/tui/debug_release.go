//go:build release

package tui

import (
	"errors"
	"io"
	"log"
)

// Debug logging - disabled in release builds
var tuiDebugLog *log.Logger
var menuDebugLog *log.Logger

// DebugEnabled indicates if debug mode is active
const DebugEnabled = false

func init() {
	// In release mode, all debug logging goes to discard
	tuiDebugLog = log.New(io.Discard, "", 0)
	menuDebugLog = log.New(io.Discard, "", 0)
}

// LaunchMonitor is a no-op in release builds
func LaunchMonitor() error {
	return errors.New("monitoring not available in release builds")
}

// LaunchAPILog is a no-op in release builds
func LaunchAPILog() error {
	return errors.New("API log not available in release builds")
}
