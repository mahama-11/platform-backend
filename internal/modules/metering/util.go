package metering

import "time"

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func ternaryInt64(cond bool, t, f int64) int64 {
	if cond {
		return t
	}
	return f
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
