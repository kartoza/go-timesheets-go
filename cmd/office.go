package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/kartoza/go-timesheets-go/internal/office"
)

// officeCmd represents the office command
var officeCmd = &cobra.Command{
	Use:   "office",
	Short: "Launch the Kartoza virtual office",
	Long: `Launch an 8-bit style virtual office scene showing Kartoza team members.

Team members are fetched from https://kartoza.com/the_team and displayed
as animated characters in a top-down office view. Each character shows
their current activity in a speech bubble.

Controls:
  q, ESC - Quit
  b      - Toggle speech bubbles

Examples:
  kartoza-timesheet office`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := office.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(officeCmd)
}
