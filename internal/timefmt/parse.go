// Package timefmt centralises the parsing of user-entered time strings.
//
// The app stores and displays times as canonical 24-hour "HH:MM" but accepts
// a range of input forms so users don't have to think about format:
//
//	"17:30"   → "17:30"   24-hour HH:MM
//	"17h30"   → "17:30"   continental h-separator
//	"17h"     → "17:00"   hour shorthand
//	"5:55pm"  → "17:55"   12-hour with AM/PM (case-insensitive, optional space)
//	"5pm"     → "17:00"   12-hour shorthand
//	"5 pm"    → "17:00"   AM/PM may have whitespace
//	"12:00am" → "00:00"   midnight
//	"12:00pm" → "12:00"   noon
//	"9"       → "09:00"   bare hour, treated as 24-hour
//
// Anything else returns an error.
package timefmt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseFlexibleDateTime combines a date in "YYYY-MM-DD" form with a user-entered
// time (any form NormalizeTimeInput accepts) into a time.Time in loc. Returns
// an error if either component is invalid or if the time is empty.
func ParseFlexibleDateTime(dateStr, timeStr string, loc *time.Location) (time.Time, error) {
	norm, err := NormalizeTimeInput(timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time: %w", err)
	}
	if norm == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	return time.ParseInLocation("2006-01-02 15:04", dateStr+" "+norm, loc)
}

// NormalizeTimeInput converts a flexible user-entered time string to the
// canonical "HH:MM" 24-hour form. Empty input returns "" without an error so
// callers can decide whether an empty time is acceptable.
func NormalizeTimeInput(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}

	// Detect am/pm suffix (case-insensitive, optional whitespace before it).
	lower := strings.ToLower(s)
	var rest string
	var ampm string
	switch {
	case strings.HasSuffix(lower, "pm"):
		rest = strings.TrimSpace(strings.TrimSuffix(lower, "pm"))
		ampm = "pm"
	case strings.HasSuffix(lower, "am"):
		rest = strings.TrimSpace(strings.TrimSuffix(lower, "am"))
		ampm = "am"
	default:
		rest = lower
	}

	h, m, err := parseHourMinute(rest)
	if err != nil {
		return "", err
	}

	switch ampm {
	case "pm":
		if h < 1 || h > 12 {
			return "", fmt.Errorf("hour %d is invalid for PM (expected 1-12)", h)
		}
		if h != 12 {
			h += 12
		}
	case "am":
		if h < 1 || h > 12 {
			return "", fmt.Errorf("hour %d is invalid for AM (expected 1-12)", h)
		}
		if h == 12 {
			h = 0 // 12am = 00:00
		}
	default:
		if h < 0 || h > 23 {
			return "", fmt.Errorf("hour %d out of range (00-23)", h)
		}
	}

	if m < 0 || m > 59 {
		return "", fmt.Errorf("minute %d out of range (00-59)", m)
	}

	return fmt.Sprintf("%02d:%02d", h, m), nil
}

// hourMinuteRe matches an hour, optionally followed by `:` or `h` and zero to
// two digits of minutes. Examples that match: "5", "17", "5:55", "17h30",
// "17h", "5:".
var hourMinuteRe = regexp.MustCompile(`^(\d{1,2})(?:[:h](\d{0,2}))?$`)

func parseHourMinute(s string) (int, int, error) {
	matches := hourMinuteRe.FindStringSubmatch(s)
	if matches == nil {
		return 0, 0, fmt.Errorf("could not parse time %q", s)
	}
	h, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour %q", matches[1])
	}
	m := 0
	if len(matches) > 2 && matches[2] != "" {
		m, err = strconv.Atoi(matches[2])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid minute %q", matches[2])
		}
	}
	return h, m, nil
}
