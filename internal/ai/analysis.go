package ai

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// TimesheetAnalyzer provides rule-based analytics over timesheet entries
type TimesheetAnalyzer struct{}

// NewTimesheetAnalyzer creates a new analyzer
func NewTimesheetAnalyzer() *TimesheetAnalyzer {
	return &TimesheetAnalyzer{}
}

// Analyze runs an analytical query against timesheet entries.
// Returns a result if the query matches a known pattern, or nil if not analytical.
func (a *TimesheetAnalyzer) Analyze(query string, entries []EntryInfo) *QueryResult {
	q := strings.ToLower(strings.TrimSpace(query))

	// Try each analytical pattern
	if result := a.analyzeProjectsWorkedOn(q, entries); result != nil {
		return result
	}
	if result := a.analyzeMostTimeSpent(q, entries); result != nil {
		return result
	}
	if result := a.analyzeConsistency(q, entries); result != nil {
		return result
	}
	if result := a.analyzeTotalHours(q, entries); result != nil {
		return result
	}
	if result := a.analyzeActivityBreakdown(q, entries); result != nil {
		return result
	}

	return nil
}

// IsAnalyticalQuery returns true if the query looks like an analytics question
func (a *TimesheetAnalyzer) IsAnalyticalQuery(query string) bool {
	q := strings.ToLower(query)
	analyticalKeywords := []string{
		"which project", "how many hours", "how much time",
		"last week", "this week", "last month", "this month",
		"most time", "least time", "consistent", "consistency",
		"timekeeping", "total hours", "worked on", "spent on",
		"breakdown", "summary", "report", "average",
	}
	for _, kw := range analyticalKeywords {
		if strings.Contains(q, kw) {
			return true
		}
	}
	return false
}

// analyzeProjectsWorkedOn handles "which projects did I work on last week/this week/etc."
func (a *TimesheetAnalyzer) analyzeProjectsWorkedOn(query string, entries []EntryInfo) *QueryResult {
	patterns := []string{
		`(?:which|what) projects? (?:did i|have i|i) (?:work|worked|been working) on (.+)`,
		`(?:what|which) (?:did i|have i) (?:work|worked) on (.+)`,
		`projects? (?:i worked on|from) (.+)`,
	}

	var period string
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(query); len(m) > 1 {
			period = strings.TrimSpace(m[1])
			break
		}
	}
	if period == "" {
		return nil
	}

	start, end := parsePeriod(period)
	filtered := filterEntries(entries, start, end)

	if len(filtered) == 0 {
		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   fmt.Sprintf("Projects worked on %s", period),
				Summary: "No entries found for this period.",
			},
		}
	}

	// Aggregate by project
	projectHours := make(map[string]float64)
	for _, e := range filtered {
		projectHours[e.ProjectName] += e.Hours
	}

	// Sort by hours desc
	type projectTime struct {
		name  string
		hours float64
	}
	var sorted []projectTime
	for name, hours := range projectHours {
		sorted = append(sorted, projectTime{name, hours})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hours > sorted[j].hours
	})

	var details []AnalysisDetail
	totalHours := 0.0
	for _, pt := range sorted {
		details = append(details, AnalysisDetail{
			Label: pt.name,
			Value: fmt.Sprintf("%.1fh", pt.hours),
		})
		totalHours += pt.hours
	}
	details = append(details, AnalysisDetail{
		Label: "Total",
		Value: fmt.Sprintf("%.1fh across %d projects", totalHours, len(sorted)),
	})

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   fmt.Sprintf("Projects worked on %s", period),
			Summary: fmt.Sprintf("Worked on %d projects totalling %.1f hours.", len(sorted), totalHours),
			Details: details,
		},
	}
}

// analyzeMostTimeSpent handles "which project did I spend the most time on"
func (a *TimesheetAnalyzer) analyzeMostTimeSpent(query string, entries []EntryInfo) *QueryResult {
	patterns := []string{
		`(?:which|what) project (?:did i|have i|do i) (?:spend|spent) (?:the )?most time (?:on)?(.*)`,
		`most time (?:spent|on) (?:which|what) project(.*)`,
		`(?:which|what) project (?:took|takes) (?:the )?most (?:time|hours)(.*)`,
	}

	var periodStr string
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(query); len(m) > 0 {
			periodStr = strings.TrimSpace(m[1])
			break
		}
	}

	// Also check for simpler patterns
	if periodStr == "" && !strings.Contains(query, "most time") && !strings.Contains(query, "most hours") {
		return nil
	}

	start, end := parsePeriod(periodStr)
	filtered := filterEntries(entries, start, end)

	if len(filtered) == 0 {
		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   "Most time spent",
				Summary: "No entries found for this period.",
			},
		}
	}

	// Aggregate by project
	projectHours := make(map[string]float64)
	for _, e := range filtered {
		projectHours[e.ProjectName] += e.Hours
	}

	// Sort by hours desc
	type projectTime struct {
		name  string
		hours float64
	}
	var sorted []projectTime
	for name, hours := range projectHours {
		sorted = append(sorted, projectTime{name, hours})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hours > sorted[j].hours
	})

	var details []AnalysisDetail
	for i, pt := range sorted {
		rank := fmt.Sprintf("#%d", i+1)
		details = append(details, AnalysisDetail{
			Label: fmt.Sprintf("%s %s", rank, pt.name),
			Value: fmt.Sprintf("%.1fh", pt.hours),
		})
	}

	top := sorted[0]
	periodLabel := "overall"
	if periodStr != "" {
		periodLabel = periodStr
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   fmt.Sprintf("Time distribution (%s)", periodLabel),
			Summary: fmt.Sprintf("Most time spent on \"%s\" with %.1f hours.", top.name, top.hours),
			Details: details,
		},
	}
}

