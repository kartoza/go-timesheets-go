package ai

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ImportCSVHistory parses an ERPNext Timesheet CSV export into EntryInfo structs.
// The CSV has parent rows (with TS ID, hours, customer, dates) and child rows
// (with individual timelog descriptions). Child rows inherit parent metadata.
func ImportCSVHistory(path string) ([]EntryInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable field count

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}

	// Verify required columns exist
	required := []string{"ID", "Total Working Hours", "Customer", "Start Date", "End Date", "Description (Time Sheets)"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	idCol := colIdx["ID"]
	hoursCol := colIdx["Total Working Hours"]
	customerCol := colIdx["Customer"]
	startCol := colIdx["Start Date"]
	endCol := colIdx["End Date"]
	billableCol := colIdx["Total Billable Hours"]
	descCol := colIdx["Description (Time Sheets)"]

	var entries []EntryInfo

	// Current parent row state
	var parentHours float64
	var parentCustomer string
	var parentStart, parentEnd time.Time
	var parentBillable float64

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip malformed rows
		}

		// Ensure we have enough fields
		if len(record) <= descCol {
			continue
		}

		tsID := strings.TrimSpace(record[idCol])
		desc := strings.TrimSpace(record[descCol])

		if tsID != "" {
			// Parent row — update context
			parentHours, _ = strconv.ParseFloat(strings.TrimSpace(record[hoursCol]), 64)
			parentCustomer = strings.TrimSpace(record[customerCol])
			parentBillable, _ = strconv.ParseFloat(strings.TrimSpace(record[billableCol]), 64)
			parentStart, _ = time.Parse("2006-01-02", strings.TrimSpace(record[startCol]))
			parentEnd, _ = time.Parse("2006-01-02", strings.TrimSpace(record[endCol]))

			// Create entry from parent — only the parent carries the hours
			if desc != "" && desc != "-" {
				entries = append(entries, EntryInfo{
					ProjectName: parentCustomer,
					Description: desc,
					Hours:       parentHours,
					StartTime:   parentStart,
					EndTime:     parentEnd,
					Submitted:   parentBillable > 0,
				})
			} else if parentCustomer != "" {
				entries = append(entries, EntryInfo{
					ProjectName: parentCustomer,
					Description: parentCustomer,
					Hours:       parentHours,
					StartTime:   parentStart,
					EndTime:     parentEnd,
					Submitted:   parentBillable > 0,
				})
			}
		} else {
			// Child row — inherits parent metadata but NOT hours
			// (hours belong to the parent timesheet, not individual logs)
			if desc != "" && desc != "-" {
				entries = append(entries, EntryInfo{
					ProjectName: parentCustomer,
					Description: desc,
					StartTime:   parentStart,
					EndTime:     parentEnd,
					Submitted:   parentBillable > 0,
				})
			}
		}
	}

	return entries, nil
}

// HistorySummary holds a condensed summary of historical timesheet data
// for inclusion in LLM prompts without exceeding context limits.
type HistorySummary struct {
	TotalEntries    int
	DateRange       string
	TotalHours      float64
	AvgDailyHours   float64
	Customers       []CustomerSummary
	TopDescriptions []string // Most common description keywords/phrases
	WorkPatterns    string   // Narrative summary of patterns
}

// CustomerSummary holds aggregated data per customer
type CustomerSummary struct {
	Name       string
	Hours      float64
	DateRange  string
	EntryCount int
}

