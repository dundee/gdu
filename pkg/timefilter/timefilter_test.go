package timefilter

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	// Use America/Vancouver timezone for testing (UTC-7 or UTC-8 depending on DST)
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		expectError bool
		expectType  string // "instant", "dateOnly", or "empty"
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: false,
			expectType:  "empty",
		},
		{
			name:        "RFC3339 with timezone",
			input:       "2025-08-11T01:00:00-07:00",
			expectError: false,
			expectType:  "instant",
		},
		{
			name:        "RFC3339 UTC",
			input:       "2025-08-11T08:00:00Z",
			expectError: false,
			expectType:  "instant",
		},
		{
			name:        "RFC3339 with nanoseconds",
			input:       "2025-08-11T01:00:00.123456789-07:00",
			expectError: false,
			expectType:  "instant",
		},
		{
			name:        "date only YYYY-MM-DD",
			input:       "2025-08-11",
			expectError: false,
			expectType:  "dateOnly",
		},
		{
			name:        "invalid format",
			input:       "2025/08/11",
			expectError: true,
			expectType:  "",
		},
		{
			name:        "invalid date",
			input:       "2025-13-01",
			expectError: true,
			expectType:  "",
		},
		{
			name:        "too short date",
			input:       "2025-8-1",
			expectError: true,
			expectType:  "",
		},
		{
			name:        "too long date",
			input:       "2025-08-011",
			expectError: true,
			expectType:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimeValue(tt.input, loc)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			switch tt.expectType {
			case "empty":
				if !result.IsEmpty() {
					t.Errorf("Expected empty result")
				}
			case "instant":
				if result.instant == nil {
					t.Errorf("Expected instant to be set")
				}
				if result.dateOnly != nil {
					t.Errorf("Expected dateOnly to be nil")
				}
			case "dateOnly":
				if result.dateOnly == nil {
					t.Errorf("Expected dateOnly to be set")
				}
				if result.instant != nil {
					t.Errorf("Expected instant to be nil")
				}
			}
		})
	}
}

func TestIncludeBySince(t *testing.T) {
	// Use America/Vancouver timezone for testing (UTC-7 or UTC-8 depending on DST)
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	// Test cases from the MVP document
	tests := []struct {
		name          string
		fileMtime     string // local time
		sinceArg      string
		expectInclude bool
	}{
		{
			name:          "file before date boundary",
			fileMtime:     "2025-08-10T23:59:00-07:00",
			sinceArg:      "2025-08-11",
			expectInclude: false,
		},
		{
			name:          "file at start of date",
			fileMtime:     "2025-08-11T00:00:00-07:00",
			sinceArg:      "2025-08-11",
			expectInclude: true,
		},
		{
			name:          "file during date",
			fileMtime:     "2025-08-11T01:00:00-07:00",
			sinceArg:      "2025-08-11",
			expectInclude: true,
		},
		{
			name:          "file at end of date",
			fileMtime:     "2025-08-11T23:59:00-07:00",
			sinceArg:      "2025-08-11",
			expectInclude: true,
		},
		{
			name:          "file after date",
			fileMtime:     "2025-08-12T00:00:00-07:00",
			sinceArg:      "2025-08-11",
			expectInclude: true,
		},
		{
			name:          "instant mode - file before",
			fileMtime:     "2025-08-11T01:00:00-07:00",
			sinceArg:      "2025-08-11T02:00:00-07:00",
			expectInclude: false,
		},
		{
			name:          "instant mode - file after",
			fileMtime:     "2025-08-11T03:00:00-07:00",
			sinceArg:      "2025-08-11T02:00:00-07:00",
			expectInclude: true,
		},
		{
			name:          "instant mode - file exactly at boundary",
			fileMtime:     "2025-08-11T02:00:00-07:00",
			sinceArg:      "2025-08-11T02:00:00-07:00",
			expectInclude: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse file mtime
			fileMtime, err := time.Parse(time.RFC3339, tt.fileMtime)
			if err != nil {
				t.Fatalf("Failed to parse file mtime: %v", err)
			}

			// Parse since bound
			sinceBound, err := parseTimeValue(tt.sinceArg, loc)
			if err != nil {
				t.Fatalf("Failed to parse since arg: %v", err)
			}

			// Test inclusion
			result := includeByTimeBound(fileMtime, sinceBound, loc, false)
			if result != tt.expectInclude {
				t.Errorf("Expected include=%v, got include=%v", tt.expectInclude, result)
			}
		})
	}
}