// analyzeConsistency handles "how consistent was my timekeeping"
func (a *TimesheetAnalyzer) analyzeConsistency(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "consistent") && !strings.Contains(query, "consistency") && !strings.Contains(query, "timekeeping") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   "Timekeeping Consistency",
				Summary: "No entries to analyze.",
			},
		}
	}

	totalEntries := len(entries)
	roundedEntries := 0
	exactHourEntries := 0
	exactHalfHourEntries := 0
	exactQuarterHourEntries := 0
	veryShortEntries := 0
	veryLongEntries := 0

	var durations []float64

	for _, e := range entries {
		hours := e.Hours
		durations = append(durations, hours)

		// Check for exact hour rounding (e.g., 1.0, 2.0, 3.0)
		if hours > 0 && math.Abs(hours-math.Round(hours)) < 0.001 {
			exactHourEntries++
			roundedEntries++
			continue
		}

		// Check for half-hour rounding (e.g., 1.5, 2.5)
		halfHour := hours * 2
		if math.Abs(halfHour-math.Round(halfHour)) < 0.001 {
			exactHalfHourEntries++
			roundedEntries++
			continue
		}

		// Check for quarter-hour rounding (e.g., 1.25, 1.75)
		quarterHour := hours * 4
		if math.Abs(quarterHour-math.Round(quarterHour)) < 0.001 {
			exactQuarterHourEntries++
			roundedEntries++
			continue
		}

		// Check for very short entries (<5 min)
		if hours > 0 && hours < 5.0/60.0 {
			veryShortEntries++
		}

		// Check for very long entries (>8 hours)
		if hours > 8.0 {
			veryLongEntries++
		}
	}

	// Calculate statistics
	roundedPct := 0.0
	if totalEntries > 0 {
		roundedPct = float64(roundedEntries) / float64(totalEntries) * 100
	}

	// Determine consistency rating
	var rating, explanation string
	switch {
	case roundedPct > 80:
		rating = "Low (likely retroactive)"
		explanation = fmt.Sprintf("%.0f%% of entries are rounded to exact intervals, suggesting time is being logged after the fact rather than tracked as-you-work.", roundedPct)
	case roundedPct > 50:
		rating = "Moderate"
		explanation = fmt.Sprintf("%.0f%% of entries are rounded. Some as-you-work tracking with some retroactive logging.", roundedPct)
	case roundedPct > 25:
		rating = "Good"
		explanation = fmt.Sprintf("Only %.0f%% of entries are rounded. Mostly as-you-work tracking.", roundedPct)
	default:
		rating = "Excellent"
		explanation = fmt.Sprintf("Only %.0f%% of entries are rounded. Consistent as-you-work time tracking.", roundedPct)
	}

	// Calculate average and stddev
	avgHours := 0.0
	for _, d := range durations {
		avgHours += d
	}
	avgHours /= float64(len(durations))

	variance := 0.0
	for _, d := range durations {
		diff := d - avgHours
		variance += diff * diff
	}
	variance /= float64(len(durations))
	stddev := math.Sqrt(variance)

	details := []AnalysisDetail{
		{Label: "Rating", Value: rating},
		{Label: "Total entries", Value: fmt.Sprintf("%d", totalEntries)},
		{Label: "Rounded entries", Value: fmt.Sprintf("%d (%.0f%%)", roundedEntries, roundedPct)},
		{Label: "  Exact hours", Value: fmt.Sprintf("%d", exactHourEntries)},
		{Label: "  Half hours", Value: fmt.Sprintf("%d", exactHalfHourEntries)},
		{Label: "  Quarter hours", Value: fmt.Sprintf("%d", exactQuarterHourEntries)},
		{Label: "Avg duration", Value: fmt.Sprintf("%.1fh", avgHours)},
		{Label: "Std deviation", Value: fmt.Sprintf("%.2fh", stddev)},
	}

	if veryShortEntries > 0 {
		details = append(details, AnalysisDetail{
			Label: "Very short (<5min)",
			Value: fmt.Sprintf("%d", veryShortEntries),
		})
	}
	if veryLongEntries > 0 {
		details = append(details, AnalysisDetail{
			Label: "Very long (>8h)",
			Value: fmt.Sprintf("%d", veryLongEntries),
		})
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Timekeeping Consistency Analysis",
			Summary: explanation,
			Details: details,
		},
	}
}

