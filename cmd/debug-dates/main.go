package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
)

func main() {
	// Load configuration
	config := loadConfig()
	
	fmt.Printf("=== Debug Run Dates ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("After Date: %s\n", config.AfterDate.Format("2006-01-02 15:04:05"))
	
	// Create API client
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)
	
	// Fetch first few runs to see their dates
	fmt.Printf("\n--- Fetching First 10 Runs ---\n")
	runs, err := qase.GetRuns(srcClient, config.SourceProject, time.Time{}) // Get all runs
	if err != nil {
		log.Fatalf("Failed to fetch runs: %v", err)
	}
	
	fmt.Printf("Total runs found: %d\n", len(runs))
	
	// Show first 10 runs with their dates
	for i, run := range runs {
		if i >= 10 {
			break
		}
		fmt.Printf("Run %d: ID=%d, Title='%s', CreatedAt=%s\n", 
			i+1, run.ID, run.Title, run.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	
	// Check how many runs are after the specified date
	afterCount := 0
	for _, run := range runs {
		if run.CreatedAt.After(config.AfterDate) {
			afterCount++
		}
	}
	
	fmt.Printf("\n--- Date Analysis ---\n")
	fmt.Printf("Runs after %s: %d\n", config.AfterDate.Format("2006-01-02"), afterCount)
	fmt.Printf("Runs before %s: %d\n", config.AfterDate.Format("2006-01-02"), len(runs)-afterCount)
	
	// Show some recent runs
	fmt.Printf("\n--- Recent Runs (last 5) ---\n")
	recentCount := 0
	for i := len(runs) - 1; i >= 0 && recentCount < 5; i-- {
		run := runs[i]
		fmt.Printf("Run %d: ID=%d, Title='%s', CreatedAt=%s\n", 
			len(runs)-i, run.ID, run.Title, run.CreatedAt.Format("2006-01-02 15:04:05"))
		recentCount++
	}
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
	
	// Parse after date
	afterDateStr := getEnv("QASE_AFTER_DATE", "2025-08-18T00:00:00Z")
	afterDate, err := time.Parse(time.RFC3339, afterDateStr)
	if err != nil {
		log.Fatalf("Invalid QASE_AFTER_DATE format: %v", err)
	}
	config.AfterDate = afterDate
	
	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
