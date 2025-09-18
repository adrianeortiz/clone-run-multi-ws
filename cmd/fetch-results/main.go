package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
	"github.com/adrianeortiz/clone-run-multi-ws/utils"
)

type ResultsData struct {
	SourceProject string        `json:"source_project"`
	AfterDate     time.Time     `json:"after_date"`
	FetchTime     time.Time     `json:"fetch_time"`
	TotalResults  int           `json:"total_results"`
	Results       []qase.Result `json:"results"`

	// Grouped by run for easier processing
	ResultsByRun map[int][]qase.Result `json:"results_by_run"`
}

func main() {
	// Load configuration
	config := loadConfig()

	fmt.Printf("=== Fetch Test Results ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("After Date: %s\n", config.AfterDate.Format("2006-01-02"))

	// Create API client
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)

	// Fetch results after the specified date
	fmt.Printf("\nFetching results after %s...\n", config.AfterDate.Format("2006-01-02"))
	startTime := time.Now()

	results, err := qase.GetResultsAfterDate(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch results: %v", err)
	}

	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d results in %v\n", len(results), fetchDuration)

	// Group results by run ID
	resultsByRun := make(map[int][]qase.Result)
	for _, result := range results {
		resultsByRun[result.RunID] = append(resultsByRun[result.RunID], result)
	}

	fmt.Printf("Grouped into %d runs\n", len(resultsByRun))

	// Create results data structure
	resultsData := ResultsData{
		SourceProject: config.SourceProject,
		AfterDate:     config.AfterDate,
		FetchTime:     time.Now(),
		TotalResults:  len(results),
		Results:       results,
		ResultsByRun:  resultsByRun,
	}

	// Save results data
	resultsDataJSON, err := json.MarshalIndent(resultsData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal results data: %v", err)
	}

	if err := os.WriteFile("results-data.json", resultsDataJSON, 0644); err != nil {
		log.Fatalf("Failed to write results data: %v", err)
	}

	fmt.Printf("\n=== Fetch Complete ===\n")
	fmt.Printf("Results data saved to: results-data.json\n")

	// Print summary
	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("Total results found: %d\n", len(results))
	fmt.Printf("Runs with results: %d\n", len(resultsByRun))
	fmt.Printf("Fetch time: %v\n", fetchDuration)

	// Show results distribution by run
	if len(resultsByRun) > 0 {
		fmt.Printf("\n--- Results by Run ---\n")
		count := 0
		for runID, runResults := range resultsByRun {
			if count >= 10 { // Show first 10 runs
				fmt.Printf("... and %d more runs\n", len(resultsByRun)-10)
				break
			}
			fmt.Printf("Run %d: %d results\n", runID, len(runResults))
			count++
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
	afterDateStr := getEnv("QASE_AFTER_DATE", "1755500400")
	afterDate, err := utils.ParseUnixTimestamp(afterDateStr)
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
