package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
)

type RunsData struct {
	SourceProject string      `json:"source_project"`
	AfterDate     time.Time   `json:"after_date"`
	FetchTime     time.Time   `json:"fetch_time"`
	TotalRuns     int         `json:"total_runs"`
	Runs          []qase.Run  `json:"runs"`
}

func main() {
	// Load configuration
	config := loadConfig()
	
	fmt.Printf("=== Fetch Test Runs ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("After Date: %s\n", config.AfterDate.Format("2006-01-02"))
	
	// Create API client
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)
	
	// Fetch runs after the specified date
	fmt.Printf("\nFetching runs after %s...\n", config.AfterDate.Format("2006-01-02"))
	startTime := time.Now()
	
	runs, err := qase.GetRuns(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch runs: %v", err)
	}
	
	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d runs in %v\n", len(runs), fetchDuration)
	
	// Create runs data structure
	runsData := RunsData{
		SourceProject: config.SourceProject,
		AfterDate:     config.AfterDate,
		FetchTime:     time.Now(),
		TotalRuns:     len(runs),
		Runs:          runs,
	}
	
	// Save runs data
	runsDataJSON, err := json.MarshalIndent(runsData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal runs data: %v", err)
	}
	
	if err := os.WriteFile("runs-data.json", runsDataJSON, 0644); err != nil {
		log.Fatalf("Failed to write runs data: %v", err)
	}
	
	fmt.Printf("\n=== Fetch Complete ===\n")
	fmt.Printf("Runs data saved to: runs-data.json\n")
	
	// Print summary
	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("Total runs found: %d\n", len(runs))
	fmt.Printf("Fetch time: %v\n", fetchDuration)
	
	if len(runs) > 0 {
		fmt.Printf("\n--- Sample Runs ---\n")
		for i, run := range runs {
			if i >= 5 { // Show first 5 runs
				fmt.Printf("... and %d more runs\n", len(runs)-5)
				break
			}
			fmt.Printf("Run %d: %s (ID: %d, Created: %s)\n", 
				i+1, run.Title, run.ID, run.CreatedAt.Format("2006-01-02 15:04:05"))
		}
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
