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
	if result := a.analyzeUtilization(q, entries); result != nil {
		return result
	}
	if result := a.analyzeOvertime(q, entries); result != nil {
		return result
	}
	if result := a.analyzeDailyPace(q, entries); result != nil {
		return result
	}
	if result := a.analyzeContextSwitching(q, entries); result != nil {
		return result
	}
	if result := a.analyzeGaps(q, entries); result != nil {
		return result
	}
	if result := a.analyzeUnsubmitted(q, entries); result != nil {
		return result
	}
	if result := a.analyzeWeeklyComparison(q, entries); result != nil {
		return result
	}
	if result := a.analyzeProjectComparison(q, entries); result != nil {
		return result
	}
	if result := a.analyzeLongestPeriod(q, entries); result != nil {
		return result
	}
	if result := a.analyzeMostProductiveDay(q, entries); result != nil {
		return result
	}
	if result := a.analyzeSessionLength(q, entries); result != nil {
		return result
	}
	if result := a.analyzeOldestNewestProject(q, entries); result != nil {
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
		"utilization", "utilisation", "billable",
		"overtime", "over time", "overwork",
		"on track", "pace", "enough hours", "logging enough",
		"context switch", "switching", "focus",
		"gaps", "missing", "untracked",
		"unsubmitted", "not submitted", "pending",
		"compare", "comparison", "week over week", "weekly",
		"yesterday", "today",
		"longest day", "longest week", "longest month",
		"most productive", "productive day",
		"session length", "average session", "avg session",
		"never have time", "never logged", "no time logged",
		"oldest project", "first project", "earliest project", "earliest entry",
		"newest project", "latest project", "most recent project",
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

// analyzeUtilization handles "utilization rate" / "billable hours" queries
func (a *TimesheetAnalyzer) analyzeUtilization(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "utiliz") && !strings.Contains(query, "utilis") && !strings.Contains(query, "billable") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Utilization Rate", Summary: "No entries to analyze."},
		}
	}

	// Group by date to count working days
	dayHours := make(map[string]float64)
	for _, e := range entries {
		day := e.StartTime.Format("2006-01-02")
		dayHours[day] += e.Hours
	}

	totalHours := 0.0
	for _, h := range dayHours {
		totalHours += h
	}
	workingDays := len(dayHours)
	capacity := float64(workingDays) * 8.0
	utilization := safeDiv(totalHours, capacity) * 100

	var rating string
	switch {
	case utilization >= 85:
		rating = "High (may risk burnout)"
	case utilization >= 70:
		rating = "Healthy (target range)"
	case utilization >= 50:
		rating = "Moderate (room to improve)"
	default:
		rating = "Low (may indicate incomplete logging)"
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Utilization Rate",
			Summary: fmt.Sprintf("%.0f%% utilization over %d working days. %s", utilization, workingDays, rating),
			Details: []AnalysisDetail{
				{Label: "Total hours logged", Value: fmt.Sprintf("%.1fh", totalHours)},
				{Label: "Working days", Value: fmt.Sprintf("%d", workingDays)},
				{Label: "Capacity (8h/day)", Value: fmt.Sprintf("%.0fh", capacity)},
				{Label: "Utilization", Value: fmt.Sprintf("%.0f%%", utilization)},
				{Label: "Rating", Value: rating},
				{Label: "Avg hours/day", Value: fmt.Sprintf("%.1fh", safeDiv(totalHours, float64(workingDays)))},
			},
		},
	}
}

// analyzeOvertime handles "overtime" / "overwork" queries
func (a *TimesheetAnalyzer) analyzeOvertime(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "overtime") && !strings.Contains(query, "over time") && !strings.Contains(query, "overwork") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Overtime Analysis", Summary: "No entries to analyze."},
		}
	}

	dayHours := make(map[string]float64)
	for _, e := range entries {
		day := e.StartTime.Format("2006-01-02")
		dayHours[day] += e.Hours
	}

	overtimeDays := 0
	totalOvertime := 0.0
	var details []AnalysisDetail

	// Sort days
	var days []string
	for d := range dayHours {
		days = append(days, d)
	}
	sort.Strings(days)

	for _, day := range days {
		h := dayHours[day]
		if h > 8.0 {
			overtimeDays++
			ot := h - 8.0
			totalOvertime += ot
			details = append(details, AnalysisDetail{
				Label: day,
				Value: fmt.Sprintf("%.1fh logged (%.1fh overtime)", h, ot),
			})
		}
	}

	summary := fmt.Sprintf("%d days with overtime (>8h), totalling %.1fh extra across %d working days.",
		overtimeDays, totalOvertime, len(dayHours))
	if overtimeDays == 0 {
		summary = "No overtime detected. All days are within 8-hour capacity."
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Overtime Analysis",
			Summary: summary,
			Details: details,
		},
	}
}