// analyzeTotalHours handles "how many hours did I work" queries
func (a *TimesheetAnalyzer) analyzeTotalHours(query string, entries []EntryInfo) *QueryResult {
	patterns := []string{
		`how (?:many|much) (?:hours|time) (?:did i|have i|i) (?:work|worked|logged|log)(.*)`,
		`total hours?(.*)`,
	}

	var periodStr string
	matched := false
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(query); len(m) > 0 {
			periodStr = strings.TrimSpace(m[1])
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}

	start, end := parsePeriod(periodStr)
	filtered := filterEntries(entries, start, end)

	totalHours := 0.0
	for _, e := range filtered {
		totalHours += e.Hours
	}

	periodLabel := "overall"
	if periodStr != "" {
		periodLabel = periodStr
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   fmt.Sprintf("Total hours (%s)", periodLabel),
			Summary: fmt.Sprintf("Logged %.1f hours across %d entries.", totalHours, len(filtered)),
			Details: []AnalysisDetail{
				{Label: "Total hours", Value: fmt.Sprintf("%.1fh", totalHours)},
				{Label: "Entries", Value: fmt.Sprintf("%d", len(filtered))},
				{Label: "Avg per entry", Value: fmt.Sprintf("%.1fh", safeDiv(totalHours, float64(len(filtered))))},
			},
		},
	}
}

// analyzeActivityBreakdown handles "breakdown by activity" queries
func (a *TimesheetAnalyzer) analyzeActivityBreakdown(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "breakdown") && !strings.Contains(query, "activity") && !strings.Contains(query, "summary") {
		return nil
	}

	if len(entries) == 0 {
		return nil
	}

	// Group by activity
	activityHours := make(map[string]float64)
	for _, e := range entries {
		name := e.ActivityName
		if name == "" {
			name = "(none)"
		}
		activityHours[name] += e.Hours
	}

	type actTime struct {
		name  string
		hours float64
	}
	var sorted []actTime
	for name, hours := range activityHours {
		sorted = append(sorted, actTime{name, hours})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hours > sorted[j].hours
	})

	var details []AnalysisDetail
	totalHours := 0.0
	for _, at := range sorted {
		details = append(details, AnalysisDetail{
			Label: at.name,
			Value: fmt.Sprintf("%.1fh", at.hours),
		})
		totalHours += at.hours
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Activity Breakdown",
			Summary: fmt.Sprintf("%.1f hours across %d activity types.", totalHours, len(sorted)),
			Details: details,
		},
	}
}

// parsePeriod converts natural language time periods to start/end times
func parsePeriod(period string) (time.Time, time.Time) {
	period = strings.TrimSpace(strings.ToLower(period))
	now := time.Now()

	switch {
	case strings.Contains(period, "last week"):
		// Last Monday to Sunday
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		lastMonday := now.AddDate(0, 0, -weekday-6)
		lastSunday := lastMonday.AddDate(0, 0, 7)
		return startOfDay(lastMonday), endOfDay(lastSunday)

	case strings.Contains(period, "this week"):
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -weekday+1)
		return startOfDay(monday), endOfDay(now)

	case strings.Contains(period, "last month"):
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastMonth := firstOfThisMonth.AddDate(0, -1, 0)
		return startOfDay(lastMonth), endOfDay(firstOfThisMonth.AddDate(0, 0, -1))

	case strings.Contains(period, "this month"):
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return startOfDay(firstOfMonth), endOfDay(now)

	case strings.Contains(period, "today"):
		return startOfDay(now), endOfDay(now)

	case strings.Contains(period, "yesterday"):
		yesterday := now.AddDate(0, 0, -1)
		return startOfDay(yesterday), endOfDay(yesterday)

	default:
		// Default: all time (use zero values)
		return time.Time{}, time.Time{}
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

func filterEntries(entries []EntryInfo, start, end time.Time) []EntryInfo {
	if start.IsZero() && end.IsZero() {
		return entries
	}
	var filtered []EntryInfo
	for _, e := range entries {
		if !start.IsZero() && e.StartTime.Before(start) {
			continue
		}
		if !end.IsZero() && e.StartTime.After(end) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
