package models

import "time"

// CalendarEvent represents a calendar event from Google Calendar
type CalendarEvent struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"all_day"`
	Location string    `json:"location,omitempty"`
	ColorHex string    `json:"color_hex,omitempty"`
}

// Duration returns the event duration
func (e *CalendarEvent) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// DurationHours returns the event duration in hours
func (e *CalendarEvent) DurationHours() float64 {
	return e.Duration().Hours()
}

// FormatTimeRange returns a formatted time range string (e.g., "09:00-10:30")
func (e *CalendarEvent) FormatTimeRange() string {
	if e.AllDay {
		return "All day"
	}
	return e.Start.Local().Format("15:04") + "-" + e.End.Local().Format("15:04")
}

// OverlapsWith checks if this event overlaps with a time range
func (e *CalendarEvent) OverlapsWith(start, end time.Time) bool {
	return e.Start.Before(end) && e.End.After(start)
}
