package timefmt

import "testing"

func TestNormalizeTimeInput(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		// Canonical 24-hour
		{"17:30", "17:30", false},
		{"00:00", "00:00", false},
		{"23:59", "23:59", false},
		{"9:05", "09:05", false}, // single-digit hour padded

		// 24-hour bare hour
		{"9", "09:00", false},
		{"17", "17:00", false},
		{"0", "00:00", false},
		{"23", "23:00", false},

		// Continental h-separator
		{"17h30", "17:30", false},
		{"17h00", "17:00", false},
		{"17h", "17:00", false},
		{"5h05", "05:05", false},

		// 12-hour with am/pm
		{"5pm", "17:00", false},
		{"5PM", "17:00", false},
		{"5 pm", "17:00", false},
		{"5:55pm", "17:55", false},
		{"5:55 PM", "17:55", false},
		{"12pm", "12:00", false}, // noon
		{"12am", "00:00", false}, // midnight
		{"12:30am", "00:30", false},
		{"1:01am", "01:01", false},
		{"11:59pm", "23:59", false},

		// Whitespace tolerance
		{"  17:30  ", "17:30", false},

		// Empty input — not an error, caller decides
		{"", "", false},
		{"   ", "", false},

		// Invalid forms
		{"abc", "", true},
		{"25:00", "", true},   // hour out of 24-hour range
		{"17:60", "", true},   // minute out of range
		{"13pm", "", true},    // PM hours are 1-12
		{"0pm", "", true},     // 0 PM invalid
		{"17h30pm", "", true}, // h-separator + AM/PM not supported
		{"5:55 am pm", "", true},
		{"17:30:45", "", true}, // seconds not supported
	}

	for _, c := range cases {
		got, err := NormalizeTimeInput(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeTimeInput(%q) = %q, nil — wanted error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeTimeInput(%q) returned unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeTimeInput(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
