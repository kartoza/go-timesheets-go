package ai

import (
	"time"
)

// ResultType indicates what kind of response the AI produced
type ResultType string

const (
	ResultMatches  ResultType = "matches"
	ResultAnalysis ResultType = "analysis"
	ResultText     ResultType = "text"
	ResultError    ResultType = "error"
)

// QueryResult represents the AI assistant's response
type QueryResult struct {
	Type     ResultType
	Matches  []TimesheetMatch
	Analysis *AnalysisResult
	Text     string // Plain text response for display
}

// TimesheetMatch represents a matched project/task/activity combination
type TimesheetMatch struct {
	ProjectID    int
	ProjectName  string
	TaskID       int
	TaskName     string
	ActivityID   int
	ActivityName string
	Confidence   float64
	Reason       string
}

// AnalysisResult holds the result of a timesheet analytics query
type AnalysisResult struct {
	Title   string
	Summary string
	Details []AnalysisDetail
}

// AnalysisDetail holds a single line of analysis output
type AnalysisDetail struct {
	Label string
	Value string
}

// TimesheetContext holds the data available for the AI to reason about
type TimesheetContext struct {
	Projects   []ProjectInfo
	Activities []ActivityInfo
	Entries    []EntryInfo // Recent timesheet entries for analytics
}

// ProjectInfo holds project data with its tasks
type ProjectInfo struct {
	ID    int
	Name  string
	Tasks []TaskInfo
}

// TaskInfo holds task data
type TaskInfo struct {
	ID   int
	Name string
}

// ActivityInfo holds activity data
type ActivityInfo struct {
	ID   int
	Name string
}

// EntryInfo holds a single timesheet entry for analysis
type EntryInfo struct {
	ProjectName  string
	TaskName     string
	ActivityName string
	StartTime    time.Time
	EndTime      time.Time
	Hours        float64
	Description  string
	Submitted    bool
}
