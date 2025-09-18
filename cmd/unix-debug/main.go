package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
	"github.com/adrianeortiz/clone-run-multi-ws/utils"
)

func main() {
	// Load configuration
	config := loadConfig()

	fmt.Printf("=== Unix Timestamp Debug ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("After Date: %s (Unix: %s)\n", config.AfterDate.Format("2006-01-02 15:04:05"), utils.ToUnixTimestamp(config.AfterDate))

	// Create API client
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)

	// Get just a few runs to see their actual dates
	fmt.Printf("\n--- Fetching First 5 Runs ---\n")
	runs, err := qase.GetRuns(srcClient, config.SourceProject, time.Time{}) // Get all runs
	if err != nil {
		log.Fatalf("Failed to fetch runs: %v", err)
	}

	fmt.Printf("Total runs in project: %d\n", len(runs))

	// Show first 5 runs with their dates and Unix timestamps
	for i, run := range runs {
		if i >= 5 {
			break
		}
		unixTime := run.CreatedAt.Unix()
		fmt.Printf("Run %d: ID=%d, Title='%s', CreatedAt=%s (Unix: %d)\n",
			i+1, run.ID, run.Title, run.CreatedAt.Format("2006-01-02 15:04:05"), unixTime)
	}

	// Show what Unix timestamp we should use for August 18, 2025
	aug18_2025 := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC)
	fmt.Printf("\n--- Recommended Unix Timestamp ---\n")
	fmt.Printf("For August 18, 2025: %d\n", aug18_2025.Unix())
	fmt.Printf("For August 1, 2025: %d\n", time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC).Unix())
	fmt.Printf("For July 1, 2025: %d\n", time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC).Unix())
	fmt.Printf("For January 1, 2025: %d\n", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
}

type Config struct {
	SourceToken   string
	SourceBaseURL string
	SourceProject string
	AfterDate     time.Time
}

func loadConfig() Config {
	config := Config{
		SourceToken:   getEnv("QASE_SOURCE_API_TOKEN", ""),
		SourceBaseURL: getEnv("QASE_SOURCE_API_BASE", "https://api.qase.io"),
		SourceProject: getEnv("QASE_SOURCE_PROJECT", ""),
	}

	if config.SourceToken == "" {
		log.Fatal("QASE_SOURCE_API_TOKEN is required")
	}
	if config.SourceProject == "" {
		log.Fatal("QASE_SOURCE_PROJECT is required")
	}

	// Parse after date - Unix timestamp only
	afterDateStr := getEnv("QASE_AFTER_DATE", "1755500400") // Default to Aug 18, 2025 Unix timestamp

	// Parse Unix timestamp only
	if t, err := utils.ParseUnixTimestamp(afterDateStr); err == nil {
		config.AfterDate = t
	} else {
		log.Fatalf("Invalid QASE_AFTER_DATE format '%s' (must be Unix timestamp): %v", afterDateStr, err)
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
