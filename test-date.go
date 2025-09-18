package main

import (
	"fmt"
	"time"
)

func main() {
	// Test different date formats
	formats := []string{
		"2025-08-18T00:00:00Z",
		"2025-08-18",
		"2025-08-18T00:00:00",
		"2025-08-18 00:00:00",
	}

	for _, dateStr := range formats {
		fmt.Printf("Testing: %s\n", dateStr)

		// Try RFC3339
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			fmt.Printf("  RFC3339: %v ✓\n", t)
		} else {
			fmt.Printf("  RFC3339: ERROR - %v\n", err)
		}

		// Try custom format
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			fmt.Printf("  Custom: %v ✓\n", t)
		} else {
			fmt.Printf("  Custom: ERROR - %v\n", err)
		}

		fmt.Println()
	}
}
