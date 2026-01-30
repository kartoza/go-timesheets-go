package cmd

import (
	"fmt"

	"github.com/kartoza/go-timesheets-go/internal/ai"
	"github.com/spf13/cobra"
)

var csvPath string

var importCmd = &cobra.Command{
	Use:   "import-history",
	Short: "Import CSV timesheet history for AI training",
	Long: `Import an ERPNext Timesheet CSV export to train the AI assistant's
neural network and enrich LLM prompt context with historical work patterns.

The CSV should be exported from ERPNext's Timesheet list view.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if csvPath == "" {
			return fmt.Errorf("--csv flag is required")
		}

		fmt.Printf("Importing CSV history from: %s\n", csvPath)

		// Parse CSV
		entries, err := ai.ImportCSVHistory(csvPath)
		if err != nil {
			return fmt.Errorf("failed to parse CSV: %w", err)
		}
		fmt.Printf("Parsed %d entries from CSV\n", len(entries))

		if len(entries) == 0 {
			return fmt.Errorf("no entries found in CSV")
		}

		// Build and display summary
		summary := ai.BuildHistorySummary(entries)
		if summary != nil {
			fmt.Printf("\nHistory Summary:\n")
			fmt.Printf("  Period: %s\n", summary.DateRange)
			fmt.Printf("  Total hours: %.0f\n", summary.TotalHours)
			fmt.Printf("  Average daily hours: %.1f\n", summary.AvgDailyHours)
			fmt.Printf("  Customers: %d\n", len(summary.Customers))
			for _, c := range summary.Customers {
				fmt.Printf("    - %s: %.0fh (%d entries)\n", c.Name, c.Hours, c.EntryCount)
			}
			if summary.WorkPatterns != "" {
				fmt.Printf("  Patterns: %s\n", summary.WorkPatterns)
			}
		}

		// Train neural network
		assistant := ai.NewTimesheetAssistant()
		defer assistant.Close()

		err = assistant.TrainFromCSV(csvPath)
		if err != nil {
			return fmt.Errorf("training failed: %w", err)
		}

		status := assistant.GetStatus()
		fmt.Printf("\nTraining complete:\n")
		fmt.Printf("  Vocabulary size: %v\n", status["nn_vocab_size"])
		fmt.Printf("  Documents: %v\n", status["nn_documents"])
		fmt.Printf("  Model trained: %v\n", status["nn_trained"])

		return nil
	},
}

func init() {
	importCmd.Flags().StringVar(&csvPath, "csv", "", "Path to ERPNext Timesheet CSV export")
	rootCmd.AddCommand(importCmd)
}
