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

type ProjectAnalysis struct {
	SourceProject string    `json:"source_project"`
	TargetProject string    `json:"target_project"`
	AfterDate     time.Time `json:"after_date"`
	AnalysisTime  time.Time `json:"analysis_time"`

	// Source project stats
	SourceStats struct {
		TotalCases   int `json:"total_cases"`
		TotalRuns    int `json:"total_runs"`
		TotalResults int `json:"total_results"`
	} `json:"source_stats"`

	// Filtered data counts
	FilteredRuns    int `json:"filtered_runs"`
	FilteredResults int `json:"filtered_results"`

	// Recommendations
	Recommendations []string `json:"recommendations"`
}

func main() {
	// Load configuration
	config := loadConfig()

	fmt.Printf("=== Project Analysis ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("Target Project: %s\n", config.TargetProject)
	fmt.Printf("After Date: %s\n", config.AfterDate.Format("2006-01-02"))

	// Create API clients
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)

	analysis := ProjectAnalysis{
		SourceProject: config.SourceProject,
		TargetProject: config.TargetProject,
		AfterDate:     config.AfterDate,
		AnalysisTime:  time.Now(),
	}

	// Analyze source project
	fmt.Printf("\n--- Analyzing Source Project ---\n")

	// Get project info
	fmt.Printf("Fetching project information...\n")
	// Note: We'll implement project info fetching if needed

	// Get total cases count (with pagination limit)
	fmt.Printf("Counting test cases...\n")
	cases, err := qase.GetCases(srcClient, config.SourceProject)
	if err != nil {
		log.Fatalf("Failed to fetch cases: %v", err)
	}
	analysis.SourceStats.TotalCases = len(cases)
	fmt.Printf("Total cases: %d\n", analysis.SourceStats.TotalCases)

	// Get total runs count
	fmt.Printf("Counting test runs...\n")
	runs, err := qase.GetRuns(srcClient, config.SourceProject, time.Time{}) // Get all runs
	if err != nil {
		log.Fatalf("Failed to fetch runs: %v", err)
	}
	analysis.SourceStats.TotalRuns = len(runs)
	fmt.Printf("Total runs: %d\n", analysis.SourceStats.TotalRuns)

	// Get filtered runs (after date)
	fmt.Printf("Counting runs after %s...\n", config.AfterDate.Format("2006-01-02"))
	filteredRuns, err := qase.GetRuns(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch filtered runs: %v", err)
	}
	analysis.FilteredRuns = len(filteredRuns)
	fmt.Printf("Filtered runs: %d\n", analysis.FilteredRuns)

	// Get filtered results count
	fmt.Printf("Counting results after %s...\n", config.AfterDate.Format("2006-01-02"))
	results, err := qase.GetResultsAfterDate(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch filtered results: %v", err)
	}
	analysis.FilteredResults = len(results)
	fmt.Printf("Filtered results: %d\n", analysis.FilteredResults)

	// Generate recommendations
	analysis.Recommendations = generateRecommendations(analysis)

	// Save analysis results
	analysisData, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal analysis: %v", err)
	}

	if err := os.WriteFile("analysis-results.json", analysisData, 0644); err != nil {
		log.Fatalf("Failed to write analysis results: %v", err)
	}

	fmt.Printf("\n=== Analysis Complete ===\n")
	fmt.Printf("Analysis saved to: analysis-results.json\n")

	// Print summary
	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("Source Project: %s\n", analysis.SourceProject)
	fmt.Printf("Total Cases: %d\n", analysis.SourceStats.TotalCases)
	fmt.Printf("Total Runs: %d\n", analysis.SourceStats.TotalRuns)
	fmt.Printf("Runs after %s: %d\n", config.AfterDate.Format("2006-01-02"), analysis.FilteredRuns)
	fmt.Printf("Results after %s: %d\n", config.AfterDate.Format("2006-01-02"), analysis.FilteredResults)

	fmt.Printf("\n--- Recommendations ---\n")
	for i, rec := range analysis.Recommendations {
		fmt.Printf("%d. %s\n", i+1, rec)
	}
}

func generateRecommendations(analysis ProjectAnalysis) []string {
	var recommendations []string

	if analysis.FilteredResults > 10000 {
		recommendations = append(recommendations, "Large dataset detected - consider running migration in smaller batches")
	}

	if analysis.FilteredRuns > 1000 {
		recommendations = append(recommendations, "Many runs detected - migration may take significant time")
	}

	if analysis.SourceStats.TotalCases > 50000 {
		recommendations = append(recommendations, "Very large case database - case mapping may be slow")
	}

	if analysis.FilteredResults == 0 {
		recommendations = append(recommendations, "No results found for the specified date - check date format and project data")
	}

	if analysis.FilteredRuns == 0 {
		recommendations = append(recommendations, "No runs found for the specified date - check date format and project data")
	}

	recommendations = append(recommendations, "Consider running Step 2 (Fetch Runs) before Step 3 (Fetch Results)")
	recommendations = append(recommendations, "Use dry run mode first to validate the migration approach")

	return recommendations
}

type Config struct {
	SourceToken   string
	SourceBaseURL string
	SourceProject string
	TargetProject string
	AfterDate     time.Time
}

func loadConfig() Config {
	config := Config{
		SourceToken:   getEnv("QASE_SOURCE_API_TOKEN", ""),
		SourceBaseURL: getEnv("QASE_SOURCE_API_BASE", "https://api.qase.io"),
		SourceProject: getEnv("QASE_SOURCE_PROJECT", ""),
		TargetProject: getEnv("QASE_TARGET_PROJECT", ""),
	}

	if config.SourceToken == "" {
		log.Fatal("QASE_SOURCE_API_TOKEN is required")
	}
	if config.SourceProject == "" {
		log.Fatal("QASE_SOURCE_PROJECT is required")
	}
	if config.TargetProject == "" {
		log.Fatal("QASE_TARGET_PROJECT is required")
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
