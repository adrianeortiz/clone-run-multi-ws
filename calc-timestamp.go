package main

import (
	"fmt"
	"time"
)

func main() {
	// Calculate Unix timestamp for August 18, 2025
	aug18_2025 := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC)
	fmt.Printf("August 18, 2025 Unix timestamp: %d\n", aug18_2025.Unix())
	
	// Also show some other useful timestamps
	fmt.Printf("August 1, 2025 Unix timestamp: %d\n", time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC).Unix())
	fmt.Printf("July 1, 2025 Unix timestamp: %d\n", time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC).Unix())
	fmt.Printf("January 1, 2025 Unix timestamp: %d\n", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
	
	// Show what the current wrong timestamp represents
	wrong := time.Unix(1723939200, 0)
	fmt.Printf("Current wrong timestamp 1723939200 = %s\n", wrong.Format("2006-01-02 15:04:05"))
}