// BuildHistorySummary creates a condensed summary from CSV entries
// suitable for inclusion in LLM prompts.
func BuildHistorySummary(entries []EntryInfo) *HistorySummary {
	if len(entries) == 0 {
		return nil
	}

	summary := &HistorySummary{
		TotalEntries: len(entries),
	}

	// Date range and total hours
	var minDate, maxDate time.Time
	customerMap := make(map[string]*CustomerSummary)
	wordFreq := make(map[string]int)
	dailyHours := make(map[string]float64) // date -> hours

	for _, e := range entries {
		// Date range
		if !e.StartTime.IsZero() {
			if minDate.IsZero() || e.StartTime.Before(minDate) {
				minDate = e.StartTime
			}
			if maxDate.IsZero() || e.StartTime.After(maxDate) {
				maxDate = e.StartTime
			}
			dateKey := e.StartTime.Format("2006-01-02")
			dailyHours[dateKey] += e.Hours
		}

		summary.TotalHours += e.Hours

		// Customer aggregation
		if e.ProjectName != "" {
			cs, ok := customerMap[e.ProjectName]
			if !ok {
				cs = &CustomerSummary{Name: e.ProjectName}
				customerMap[e.ProjectName] = cs
			}
			cs.Hours += e.Hours
			cs.EntryCount++
		}

		// Word frequency from descriptions
		if e.Description != "" && e.Description != "-" {
			words := tokenizeDescription(e.Description)
			for _, w := range words {
				wordFreq[w]++
			}
		}
	}

	if !minDate.IsZero() && !maxDate.IsZero() {
		summary.DateRange = fmt.Sprintf("%s to %s", minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
	}

	if len(dailyHours) > 0 {
		totalDailyHours := 0.0
		for _, h := range dailyHours {
			totalDailyHours += h
		}
		summary.AvgDailyHours = math.Round(totalDailyHours/float64(len(dailyHours))*10) / 10
	}

	// Sort customers by hours descending
	for _, cs := range customerMap {
		summary.Customers = append(summary.Customers, *cs)
	}
	sort.Slice(summary.Customers, func(i, j int) bool {
		return summary.Customers[i].Hours > summary.Customers[j].Hours
	})
	// Keep top 20 customers
	if len(summary.Customers) > 20 {
		summary.Customers = summary.Customers[:20]
	}

	// Top description keywords
	type wordCount struct {
		word  string
		count int
	}
	var wcs []wordCount
	for w, c := range wordFreq {
		if c >= 3 && len(w) > 3 { // Only words appearing 3+ times, longer than 3 chars
			wcs = append(wcs, wordCount{w, c})
		}
	}
	sort.Slice(wcs, func(i, j int) bool {
		return wcs[i].count > wcs[j].count
	})
	limit := 50
	if len(wcs) < limit {
		limit = len(wcs)
	}
	for i := 0; i < limit; i++ {
		summary.TopDescriptions = append(summary.TopDescriptions, wcs[i].word)
	}

	// Build narrative
	summary.WorkPatterns = buildPatternNarrative(summary, entries)

	return summary
}

// FormatForPrompt formats the history summary as text for LLM prompt inclusion.
func (s *HistorySummary) FormatForPrompt() string {
	if s == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("HISTORICAL WORK PATTERNS (from imported timesheet history):\n")
	sb.WriteString(fmt.Sprintf("- Period: %s (%d entries, %.0f total hours)\n", s.DateRange, s.TotalEntries, s.TotalHours))
	sb.WriteString(fmt.Sprintf("- Average daily hours: %.1f\n", s.AvgDailyHours))

	if len(s.Customers) > 0 {
		sb.WriteString("- Customer/project breakdown:\n")
		for _, c := range s.Customers {
			sb.WriteString(fmt.Sprintf("    %s: %.0fh (%d entries)\n", c.Name, c.Hours, c.EntryCount))
		}
	}

	if len(s.TopDescriptions) > 0 {
		sb.WriteString("- Common work topics: " + strings.Join(s.TopDescriptions[:min(20, len(s.TopDescriptions))], ", ") + "\n")
	}

	if s.WorkPatterns != "" {
		sb.WriteString("- Patterns: " + s.WorkPatterns + "\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

// tokenizeDescription splits a description into lowercase words,
// filtering out common stop words and short tokens.
func tokenizeDescription(desc string) []string {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"this": true, "that": true, "have": true, "been": true, "will": true,
		"are": true, "was": true, "were": true, "has": true, "had": true,
		"not": true, "but": true, "all": true, "can": true, "her": true,
		"his": true, "its": true, "they": true, "them": true, "then": true,
		"than": true, "into": true, "some": true, "such": true, "each": true,
		"which": true, "their": true, "would": true, "about": true, "there": true,
	}

	desc = strings.ToLower(desc)
	// Replace common separators
	for _, sep := range []string{"/", "\\-", ":", ",", ".", "(", ")", "[", "]", "#"} {
		desc = strings.ReplaceAll(desc, sep, " ")
	}

	words := strings.Fields(desc)
	var result []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) > 2 && !stopWords[w] {
			result = append(result, w)
		}
	}
	return result
}

func buildPatternNarrative(summary *HistorySummary, entries []EntryInfo) string {
	var parts []string

	if summary.AvgDailyHours > 0 {
		if summary.AvgDailyHours >= 8 {
			parts = append(parts, "consistently logs full workdays")
		} else if summary.AvgDailyHours >= 6 {
			parts = append(parts, "typically logs 6-8 hours daily")
		} else {
			parts = append(parts, fmt.Sprintf("averages %.1f hours daily", summary.AvgDailyHours))
		}
	}

	if len(summary.Customers) > 0 {
		parts = append(parts, fmt.Sprintf("works across %d customers/projects", len(summary.Customers)))
		parts = append(parts, fmt.Sprintf("most time on %s", summary.Customers[0].Name))
	}

	// Check for git commit descriptions
	gitCount := 0
	meetingCount := 0
	for _, e := range entries {
		desc := strings.ToLower(e.Description)
		if strings.Contains(desc, "git commit") {
			gitCount++
		}
		if strings.Contains(desc, "meeting") || strings.Contains(desc, "call") || strings.Contains(desc, "review") {
			meetingCount++
		}
	}
	if gitCount > 10 {
		parts = append(parts, "frequently logs development work with git commits")
	}
	if meetingCount > 10 {
		parts = append(parts, "regularly attends meetings and reviews")
	}

	return strings.Join(parts, "; ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