func TestIncludeBySinceEmpty(t *testing.T) {
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	// Test with empty since bound (no filter)
	emptySince := TimeBound{}
	testTime := time.Now()

	result := includeByTimeBound(testTime, emptySince, loc, false)
	if !result {
		t.Errorf("Expected true for empty since bound, got false")
	}
}

func TestTimeBoundIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		bound    TimeBound
		expected bool
	}{
		{
			name:     "empty bound",
			bound:    TimeBound{},
			expected: true,
		},
		{
			name:     "instant bound",
			bound:    TimeBound{instant: &time.Time{}},
			expected: false,
		},
		{
			name:     "dateOnly bound",
			bound:    TimeBound{dateOnly: &time.Time{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bound.IsEmpty()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:     "seconds",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "minutes",
			input:    "45m",
			expected: 45 * time.Minute,
		},
		{
			name:     "hours",
			input:    "2h",
			expected: 2 * time.Hour,
		},
		{
			name:     "days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
		},
		{
			name:     "weeks",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
		},
		{
			name:     "months",
			input:    "3mo",
			expected: 3 * 30 * 24 * time.Hour,
		},
		{
			name:     "years",
			input:    "1y",
			expected: 365 * 24 * time.Hour,
		},
		{
			name:     "combined hours and minutes",
			input:    "2h30m",
			expected: 2*time.Hour + 30*time.Minute,
		},
		{
			name:     "combined with spaces",
			input:    "2 h 30 m",
			expected: 2*time.Hour + 30*time.Minute,
		},
		{
			name:     "complex combination",
			input:    "1y2mo3w4d5h6m7s",
			expected: 365*24*time.Hour + 2*30*24*time.Hour + 3*7*24*time.Hour + 4*24*time.Hour + 5*time.Hour + 6*time.Minute + 7*time.Second,
		},
		{
			name:     "uppercase",
			input:    "2H30M",
			expected: 2*time.Hour + 30*time.Minute,
		},
		{
			name:        "invalid format",
			input:       "2x",
			expectError: true,
		},
		{
			name:        "no number",
			input:       "h",
			expectError: true,
		},
		{
			name:        "partial match",
			input:       "2h30",
			expectError: true,
		},
		{
			name:        "invalid number",
			input:       "abch",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNewTimeFilter(t *testing.T) {
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	now := time.Date(2025, 8, 11, 12, 0, 0, 0, loc)

	tests := []struct {
		name        string
		since       string
		until       string
		maxAge      string
		minAge      string
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "empty filter",
			expectEmpty: true,
		},
		{
			name:  "since only",
			since: "2025-08-10",
		},
		{
			name:  "until only",
			until: "2025-08-12",
		},
		{
			name:   "max-age only",
			maxAge: "7d",
		},
		{
			name:   "min-age only",
			minAge: "30d",
		},
		{
			name:  "since and until",
			since: "2025-08-01",
			until: "2025-08-15",
		},
		{
			name:   "max-age and min-age",
			maxAge: "7d",
			minAge: "1d",
		},
		{
			name:   "all filters",
			since:  "2025-08-01",
			until:  "2025-08-15",
			maxAge: "30d",
			minAge: "1d",
		},
		{
			name:        "invalid since",
			since:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid until",
			until:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid max-age",
			maxAge:      "invalid",
			expectError: true,
		},
		{
			name:        "invalid min-age",
			minAge:      "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewTimeFilter(tt.since, tt.until, tt.maxAge, tt.minAge, now, loc)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectEmpty {
				if !filter.IsEmpty() {
					t.Errorf("Expected empty filter")
				}
			} else {
				if filter.IsEmpty() {
					t.Errorf("Expected non-empty filter")
				}
			}
		})
	}
}

func TestTimeFilterIncludeByTimeFilter(t *testing.T) {
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	now := time.Date(2025, 8, 11, 12, 0, 0, 0, loc)

	tests := []struct {
		name          string
		since         string
		until         string
		maxAge        string
		minAge        string
		fileMtime     string
		expectInclude bool
	}{
		{
			name:          "since filter - file after",
			since:         "2025-08-10",
			fileMtime:     "2025-08-11T10:00:00-07:00",
			expectInclude: true,
		},
		{
			name:          "since filter - file before",
			since:         "2025-08-10",
			fileMtime:     "2025-08-09T10:00:00-07:00",
			expectInclude: false,
		},
		{
			name:          "until filter - file before",
			until:         "2025-08-12",
			fileMtime:     "2025-08-11T10:00:00-07:00",
			expectInclude: true,
		},
		{
			name:          "until filter - file after",
			until:         "2025-08-12",
			fileMtime:     "2025-08-13T10:00:00-07:00",
			expectInclude: false,
		},
		{
			name:          "max-age filter - file recent",
			maxAge:        "7d",
			fileMtime:     "2025-08-10T12:00:00-07:00", // 1 day ago
			expectInclude: true,
		},
		{
			name:          "max-age filter - file old",
			maxAge:        "7d",
			fileMtime:     "2025-08-01T12:00:00-07:00", // 10 days ago
			expectInclude: false,
		},
		{
			name:          "min-age filter - file old",
			minAge:        "7d",
			fileMtime:     "2025-08-01T12:00:00-07:00", // 10 days ago
			expectInclude: true,
		},
		{
			name:          "min-age filter - file recent",
			minAge:        "7d",
			fileMtime:     "2025-08-10T12:00:00-07:00", // 1 day ago
			expectInclude: false,
		},
		{
			name:          "combined filters - all pass",
			since:         "2025-08-01",
			until:         "2025-08-15",
			maxAge:        "30d",
			minAge:        "1d",
			fileMtime:     "2025-08-05T12:00:00-07:00", // 6 days ago
			expectInclude: true,
		},
		{
			name:          "combined filters - since fails",
			since:         "2025-08-10",
			until:         "2025-08-15",
			maxAge:        "30d",
			minAge:        "1d",
			fileMtime:     "2025-08-05T12:00:00-07:00", // 6 days ago
			expectInclude: false,
		},
		{
			name:          "combined filters - until fails",
			since:         "2025-08-01",
			until:         "2025-08-10",
			maxAge:        "30d",
			minAge:        "1d",
			fileMtime:     "2025-08-12T12:00:00-07:00", // future
			expectInclude: false,
		},
		{
			name:          "combined filters - max-age fails",
			since:         "2025-08-01",
			until:         "2025-08-15",
			maxAge:        "5d",
			minAge:        "1d",
			fileMtime:     "2025-08-01T12:00:00-07:00", // 10 days ago
			expectInclude: false,
		},
		{
			name:          "combined filters - min-age fails",
			since:         "2025-08-01",
			until:         "2025-08-15",
			maxAge:        "30d",
			minAge:        "5d",
			fileMtime:     "2025-08-10T12:00:00-07:00", // 1 day ago
			expectInclude: false,
		},
		{
			name:          "date-only since and max-age - fail",
			since:         "2025-08-10",
			maxAge:        "3d",
			fileMtime:     "2025-08-09T12:00:00-07:00", // 2 days old, but before since date
			expectInclude: false,
		},
		{
			name:          "date-only since and max-age - pass",
			since:         "2025-08-10",
			maxAge:        "3d",
			fileMtime:     "2025-08-10T12:00:00-07:00", // 1 day old, and on since date
			expectInclude: true,
		},
		{
			name:          "date-only until and min-age - fail",
			until:         "2025-08-10",
			minAge:        "1d",
			fileMtime:     "2025-08-10T12:00:00-07:00", // 1 day old, but not old enough to be excluded by until
			expectInclude: true,
		},
		{
			name:          "date-only until and min-age - pass",
			until:         "2025-08-10",
			minAge:        "2d",
			fileMtime:     "2025-08-08T12:00:00-07:00", // 3 days old, and before until date
			expectInclude: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse file mtime
			fileMtime, err := time.Parse(time.RFC3339, tt.fileMtime)
			if err != nil {
				t.Fatalf("Failed to parse file mtime: %v", err)
			}

			// Create time filter
			filter, err := NewTimeFilter(tt.since, tt.until, tt.maxAge, tt.minAge, now, loc)
			if err != nil {
				t.Fatalf("Failed to create time filter: %v", err)
			}

			// Test inclusion
			result := filter.IncludeByTimeFilter(fileMtime, loc)
			if result != tt.expectInclude {
				t.Errorf("Expected include=%v, got include=%v", tt.expectInclude, result)
			}
		})
	}
}