// analyzeDailyPace handles "am I on track" / "enough hours" / "pace" queries
func (a *TimesheetAnalyzer) analyzeDailyPace(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "on track") && !strings.Contains(query, "pace") &&
		!strings.Contains(query, "enough hours") && !strings.Contains(query, "logging enough") {
		return nil
	}

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -weekday+1)
	weekStart := startOfDay(monday)

	filtered := filterEntries(entries, weekStart, endOfDay(now))

	totalHours := 0.0
	for _, e := range filtered {
		totalHours += e.Hours
	}

	// Expected hours based on day of week (Mon=1 through current day)
	workdaysSoFar := weekday
	if workdaysSoFar > 5 {
		workdaysSoFar = 5
	}
	expectedHours := float64(workdaysSoFar) * 8.0
	deficit := expectedHours - totalHours

	var status string
	switch {
	case deficit <= 0:
		status = "Ahead of pace"
	case deficit < 2:
		status = "Slightly behind"
	case deficit < 8:
		status = "Behind pace"
	default:
		status = "Significantly behind"
	}

	remainingDays := 5 - workdaysSoFar
	remainingNeeded := 40.0 - totalHours
	if remainingNeeded < 0 {
		remainingNeeded = 0
	}

	details := []AnalysisDetail{
		{Label: "This week so far", Value: fmt.Sprintf("%.1fh", totalHours)},
		{Label: "Expected by now", Value: fmt.Sprintf("%.0fh (%d days x 8h)", expectedHours, workdaysSoFar)},
		{Label: "Status", Value: status},
	}
	if remainingDays > 0 {
		details = append(details, AnalysisDetail{
			Label: "Remaining to 40h",
			Value: fmt.Sprintf("%.1fh over %d days (%.1fh/day)", remainingNeeded, remainingDays, safeDiv(remainingNeeded, float64(remainingDays))),
		})
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Weekly Pace Check",
			Summary: fmt.Sprintf("%.1fh logged this week vs %.0fh expected. %s.", totalHours, expectedHours, status),
			Details: details,
		},
	}
}

// analyzeContextSwitching handles "context switching" / "focus" queries
func (a *TimesheetAnalyzer) analyzeContextSwitching(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "context switch") && !strings.Contains(query, "switching") && !strings.Contains(query, "focus") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Context Switching Analysis", Summary: "No entries to analyze."},
		}
	}

	// Group entries by day, count unique projects per day
	dayProjects := make(map[string]map[string]bool)
	dayEntries := make(map[string]int)
	for _, e := range entries {
		day := e.StartTime.Format("2006-01-02")
		if dayProjects[day] == nil {
			dayProjects[day] = make(map[string]bool)
		}
		dayProjects[day][e.ProjectName] = true
		dayEntries[day]++
	}

	totalSwitches := 0
	highSwitchDays := 0
	var details []AnalysisDetail

	var days []string
	for d := range dayProjects {
		days = append(days, d)
	}
	sort.Strings(days)

	// Show last 10 days
	start := 0
	if len(days) > 10 {
		start = len(days) - 10
	}

	for _, day := range days[start:] {
		projects := len(dayProjects[day])
		entries := dayEntries[day]
		totalSwitches += projects - 1
		if projects > 3 {
			highSwitchDays++
		}
		details = append(details, AnalysisDetail{
			Label: day,
			Value: fmt.Sprintf("%d projects, %d entries", projects, entries),
		})
	}

	avgProjectsPerDay := 0.0
	for _, ps := range dayProjects {
		avgProjectsPerDay += float64(len(ps))
	}
	avgProjectsPerDay = safeDiv(avgProjectsPerDay, float64(len(dayProjects)))

	var rating string
	switch {
	case avgProjectsPerDay <= 2:
		rating = "Good focus (avg 1-2 projects/day)"
	case avgProjectsPerDay <= 3:
		rating = "Moderate switching (avg 2-3 projects/day)"
	default:
		rating = "High switching (avg >3 projects/day) - consider blocking time"
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Context Switching Analysis",
			Summary: fmt.Sprintf("Avg %.1f projects/day. %s", avgProjectsPerDay, rating),
			Details: details,
		},
	}
}

