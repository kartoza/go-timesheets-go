package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all available projects",
	Long:  `Fetches and displays all available projects from the timesheet API in a formatted table.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load auth token
		token, err := config.LoadToken()
		if err != nil || token == nil {
			fmt.Fprintln(os.Stderr, "Error: Not logged in. Please run the TUI first to authenticate.")
			os.Exit(1)
		}

		// Create API client
		client, err := api.NewClient(api.Config{
			BaseURL:   token.BaseURL,
			AuthToken: token.Token,
			Timeout:   30,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create API client: %v\n", err)
			os.Exit(1)
		}

		// Fetch projects
		projects, err := client.GetProjects("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to fetch projects: %v\n", err)
			os.Exit(1)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return
		}

		// Create table
		re := lipgloss.NewRenderer(os.Stdout)

		headerStyle := re.NewStyle().
			Foreground(lipgloss.Color("#E95420")).
			Bold(true).
			Align(lipgloss.Center)

		evenRowStyle := re.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

		oddRowStyle := re.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

		borderStyle := re.NewStyle().
			Foreground(lipgloss.Color("#E95420"))

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(borderStyle).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == 0 {
					return headerStyle
				}
				if row%2 == 0 {
					return evenRowStyle
				}
				return oddRowStyle
			}).
			Headers("ID", "LABEL")

		// Add rows
		for _, project := range projects {
			t.Row(
				fmt.Sprintf("%d", project.ID),
				project.Label,
			)
		}

		// Print table
		fmt.Println(t.Render())
		fmt.Printf("\nTotal projects: %d\n", len(projects))
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}
