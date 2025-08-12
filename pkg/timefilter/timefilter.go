package timefilter

import (
	"fmt"
	"time"
)

// SinceBound represents a parsed --since value that can be either an instant or a date-only value
type SinceBound struct {
	instant  *time.Time // absolute instant (UTC)
	dateOnly *time.Time // at local midnight; only YYYY-MM-DD will set this
}

// ParseSince parses --since into either a timestamp Instant or a DateOnly value.
func ParseSince(arg string, loc *time.Location) (SinceBound, error) {
	if arg == "" {
		return SinceBound{}, nil
	}

	// 1) Try RFC3339 instant
	if t, err := time.Parse(time.RFC3339Nano, arg); err == nil {
		u := t.UTC()
		return SinceBound{instant: &u}, nil
	}

	// 2) Try strict YYYY-MM-DD
	if len(arg) == 10 {
		if d, err := time.ParseInLocation("2006-01-02", arg, loc); err == nil {
			// dateOnly uses local date; we will compare date parts only
			return SinceBound{dateOnly: &d}, nil
		}
	}

	return SinceBound{}, fmt.Errorf("invalid --since %q. Use RFC3339 timestamp or YYYY-MM-DD", arg)
}

// IncludeBySince determines if a file should be included based on its mtime and the since bound
func IncludeBySince(mtime time.Time, sb SinceBound, loc *time.Location) bool {
	if sb.instant == nil && sb.dateOnly == nil {
		return true // no filter applied
	}

	if sb.instant != nil {
		return !mtime.Before(*sb.instant) // inclusive
	}

	if sb.dateOnly != nil {
		// Compare local date parts only
		y1, m1, d1 := mtime.In(loc).Date()
		y2, m2, d2 := sb.dateOnly.In(loc).Date()
		if y1 != y2 {
			return y1 > y2
		}
		if m1 != m2 {
			return m1 > m2
		}
		return d1 >= d2
	}

	return true
}

// IsEmpty returns true if the SinceBound has no filter criteria
func (sb SinceBound) IsEmpty() bool {
	return sb.instant == nil && sb.dateOnly == nil
}