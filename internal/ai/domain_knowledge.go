package ai

import (
	_ "embed"
)

//go:embed query_reference.md
var queryReferenceDoc string

// GetQueryReference returns the embedded query reference document content
func GetQueryReference() string {
	return queryReferenceDoc
}

// DomainKnowledge contains expert knowledge about time tracking, timesheet reporting,
// and team productivity analysis. This is injected into LLM prompts to enable the
// AI assistant to respond usefully to a broad range of timesheet-related queries.
const DomainKnowledge = `
DOMAIN EXPERTISE - TIME TRACKING & TIMESHEET MANAGEMENT:

You are an expert in time tracking, timesheet management, and workforce productivity analysis.
You understand the following concepts deeply and can advise users on all of them:

1. TIME TRACKING FUNDAMENTALS:
   - Real-time tracking (start/stop timers) vs retrospective logging (filling in after the fact)
   - Real-time tracking produces more accurate data with irregular durations (e.g., 1h23m)
   - Retrospective logging tends to produce rounded values (1h, 1.5h, 2h) and is less accurate
   - Best practice: track time as-you-work for accuracy, review and adjust at end of day
   - Common pitfall: forgetting to stop timers leads to inflated entries
   - Common pitfall: "parking" time on admin when unsure which project to log to

2. TIMESHEET REPORTING & ANALYSIS:
   - Utilization rate = billable hours / available hours (target typically 70-85% for consulting)
   - Capacity = total available working hours per period (typically 8h/day, 40h/week)
   - Overtime = hours exceeding standard capacity (>8h/day or >40h/week)
   - Undertime = hours below expected capacity, may indicate incomplete logging
   - Time distribution = how hours are spread across projects/activities
   - Focus time = long uninterrupted blocks on a single project (>2h stretches)
   - Context switching = frequent short entries across different projects (productivity cost)

3. TIMESHEET CONSISTENCY INDICATORS:
   - High percentage of rounded durations (1h, 2h) suggests retrospective logging
   - Irregular durations (1h17m, 2h43m) suggest real-time tracking
   - Entries consistently logged at exact same time each day may be batch-entered
   - Gaps between entries may indicate untracked time
   - Overlapping entries indicate data quality issues
   - Very short entries (<5 min) may be accidental or indicate micro-task tracking
   - Very long entries (>8h) are unusual and may indicate forgotten stop timers

4. TEAM TIMESHEET COMPARISON:
   - Compare hours logged per team member to identify under/over-reporters
   - Compare project distribution to see who works on what
   - Compare activity type ratios (development vs meetings vs admin)
   - Identify team members who may need help (high admin ratio, low billable)
   - Track team velocity trends over time
   - Fair comparison requires normalizing for available days (leave, holidays)

5. PROJECT TIME ANALYSIS:
   - Budget burn rate = hours used / hours budgeted
   - Estimated completion = current velocity extrapolated to remaining work
   - Task-level granularity shows where time is actually spent vs planned
   - Compare actual hours to estimates for future estimation accuracy
   - Identify scope creep through increasing hours on "completed" tasks

6. COMMON TIMESHEET QUERIES AND HOW TO ANSWER THEM:
   - "Am I logging enough hours?" - Compare to capacity (8h/day, 40h/week)
   - "Where does my time go?" - Activity/project breakdown with percentages
   - "Am I on track this week?" - Compare logged hours to day-of-week expected pace
   - "Which project needs attention?" - Budget remaining vs burn rate
   - "How do I compare to the team?" - Relative hours and distribution
   - "Is my time tracking accurate?" - Consistency analysis of rounding patterns
   - "What did I work on yesterday/last week?" - Filtered entry listing
   - "How much overtime am I doing?" - Hours above capacity threshold

7. TIMESHEET BEST PRACTICES FOR MANAGERS:
   - Review timesheets weekly, not monthly (catch issues early)
   - Look for patterns, not individual entries
   - High admin ratios may indicate unclear project assignments
   - Zero-hour days need investigation (forgot to log or was on leave?)
   - Encourage real-time tracking over retrospective entry
   - Set clear expectations for granularity (15-min vs hourly resolution)

8. KEY METRICS AND BENCHMARKS:
   - Healthy utilization: 70-85% of available hours on billable/project work
   - Admin overhead: typically 10-20% of total hours
   - Meeting time: should ideally be <25% of working hours
   - Context switching penalty: each switch costs ~15-30 minutes of productivity
   - Ideal entry granularity: 15-minute to 1-hour blocks
   - Weekly expected hours: 40h (8h x 5 days) in standard work week
`

// ExampleQueries provides a curated list of example queries the AI can handle,
// organized by category for display in the UI.
var ExampleQueries = []struct {
	Category string
	Queries  []string
}{
	{
		Category: "Project Matching",
		Queries: []string{
			"Which project for GeoServer Docker?",
			"Where should I log code review time?",
		},
	},
	{
		Category: "Time Analysis",
		Queries: []string{
			"Which projects did I work on last week?",
			"Which project did I spend the most time on?",
			"How many hours did I work today?",
			"What did I work on yesterday?",
		},
	},
	{
		Category: "Tracking Quality",
		Queries: []string{
			"How consistent was my timekeeping?",
			"Am I logging enough hours this week?",
			"Do I have any gaps in my timesheet?",
		},
	},
	{
		Category: "Productivity Insights",
		Queries: []string{
			"What's my utilization rate this month?",
			"How much context switching am I doing?",
			"Compare my hours across projects this month",
			"What's my overtime this week?",
		},
	},
}