func TestIncludeByTimeBound(t *testing.T) {
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	tests := []struct {
		name          string
		boundArg      string
		fileMtime     string
		isUntil       bool
		expectInclude bool
	}{
		{
			name:          "since instant - file after",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T11:00:00-07:00",
			isUntil:       false,
			expectInclude: true,
		},
		{
			name:          "since instant - file before",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T09:00:00-07:00",
			isUntil:       false,
			expectInclude: false,
		},
		{
			name:          "since instant - file exactly at boundary",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T10:00:00-07:00",
			isUntil:       false,
			expectInclude: true,
		},
		{
			name:          "until instant - file before",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T09:00:00-07:00",
			isUntil:       true,
			expectInclude: true,
		},
		{
			name:          "until instant - file after",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T11:00:00-07:00",
			isUntil:       true,
			expectInclude: false,
		},
		{
			name:          "until instant - file exactly at boundary",
			boundArg:      "2025-08-11T10:00:00-07:00",
			fileMtime:     "2025-08-11T10:00:00-07:00",
			isUntil:       true,
			expectInclude: true,
		},
		{
			name:          "since date - file just before day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-10T23:59:59-07:00",
			isUntil:       false,
			expectInclude: false,
		},
		{
			name:          "since date - file at start of day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-11T00:00:00-07:00",
			isUntil:       false,
			expectInclude: true,
		},
		{
			name:          "since date - file at end of day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-11T23:59:59-07:00",
			isUntil:       false,
			expectInclude: true,
		},
		{
			name:          "since date - file on next day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-12T00:00:00-07:00",
			isUntil:       false,
			expectInclude: true,
		},
		{
			name:          "until date - file on previous day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-10T23:59:59-07:00",
			isUntil:       true,
			expectInclude: true,
		},
		{
			name:          "until date - file at start of day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-11T00:00:00-07:00",
			isUntil:       true,
			expectInclude: true,
		},
		{
			name:          "until date - file at end of day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-11T23:59:59-07:00",
			isUntil:       true,
			expectInclude: true,
		},
		{
			name:          "until date - file just after day",
			boundArg:      "2025-08-11",
			fileMtime:     "2025-08-12T00:00:00-07:00",
			isUntil:       true,
			expectInclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse time bound
			bound, err := parseTimeValue(tt.boundArg, loc)
			if err != nil {
				t.Fatalf("Failed to parse time bound: %v", err)
			}

			// Parse file mtime
			fileMtime, err := time.Parse(time.RFC3339, tt.fileMtime)
			if err != nil {
				t.Fatalf("Failed to parse file mtime: %v", err)
			}

			// Test inclusion
			result := includeByTimeBound(fileMtime, bound, loc, tt.isUntil)
			if result != tt.expectInclude {
				t.Errorf("Expected include=%v, got include=%v", tt.expectInclude, result)
			}
		})
	}
}
