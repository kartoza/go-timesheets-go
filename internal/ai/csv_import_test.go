package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportCSVHistory(t *testing.T) {
	// Create a temp CSV file
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "test.csv")

	content := `"ID","Series","Employee","Total Working Hours","Customer","Start Date","End Date","Total Billable Hours","Base Total Billable Amount","Base Total Billed Amount","ID (Time Sheets)","Description (Time Sheets)"
"TS-2026-00100","TS-.YYYY.-","HR-EMP-00002",2.5,"Acme Corp","2026-01-15","2026-01-15",2.5,100.0,0.0,"abc123","Sprint planning meeting"
"","","","","","","","","","","abc124","Code review session"
"","","","","","","","","","","abc125","-"
"TS-2026-00101","TS-.YYYY.-","HR-EMP-00002",4.0,"","2026-01-16","2026-01-16",0.0,0.0,0.0,"def456","Internal admin work"
"TS-2026-00102","TS-.YYYY.-","HR-EMP-00002",1.0,"Big Client","2026-01-17","2026-01-17",1.0,50.0,0.0,"ghi789","-"
`
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ImportCSVHistory(csvPath)
	if err != nil {
		t.Fatalf("ImportCSVHistory failed: %v", err)
	}

	// Parent "TS-2026-00100" has description "Sprint planning meeting" -> 1 entry
	// Child "abc124" has description "Code review session" -> 1 entry
	// Child "abc125" has "-" -> skipped
	// Parent "TS-2026-00101" has description "Internal admin work", no customer -> 1 entry
	// Parent "TS-2026-00102" has "-" but has customer "Big Client" -> 1 entry (customer as desc)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// Check first entry (parent with customer)
	if entries[0].ProjectName != "Acme Corp" {
		t.Errorf("entry[0] ProjectName = %q, want %q", entries[0].ProjectName, "Acme Corp")
	}
	if entries[0].Description != "Sprint planning meeting" {
		t.Errorf("entry[0] Description = %q, want %q", entries[0].Description, "Sprint planning meeting")
	}
	if entries[0].Hours != 2.5 {
		t.Errorf("entry[0] Hours = %f, want 2.5", entries[0].Hours)
	}

	// Check child entry inherits parent customer but NOT hours
	if entries[1].ProjectName != "Acme Corp" {
		t.Errorf("entry[1] ProjectName = %q, want %q", entries[1].ProjectName, "Acme Corp")
	}
	if entries[1].Description != "Code review session" {
		t.Errorf("entry[1] Description = %q, want %q", entries[1].Description, "Code review session")
	}
	if entries[1].Hours != 0 {
		t.Errorf("entry[1] Hours = %f, want 0 (child rows don't carry hours)", entries[1].Hours)
	}

	// Check parent with no customer
	if entries[2].Description != "Internal admin work" {
		t.Errorf("entry[2] Description = %q, want %q", entries[2].Description, "Internal admin work")
	}

	// Check customer-only entry
	if entries[3].ProjectName != "Big Client" {
		t.Errorf("entry[3] ProjectName = %q, want %q", entries[3].ProjectName, "Big Client")
	}
}

func TestBuildHistorySummary(t *testing.T) {
	entries, _ := ImportCSVHistory("/dev/null") // Will fail but that's ok
	_ = entries

	// Test with synthetic entries
	entries = []EntryInfo{
		{ProjectName: "Acme", Description: "Sprint planning meeting", Hours: 2.0},
		{ProjectName: "Acme", Description: "Code review session", Hours: 3.0},
		{ProjectName: "BigCo", Description: "Git commits and deployment", Hours: 5.0},
	}

	summary := BuildHistorySummary(entries)
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.TotalEntries != 3 {
		t.Errorf("TotalEntries = %d, want 3", summary.TotalEntries)
	}
	if summary.TotalHours != 10.0 {
		t.Errorf("TotalHours = %f, want 10.0", summary.TotalHours)
	}

	prompt := summary.FormatForPrompt()
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestBuildHistorySummaryEmpty(t *testing.T) {
	summary := BuildHistorySummary(nil)
	if summary != nil {
		t.Error("expected nil summary for nil entries")
	}

	summary = BuildHistorySummary([]EntryInfo{})
	if summary != nil {
		t.Error("expected nil summary for empty entries")
	}
}
