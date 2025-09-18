package utils

import (
	"fmt"
	"strconv"
	"time"
)

// ParseDateFlexible tries multiple date formats to parse a date string
func ParseDateFlexible(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	// Try different date formats in order of preference
	formats := []string{
		time.RFC3339,           // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,       // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05Z", // "2006-01-02T15:04:05Z"
		"2006-01-02T15:04:05",  // "2006-01-02T15:04:05"
		"2006-01-02 15:04:05",  // "2006-01-02 15:04:05"
		"2006-01-02",           // "2006-01-02"
		"2006/01/02",           // "2006/01/02"
		"01/02/2006",           // "01/02/2006"
		"02-01-2006",           // "02-01-2006"
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date '%s' with any known format", dateStr)
}

// ParseDateWithFallback parses a date string with a fallback to Unix timestamp
func ParseDateWithFallback(dateStr string) (time.Time, error) {
	// First try flexible parsing
	if t, err := ParseDateFlexible(dateStr); err == nil {
		return t, nil
	}

	// If that fails, try parsing as Unix timestamp
	if t, err := time.Parse("1136239445", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date '%s' as date string or Unix timestamp", dateStr)
}

// ParseUnixTimestamp parses a Unix timestamp string (seconds since epoch)
func ParseUnixTimestamp(timestampStr string) (time.Time, error) {
	// Try parsing as Unix timestamp (seconds)
	if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		return time.Unix(timestamp, 0), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse '%s' as Unix timestamp", timestampStr)
}

// ToUnixTimestamp converts a time to Unix timestamp string
func ToUnixTimestamp(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}