// analyzeGaps handles "gaps" / "missing" / "untracked" queries
func (a *TimesheetAnalyzer) analyzeGaps(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "gap") && !strings.Contains(query, "missing") && !strings.Contains(query, "untracked") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Timesheet Gaps", Summary: "No entries to analyze."},
		}
	}

	// Find days with low hours in the recent 2 weeks
	now := time.Now()
	twoWeeksAgo := now.AddDate(0, 0, -14)
	filtered := filterEntries(entries, startOfDay(twoWeeksAgo), endOfDay(now))

	dayHours := make(map[string]float64)
	for _, e := range filtered {
		day := e.StartTime.Format("2006-01-02")
		dayHours[day] += e.Hours
	}

	var details []AnalysisDetail
	gapDays := 0

	// Check each weekday in the last 2 weeks
	for d := twoWeeksAgo; !d.After(now); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		dayStr := d.Format("2006-01-02")
		hours := dayHours[dayStr]
		if hours < 4.0 {
			gapDays++
			if hours == 0 {
				details = append(details, AnalysisDetail{
					Label: dayStr + " (" + d.Weekday().String()[:3] + ")",
					Value: "No entries",
				})
			} else {
				details = append(details, AnalysisDetail{
					Label: dayStr + " (" + d.Weekday().String()[:3] + ")",
					Value: fmt.Sprintf("%.1fh (below 4h threshold)", hours),
				})
			}
		}
	}

	summary := fmt.Sprintf("Found %d days with <4h logged in the last 2 weeks.", gapDays)
	if gapDays == 0 {
		summary = "No gaps detected in the last 2 weeks. All weekdays have 4+ hours logged."
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Timesheet Gaps (Last 2 Weeks)",
			Summary: summary,
			Details: details,
		},
	}
}

// analyzeUnsubmitted handles "unsubmitted" / "not submitted" / "pending" queries
func (a *TimesheetAnalyzer) analyzeUnsubmitted(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "unsubmitted") && !strings.Contains(query, "not submitted") &&
		!strings.Contains(query, "pending") {
		return nil
	}

	unsubmitted := 0
	unsubmittedHours := 0.0
	projectHours := make(map[string]float64)

	for _, e := range entries {
		if !e.Submitted {
			unsubmitted++
			unsubmittedHours += e.Hours
			projectHours[e.ProjectName] += e.Hours
		}
	}

	var details []AnalysisDetail
	type ph struct {
		name  string
		hours float64
	}
	var sorted []ph
	for n, h := range projectHours {
		sorted = append(sorted, ph{n, h})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].hours > sorted[j].hours })
	for _, p := range sorted {
		details = append(details, AnalysisDetail{
			Label: p.name,
			Value: fmt.Sprintf("%.1fh", p.hours),
		})
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Unsubmitted Entries",
			Summary: fmt.Sprintf("%d entries (%.1fh) not yet submitted across %d projects.", unsubmitted, unsubmittedHours, len(projectHours)),
			Details: details,
		},
	}
}

// analyzeWeeklyComparison handles "week over week" / "weekly comparison" queries
func (a *TimesheetAnalyzer) analyzeWeeklyComparison(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "week over week") && !strings.Contains(query, "weekly") &&
		!(strings.Contains(query, "compare") && strings.Contains(query, "week")) {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Weekly Comparison", Summary: "No entries to analyze."},
		}
	}

	// Group by ISO week
	weekHours := make(map[string]float64)
	weekEntries := make(map[string]int)
	for _, e := range entries {
		year, week := e.StartTime.ISOWeek()
		key := fmt.Sprintf("%d-W%02d", year, week)
		weekHours[key] += e.Hours
		weekEntries[key]++
	}

	var weeks []string
	for w := range weekHours {
		weeks = append(weeks, w)
	}
	sort.Strings(weeks)

	// Show last 8 weeks
	start := 0
	if len(weeks) > 8 {
		start = len(weeks) - 8
	}

	var details []AnalysisDetail
	var prevHours float64
	for i, w := range weeks[start:] {
		h := weekHours[w]
		trend := ""
		if i > 0 {
			diff := h - prevHours
			if diff > 0 {
				trend = fmt.Sprintf(" (+%.1fh)", diff)
			} else if diff < 0 {
				trend = fmt.Sprintf(" (%.1fh)", diff)
			}
		}
		details = append(details, AnalysisDetail{
			Label: w,
			Value: fmt.Sprintf("%.1fh, %d entries%s", h, weekEntries[w], trend),
		})
		prevHours = h
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Weekly Comparison",
			Summary: fmt.Sprintf("Showing %d weeks of data.", len(weeks[start:])),
			Details: details,
		},
	}
}

