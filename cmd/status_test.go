package cmd

import (
	"testing"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "00:00"},
		{"seconds only", 45 * time.Second, "00:45"},
		{"minutes only", 5 * time.Minute, "05:00"},
		{"minutes and seconds", 5*time.Minute + 30*time.Second, "05:30"},
		{"one hour", time.Hour, "1:00:00"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2:30:00"},
		{"hours, minutes, seconds", 3*time.Hour + 45*time.Minute + 15*time.Second, "3:45:15"},
		{"many hours", 12*time.Hour + 5*time.Minute + 30*time.Second, "12:05:30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestIsActiveEntry(t *testing.T) {
	tests := []struct {
		name     string
		entry    *api.TimelogEntry
		expected bool
	}{
		{
			name: "empty to_time is active",
			entry: &api.TimelogEntry{
				FromTime: "2024-01-01T09:00:00Z",
				ToTime:   "",
			},
			expected: true,
		},
		{
			name: "same from and to time is active",
			entry: &api.TimelogEntry{
				FromTime: "2024-01-01T09:00:00Z",
				ToTime:   "2024-01-01T09:00:00Z",
			},
			expected: true,
		},
		{
			name: "different from and to time is not active",
			entry: &api.TimelogEntry{
				FromTime: "2024-01-01T09:00:00Z",
				ToTime:   "2024-01-01T17:00:00Z",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isActiveEntry(tt.entry)
			if result != tt.expected {
				t.Errorf("isActiveEntry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindMatchingWorkspace(t *testing.T) {
	assocs := models.NewWorkspaceAssociations()
	// Put task-specific match first to test priority
	assocs.Associations[0] = models.WorkspaceAssociation{
		WorkspaceNumber: 1,
		WorkspaceName:   "Testing",
		ProjectID:       100,
		TaskID:          200,
	}
	assocs.Associations[1] = models.WorkspaceAssociation{
		WorkspaceNumber: 2,
		WorkspaceName:   "Dev",
		ProjectID:       100,
	}
	assocs.Associations[2] = models.WorkspaceAssociation{
		WorkspaceNumber: 3,
		WorkspaceName:   "Other",
		ProjectID:       300,
	}

	tests := []struct {
		name               string
		projectID          int
		taskID             int
		expectedName       string
		expectedNumber     int
	}{
		{
			name:           "match project and task",
			projectID:      100,
			taskID:         200,
			expectedName:   "Testing",
			expectedNumber: 1,
		},
		{
			name:           "match project only (no task specified)",
			projectID:      100,
			taskID:         0,
			expectedName:   "Dev",
			expectedNumber: 2,
		},
		{
			name:           "no match",
			projectID:      999,
			taskID:         0,
			expectedName:   "",
			expectedNumber: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, number := findMatchingWorkspace(assocs, tt.projectID, tt.taskID)
			if name != tt.expectedName {
				t.Errorf("findMatchingWorkspace() name = %s, want %s", name, tt.expectedName)
			}
			if number != tt.expectedNumber {
				t.Errorf("findMatchingWorkspace() number = %d, want %d", number, tt.expectedNumber)
			}
		})
	}

	// Test nil assocs
	name, number := findMatchingWorkspace(nil, 100, 0)
	if name != "" || number != 0 {
		t.Error("findMatchingWorkspace with nil should return empty values")
	}
}

func TestCreateWaybarStatusFromInfo(t *testing.T) {
	// Test idle state
	t.Run("idle state", func(t *testing.T) {
		info := StatusInfo{
			IsRecording: false,
			DailyHours:  4.5,
		}

		status := createWaybarStatusFromInfo(info)

		if status.Class != "idle" {
			t.Errorf("Expected class 'idle', got '%s'", status.Class)
		}
		if status.Alt != "idle" {
			t.Errorf("Expected alt 'idle', got '%s'", status.Alt)
		}
		// Check for Nerd Font pause icon (nf-md-pause)
		if status.Text != "󰏤 4.5h" {
			t.Errorf("Unexpected text: %s", status.Text)
		}
	})

	// Test recording state
	t.Run("recording state", func(t *testing.T) {
		info := StatusInfo{
			IsRecording:     true,
			ProjectName:     "Test Project",
			TaskName:        "Test Task",
			ActivityName:    "Coding",
			StartTime:       time.Now().Add(-time.Hour),
			CurrentDuration: time.Hour,
			TaskHours:       5.5,
			DailyHours:      8.0,
			RepoName:        "kartoza/test",
			WorkspaceName:   "Development",
			WorkspaceNumber: 1,
		}

		status := createWaybarStatusFromInfo(info)

		if status.Class != "recording" {
			t.Errorf("Expected class 'recording', got '%s'", status.Class)
		}
		if status.Alt != "recording" {
			t.Errorf("Expected alt 'recording', got '%s'", status.Alt)
		}

		// Check tooltip contains expected information
		if len(status.Tooltip) == 0 {
			t.Error("Tooltip should not be empty")
		}
	})
}
