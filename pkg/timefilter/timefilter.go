package timefilter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeBound represents a parsed time filter value that can be either an instant or a date-only value
type TimeBound struct {
	instant  *time.Time // absolute instant (UTC)
	dateOnly *time.Time // at local midnight; only YYYY-MM-DD will set this
}

// TimeFilter represents multiple time filtering criteria
type TimeFilter struct {
	since   *TimeBound
	until   *TimeBound
	maxAge  *time.Duration
	minAge  *time.Duration
}

// SinceBound represents a parsed --since value that can be either an instant or a date-only value
// Deprecated: Use TimeBound instead
type SinceBound = TimeBound

// ParseDuration parses a duration string with support for extended units
// Supports: s, m, h, d (=24h), w (=7d), mo (=30d), y (=365d)
// Examples: "90m", "2h30m", "7d", "6w", "1y2mo"
func ParseDuration(input string) (time.Duration, error) {
	if input == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Remove whitespace and convert to lowercase
	input = strings.ToLower(strings.ReplaceAll(input, " ", ""))

	// Regex to match number+unit pairs (mo must come before m to avoid greedy matching)
	re := regexp.MustCompile(`(\d+)(mo|s|m|h|d|w|y)`)
	matches := re.FindAllStringSubmatch(input, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format %q. Use combinations like 7d, 2h30m, 1y2mo", input)
	}

	// Check if the entire input was consumed by matches
	consumed := ""
	for _, match := range matches {
		consumed += match[0]
	}
	if consumed != input {
		return 0, fmt.Errorf("invalid duration format %q. Use combinations like 7d, 2h30m, 1y2mo", input)
	}

	var total time.Duration
	for _, match := range matches {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", match[1])
		}

		unit := match[2]
		var duration time.Duration

		switch unit {
		case "s":
			duration = time.Duration(value) * time.Second
		case "m":
			duration = time.Duration(value) * time.Minute
		case "h":
			duration = time.Duration(value) * time.Hour
		case "d":
			duration = time.Duration(value) * 24 * time.Hour
		case "w":
			duration = time.Duration(value) * 7 * 24 * time.Hour
		case "mo":
			duration = time.Duration(value) * 30 * 24 * time.Hour
		case "y":
			duration = time.Duration(value) * 365 * 24 * time.Hour
		default:
			return 0, fmt.Errorf("unsupported duration unit: %s", unit)
		}

		total += duration
	}

	return total, nil
}

// ParseTimeValue parses a time value into either a timestamp instant or a date-only value
func ParseTimeValue(arg string, loc *time.Location) (TimeBound, error) {
	if arg == "" {
		return TimeBound{}, nil
	}

	// 1) Try RFC3339 instant
	if t, err := time.Parse(time.RFC3339Nano, arg); err == nil {
		u := t.UTC()
		return TimeBound{instant: &u}, nil
	}

	// 2) Try strict YYYY-MM-DD
	if len(arg) == 10 {
		if d, err := time.ParseInLocation("2006-01-02", arg, loc); err == nil {
			// dateOnly uses local date; we will compare date parts only
			return TimeBound{dateOnly: &d}, nil
		}
	}

	return TimeBound{}, fmt.Errorf("invalid time value %q. Use RFC3339 timestamp or YYYY-MM-DD", arg)
}

// ParseSince parses --since into either a timestamp Instant or a DateOnly value.
func ParseSince(arg string, loc *time.Location) (SinceBound, error) {
	return ParseTimeValue(arg, loc)
}

// NewTimeFilter creates a new TimeFilter with the given parameters
func NewTimeFilter(since, until string, maxAge, minAge string, now time.Time, loc *time.Location) (*TimeFilter, error) {
	tf := &TimeFilter{}

	// Parse since
	if since != "" {
		sinceBound, err := ParseTimeValue(since, loc)
		if err != nil {
			return nil, fmt.Errorf("invalid --since value: %w", err)
		}
		if !sinceBound.IsEmpty() {
			tf.since = &sinceBound
		}
	}

	// Parse until
	if until != "" {
		untilBound, err := ParseTimeValue(until, loc)
		if err != nil {
			return nil, fmt.Errorf("invalid --until value: %w", err)
		}
		if !untilBound.IsEmpty() {
			tf.until = &untilBound
		}
	}

	// Parse max-age (convert to since)
	if maxAge != "" {
		duration, err := ParseDuration(maxAge)
		if err != nil {
			return nil, fmt.Errorf("invalid --max-age value: %w", err)
		}
		tf.maxAge = &duration
	}

	// Parse min-age (convert to until)
	if minAge != "" {
		duration, err := ParseDuration(minAge)
		if err != nil {
			return nil, fmt.Errorf("invalid --min-age value: %w", err)
		}
		tf.minAge = &duration
	}

	return tf, nil
}

// IncludeByTimeBound determines if a file should be included based on its mtime and the time bound
func IncludeByTimeBound(mtime time.Time, tb TimeBound, loc *time.Location, isUntil bool) bool {
	if tb.instant == nil && tb.dateOnly == nil {
		return true // no filter applied
	}

	if tb.instant != nil {
		if isUntil {
			return !mtime.After(*tb.instant) // inclusive (<=)
		}
		return !mtime.Before(*tb.instant) // inclusive (>=)
	}

	if tb.dateOnly != nil {
		// Compare local date parts only
		y1, m1, d1 := mtime.In(loc).Date()
		y2, m2, d2 := tb.dateOnly.In(loc).Date()
		
		if y1 != y2 {
			if isUntil {
				return y1 < y2 || (y1 == y2) // <=
			}
			return y1 > y2 // >=
		}
		if m1 != m2 {
			if isUntil {
				return m1 < m2 || (m1 == m2) // <=
			}
			return m1 > m2 // >=
		}
		if isUntil {
			return d1 <= d2 // <=
		}
		return d1 >= d2 // >=
	}

	return true
}

// IncludeBySince determines if a file should be included based on its mtime and the since bound
func IncludeBySince(mtime time.Time, sb SinceBound, loc *time.Location) bool {
	return IncludeByTimeBound(mtime, sb, loc, false)
}

// IncludeByTimeFilter determines if a file should be included based on the complete time filter
func (tf *TimeFilter) IncludeByTimeFilter(mtime time.Time, now time.Time, loc *time.Location) bool {
	// Check since bound
	if tf.since != nil {
		if !IncludeByTimeBound(mtime, *tf.since, loc, false) {
			return false
		}
	}

	// Check until bound
	if tf.until != nil {
		if !IncludeByTimeBound(mtime, *tf.until, loc, true) {
			return false
		}
	}

	// Check max-age (convert to since)
	if tf.maxAge != nil {
		sinceTime := now.Add(-*tf.maxAge).UTC()
		sinceBound := TimeBound{instant: &sinceTime}
		if !IncludeByTimeBound(mtime, sinceBound, loc, false) {
			return false
		}
	}

	// Check min-age (convert to until)
	if tf.minAge != nil {
		untilTime := now.Add(-*tf.minAge).UTC()
		untilBound := TimeBound{instant: &untilTime}
		if !IncludeByTimeBound(mtime, untilBound, loc, true) {
			return false
		}
	}

	return true
}

// IsEmpty returns true if the TimeFilter has no filter criteria
func (tf *TimeFilter) IsEmpty() bool {
	return tf.since == nil && tf.until == nil && tf.maxAge == nil && tf.minAge == nil
}

// IsEmpty returns true if the TimeBound has no filter criteria
func (tb TimeBound) IsEmpty() bool {
	return tb.instant == nil && tb.dateOnly == nil
}