// analyzeProjectComparison handles "compare hours across projects" queries
func (a *TimesheetAnalyzer) analyzeProjectComparison(query string, entries []EntryInfo) *QueryResult {
	if !(strings.Contains(query, "compare") && strings.Contains(query, "project")) {
		return nil
	}

	// Determine period
	periodStr := ""
	for _, p := range []string{"this month", "last month", "this week", "last week"} {
		if strings.Contains(query, p) {
			periodStr = p
			break
		}
	}

	start, end := parsePeriod(periodStr)
	filtered := filterEntries(entries, start, end)

	if len(filtered) == 0 {
		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   "Project Comparison",
				Summary: "No entries found for this period.",
			},
		}
	}

	projectHours := make(map[string]float64)
	projectEntries := make(map[string]int)
	for _, e := range filtered {
		projectHours[e.ProjectName] += e.Hours
		projectEntries[e.ProjectName]++
	}

	type pt struct {
		name    string
		hours   float64
		entries int
	}
	var sorted []pt
	totalHours := 0.0
	for n, h := range projectHours {
		sorted = append(sorted, pt{n, h, projectEntries[n]})
		totalHours += h
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].hours > sorted[j].hours })

	var details []AnalysisDetail
	for _, p := range sorted {
		pct := safeDiv(p.hours, totalHours) * 100
		details = append(details, AnalysisDetail{
			Label: p.name,
			Value: fmt.Sprintf("%.1fh (%.0f%%), %d entries", p.hours, pct, p.entries),
		})
	}

	label := "overall"
	if periodStr != "" {
		label = periodStr
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   fmt.Sprintf("Project Comparison (%s)", label),
			Summary: fmt.Sprintf("%.1fh across %d projects.", totalHours, len(sorted)),
			Details: details,
		},
	}
}

// analyzeLongestPeriod handles "longest day/week/month" queries
func (a *TimesheetAnalyzer) analyzeLongestPeriod(query string, entries []EntryInfo) *QueryResult {
	isDay := strings.Contains(query, "longest day")
	isWeek := strings.Contains(query, "longest week")
	isMonth := strings.Contains(query, "longest month")

	if !isDay && !isWeek && !isMonth {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Longest Period", Summary: "No entries to analyze."},
		}
	}

	if isDay {
		dayHours := make(map[string]float64)
		for _, e := range entries {
			day := e.StartTime.Format("2006-01-02")
			dayHours[day] += e.Hours
		}
		maxDay, maxH := "", 0.0
		for d, h := range dayHours {
			if h > maxH {
				maxDay, maxH = d, h
			}
		}

		// Top 5 days
		type dh struct {
			day   string
			hours float64
		}
		var sorted []dh
		for d, h := range dayHours {
			sorted = append(sorted, dh{d, h})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].hours > sorted[j].hours })

		var details []AnalysisDetail
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, s := range sorted[:limit] {
			details = append(details, AnalysisDetail{Label: s.day, Value: fmt.Sprintf("%.1fh", s.hours)})
		}

		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   "Longest Days",
				Summary: fmt.Sprintf("Longest day was %s with %.1fh logged.", maxDay, maxH),
				Details: details,
			},
		}
	}

	if isWeek {
		weekHours := make(map[string]float64)
		for _, e := range entries {
			year, week := e.StartTime.ISOWeek()
			key := fmt.Sprintf("%d-W%02d", year, week)
			weekHours[key] += e.Hours
		}

		type wh struct {
			week  string
			hours float64
		}
		var sorted []wh
		for w, h := range weekHours {
			sorted = append(sorted, wh{w, h})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].hours > sorted[j].hours })

		var details []AnalysisDetail
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, s := range sorted[:limit] {
			details = append(details, AnalysisDetail{Label: s.week, Value: fmt.Sprintf("%.1fh", s.hours)})
		}

		return &QueryResult{
			Type: ResultAnalysis,
			Analysis: &AnalysisResult{
				Title:   "Longest Weeks",
				Summary: fmt.Sprintf("Longest week was %s with %.1fh logged.", sorted[0].week, sorted[0].hours),
				Details: details,
			},
		}
	}

	// isMonth
	monthHours := make(map[string]float64)
	for _, e := range entries {
		key := e.StartTime.Format("2006-01")
		monthHours[key] += e.Hours
	}

	type mh struct {
		month string
		hours float64
	}
	var sorted []mh
	for m, h := range monthHours {
		sorted = append(sorted, mh{m, h})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].hours > sorted[j].hours })

	var details []AnalysisDetail
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, s := range sorted[:limit] {
		details = append(details, AnalysisDetail{Label: s.month, Value: fmt.Sprintf("%.1fh", s.hours)})
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Longest Months",
			Summary: fmt.Sprintf("Longest month was %s with %.1fh logged.", sorted[0].month, sorted[0].hours),
			Details: details,
		},
	}
}

