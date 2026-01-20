package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/kartoza/go-timesheets-go/cmd"
)

func main() {
	// Set up panic recovery to write to crash log
	defer func() {
		if r := recover(); r != nil {
			homeDir, _ := os.UserHomeDir()
			crashLog := filepath.Join(homeDir, ".local", "share", "kartoza-timesheet", "crash.log")
			os.MkdirAll(filepath.Dir(crashLog), 0755)

			f, err := os.OpenFile(crashLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				defer f.Close()
				fmt.Fprintf(f, "\n=== CRASH at %s ===\n", time.Now().Format(time.RFC3339))
				fmt.Fprintf(f, "Panic: %v\n\n", r)
				fmt.Fprintf(f, "Stack trace:\n%s\n", debug.Stack())
			}

			// Also print to stderr
			fmt.Fprintf(os.Stderr, "\nApplication crashed! See %s for details.\n", crashLog)
			fmt.Fprintf(os.Stderr, "Panic: %v\n", r)
			os.Exit(1)
		}
	}()

	cmd.Execute()
}