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
			result, err := ParseSince(tt.input, loc)

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
		name         string
		fileMtime    string // local time
		sinceArg     string
		expectInclude bool
	}{
		{
			name:         "file before date boundary",
			fileMtime:    "2025-08-10T23:59:00-07:00",
			sinceArg:     "2025-08-11",
			expectInclude: false,
		},
		{
			name:         "file at start of date",
			fileMtime:    "2025-08-11T00:00:00-07:00",
			sinceArg:     "2025-08-11",
			expectInclude: true,
		},
		{
			name:         "file during date",
			fileMtime:    "2025-08-11T01:00:00-07:00",
			sinceArg:     "2025-08-11",
			expectInclude: true,
		},
		{
			name:         "file at end of date",
			fileMtime:    "2025-08-11T23:59:00-07:00",
			sinceArg:     "2025-08-11",
			expectInclude: true,
		},
		{
			name:         "file after date",
			fileMtime:    "2025-08-12T00:00:00-07:00",
			sinceArg:     "2025-08-11",
			expectInclude: true,
		},
		{
			name:         "instant mode - file before",
			fileMtime:    "2025-08-11T01:00:00-07:00",
			sinceArg:     "2025-08-11T02:00:00-07:00",
			expectInclude: false,
		},
		{
			name:         "instant mode - file after",
			fileMtime:    "2025-08-11T03:00:00-07:00",
			sinceArg:     "2025-08-11T02:00:00-07:00",
			expectInclude: true,
		},
		{
			name:         "instant mode - file exactly at boundary",
			fileMtime:    "2025-08-11T02:00:00-07:00",
			sinceArg:     "2025-08-11T02:00:00-07:00",
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
			sinceBound, err := ParseSince(tt.sinceArg, loc)
			if err != nil {
				t.Fatalf("Failed to parse since arg: %v", err)
			}

			// Test inclusion
			result := IncludeBySince(fileMtime, sinceBound, loc)
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
	emptySince := SinceBound{}
	testTime := time.Now()

	result := IncludeBySince(testTime, emptySince, loc)
	if !result {
		t.Errorf("Expected true for empty since bound, got false")
	}
}

func TestSinceBoundIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		bound    SinceBound
		expected bool
	}{
		{
			name:     "empty bound",
			bound:    SinceBound{},
			expected: true,
		},
		{
			name:     "instant bound",
			bound:    SinceBound{instant: &time.Time{}},
			expected: false,
		},
		{
			name:     "dateOnly bound",
			bound:    SinceBound{dateOnly: &time.Time{}},
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