// analyzeMostProductiveDay handles "most productive day" queries
func (a *TimesheetAnalyzer) analyzeMostProductiveDay(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "most productive") && !strings.Contains(query, "productive day") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Most Productive Day", Summary: "No entries to analyze."},
		}
	}

	// Most productive = most hours + most entries (weighted)
	dayHours := make(map[string]float64)
	dayEntries := make(map[string]int)
	dayProjects := make(map[string]map[string]bool)
	for _, e := range entries {
		day := e.StartTime.Format("2006-01-02")
		dayHours[day] += e.Hours
		dayEntries[day]++
		if dayProjects[day] == nil {
			dayProjects[day] = make(map[string]bool)
		}
		dayProjects[day][e.ProjectName] = true
	}

	// Score = hours (primary), entries as tiebreaker
	type dayScore struct {
		day      string
		hours    float64
		entries  int
		projects int
	}
	var scored []dayScore
	for d, h := range dayHours {
		scored = append(scored, dayScore{d, h, dayEntries[d], len(dayProjects[d])})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].hours != scored[j].hours {
			return scored[i].hours > scored[j].hours
		}
		return scored[i].entries > scored[j].entries
	})

	var details []AnalysisDetail
	limit := 5
	if len(scored) < limit {
		limit = len(scored)
	}
	for _, s := range scored[:limit] {
		details = append(details, AnalysisDetail{
			Label: s.day,
			Value: fmt.Sprintf("%.1fh, %d entries, %d projects", s.hours, s.entries, s.projects),
		})
	}

	top := scored[0]
	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Most Productive Days",
			Summary: fmt.Sprintf("Most productive day was %s with %.1fh across %d projects.", top.day, top.hours, top.projects),
			Details: details,
		},
	}
}

// analyzeSessionLength handles "average session length" queries
func (a *TimesheetAnalyzer) analyzeSessionLength(query string, entries []EntryInfo) *QueryResult {
	if !strings.Contains(query, "session length") && !strings.Contains(query, "average session") &&
		!strings.Contains(query, "avg session") {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Session Length Analysis", Summary: "No entries to analyze."},
		}
	}

	var durations []float64
	for _, e := range entries {
		if e.Hours > 0 {
			durations = append(durations, e.Hours)
		}
	}

	if len(durations) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Session Length Analysis", Summary: "No entries with recorded duration."},
		}
	}

	// Compute stats
	sort.Float64s(durations)
	total := 0.0
	for _, d := range durations {
		total += d
	}
	avg := total / float64(len(durations))
	median := durations[len(durations)/2]
	shortest := durations[0]
	longest := durations[len(durations)-1]

	// Bucket distribution
	under30m, under1h, under2h, under4h, over4h := 0, 0, 0, 0, 0
	for _, d := range durations {
		switch {
		case d < 0.5:
			under30m++
		case d < 1.0:
			under1h++
		case d < 2.0:
			under2h++
		case d < 4.0:
			under4h++
		default:
			over4h++
		}
	}

	details := []AnalysisDetail{
		{Label: "Average session", Value: fmt.Sprintf("%.1fh", avg)},
		{Label: "Median session", Value: fmt.Sprintf("%.1fh", median)},
		{Label: "Shortest", Value: fmt.Sprintf("%.1fh", shortest)},
		{Label: "Longest", Value: fmt.Sprintf("%.1fh", longest)},
		{Label: "Total sessions", Value: fmt.Sprintf("%d", len(durations))},
		{Label: "< 30 min", Value: fmt.Sprintf("%d (%.0f%%)", under30m, safeDiv(float64(under30m), float64(len(durations)))*100)},
		{Label: "30min - 1h", Value: fmt.Sprintf("%d (%.0f%%)", under1h, safeDiv(float64(under1h), float64(len(durations)))*100)},
		{Label: "1h - 2h", Value: fmt.Sprintf("%d (%.0f%%)", under2h, safeDiv(float64(under2h), float64(len(durations)))*100)},
		{Label: "2h - 4h", Value: fmt.Sprintf("%d (%.0f%%)", under4h, safeDiv(float64(under4h), float64(len(durations)))*100)},
		{Label: "> 4h", Value: fmt.Sprintf("%d (%.0f%%)", over4h, safeDiv(float64(over4h), float64(len(durations)))*100)},
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   "Session Length Analysis",
			Summary: fmt.Sprintf("Average session is %.1fh (median %.1fh) across %d entries.", avg, median, len(durations)),
			Details: details,
		},
	}
}

// analyzeOldestNewestProject handles "oldest/first/earliest project" and "newest/latest/most recent project" queries
func (a *TimesheetAnalyzer) analyzeOldestNewestProject(query string, entries []EntryInfo) *QueryResult {
	isOldest := strings.Contains(query, "oldest") || strings.Contains(query, "first project") ||
		strings.Contains(query, "earliest")
	isNewest := strings.Contains(query, "newest") || strings.Contains(query, "latest project") ||
		strings.Contains(query, "most recent project")

	if !isOldest && !isNewest {
		return nil
	}

	if len(entries) == 0 {
		return &QueryResult{
			Type:     ResultAnalysis,
			Analysis: &AnalysisResult{Title: "Project Timeline", Summary: "No entries to analyze."},
		}
	}

	// Find first and last entry per project
	type projectTimeline struct {
		name       string
		firstEntry time.Time
		lastEntry  time.Time
		totalHours float64
		entryCount int
	}
	projects := make(map[string]*projectTimeline)
	for _, e := range entries {
		pt, ok := projects[e.ProjectName]
		if !ok {
			pt = &projectTimeline{
				name:       e.ProjectName,
				firstEntry: e.StartTime,
				lastEntry:  e.StartTime,
			}
			projects[e.ProjectName] = pt
		}
		if e.StartTime.Before(pt.firstEntry) {
			pt.firstEntry = e.StartTime
		}
		if e.StartTime.After(pt.lastEntry) {
			pt.lastEntry = e.StartTime
		}
		pt.totalHours += e.Hours
		pt.entryCount++
	}

	var sorted []*projectTimeline
	for _, pt := range projects {
		sorted = append(sorted, pt)
	}

	if isOldest {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].firstEntry.Before(sorted[j].firstEntry)
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].lastEntry.After(sorted[j].lastEntry)
		})
	}

	var details []AnalysisDetail
	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, pt := range sorted[:limit] {
		dateLabel := pt.firstEntry.Format("2006-01-02")
		if isNewest {
			dateLabel = pt.lastEntry.Format("2006-01-02")
		}
		details = append(details, AnalysisDetail{
			Label: pt.name,
			Value: fmt.Sprintf("%s, %.1fh total, %d entries", dateLabel, pt.totalHours, pt.entryCount),
		})
	}

	title := "Oldest Projects"
	top := sorted[0]
	summary := fmt.Sprintf("Oldest project is \"%s\" with first entry on %s (%.1fh total across %d entries).",
		top.name, top.firstEntry.Format("2006-01-02"), top.totalHours, top.entryCount)
	if isNewest {
		title = "Most Recent Projects"
		summary = fmt.Sprintf("Most recent project is \"%s\" with latest entry on %s (%.1fh total across %d entries).",
			top.name, top.lastEntry.Format("2006-01-02"), top.totalHours, top.entryCount)
	}

	return &QueryResult{
		Type: ResultAnalysis,
		Analysis: &AnalysisResult{
			Title:   title,
			Summary: summary,
			Details: details,
		},
	}
}
