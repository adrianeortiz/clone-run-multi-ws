package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/mapping"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
	"github.com/adrianeortiz/clone-run-multi-ws/utils"
)

func main() {
	// Debug: Print environment variables (without secrets)
	fmt.Println("=== Environment Debug ===")
	fmt.Printf("QASE_SOURCE_PROJECT: %s\n", os.Getenv("QASE_SOURCE_PROJECT"))
	fmt.Printf("QASE_TARGET_PROJECT: %s\n", os.Getenv("QASE_TARGET_PROJECT"))
	fmt.Printf("QASE_AFTER_DATE: %s\n", os.Getenv("QASE_AFTER_DATE"))
	fmt.Printf("QASE_MATCH_MODE: %s\n", os.Getenv("QASE_MATCH_MODE"))
	fmt.Printf("QASE_CF_ID: %s\n", os.Getenv("QASE_CF_ID"))
	fmt.Printf("QASE_DRY_RUN: %s\n", os.Getenv("QASE_DRY_RUN"))
	fmt.Printf("QASE_SOURCE_API_TOKEN: %s\n", maskToken(os.Getenv("QASE_SOURCE_API_TOKEN")))
	fmt.Printf("QASE_TARGET_API_TOKEN: %s\n", maskToken(os.Getenv("QASE_TARGET_API_TOKEN")))
	fmt.Println("========================")

	// Load environment variables
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create API clients
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)
	tgtClient := api.NewClient(config.TargetBaseURL, config.TargetToken)

	fmt.Printf("Starting cross-workspace migration from %s to %s\n", config.SourceProject, config.TargetProject)
	fmt.Printf("Filtering runs after: %s\n", config.AfterDate.Format("2006-01-02 15:04:05"))
	fmt.Printf("Mapping mode: %s\n", config.MatchMode)

	// Fetch cases from both workspaces
	fmt.Println("Fetching source cases...")
	srcCases, err := qase.GetCases(srcClient, config.SourceProject)
	if err != nil {
		log.Fatalf("Failed to fetch source cases: %v", err)
	}

	fmt.Println("Fetching target cases...")
	tgtCases, err := qase.GetCases(tgtClient, config.TargetProject)
	if err != nil {
		log.Fatalf("Failed to fetch target cases: %v", err)
	}

	// Build mapping
	var caseMapping map[int]int

	// Check if source and target projects are the same
	if config.SourceProject == config.TargetProject {
		fmt.Println("Source and target projects are the same - using direct case ID mapping")
		caseMapping = make(map[int]int)
		for caseID := range srcCases {
			caseMapping[caseID] = caseID // Direct mapping: source ID = target ID
		}
		fmt.Printf("Built direct mapping with %d entries\n", len(caseMapping))
	} else {
		fmt.Printf("Building mapping using %s mode...\n", config.MatchMode)
		caseMapping, err = mapping.Build(
			mapping.Mode(config.MatchMode),
			srcCases,
			tgtCases,
			config.CustomFieldID,
			config.MappingCSV,
		)
		if err != nil {
			log.Fatalf("Failed to build mapping: %v", err)
		}
		fmt.Printf("Built mapping with %d entries\n", len(caseMapping))
	}

	// Write mapping artifact
	if err := writeMappingArtifact(caseMapping); err != nil {
		log.Printf("Warning: Failed to write mapping artifact: %v", err)
	}

	// Fetch all results after the specified date using results API
	fmt.Printf("Fetching results from source project after %s...\n", config.AfterDate.Format("2006-01-02"))

	startTime := time.Now()

	// Fetch all results after the date directly - this should be much faster
	allResults, err := qase.GetResultsAfterDate(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch results: %v", err)
	}

	fmt.Printf("Fetched %d total results in %v\n", len(allResults), time.Since(startTime))

	if len(allResults) == 0 {
		fmt.Println("No results found for the specified runs. Nothing to migrate.")
		return
	}

	// Group results by run ID
	resultsByRun := make(map[int][]qase.Result)
	for _, result := range allResults {
		resultsByRun[result.RunID] = append(resultsByRun[result.RunID], result)
	}

	fmt.Printf("Grouped results into %d runs\n", len(resultsByRun))

	// Add timeout protection
	timeout := 30 * time.Minute
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	// Process each run that has results
	totalResults := 0
	totalSkipped := 0
	successfulRuns := 0
	failedRuns := 0

	// Create channels for coordination
	type runResult struct {
		runID       int
		results     int
		skipped     int
		success     bool
		error       error
		runDuration time.Duration
	}

	resultsChan := make(chan runResult, len(resultsByRun))
	semaphore := make(chan struct{}, config.Concurrency)

	fmt.Printf("Processing %d runs with results (concurrency: %d)\n", len(resultsByRun), config.Concurrency)

	// Launch goroutines for each run that has results
	runIndex := 0
	for runID, results := range resultsByRun {
		go func(runID int, results []qase.Result, index int) {
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			runStartTime := time.Now()
			fmt.Printf("\n--- Processing run %d/%d: ID %d with %d results ---\n",
				index+1, len(resultsByRun), runID, len(results))

			// Create run details from results data
			// Use the first result's end time to create a meaningful run title
			var runTitle string
			var runDescription string
			if len(results) > 0 {
				// Parse the end time from the first result
				if endTime, err := time.Parse("2006-01-02T15:04:05-07:00", results[0].EndTime); err == nil {
					runTitle = fmt.Sprintf("Migrated Run %d (%s)", runID, endTime.Format("2006-01-02 15:04"))
				} else {
					runTitle = fmt.Sprintf("Migrated Run %d", runID)
				}
				runDescription = fmt.Sprintf("Migrated run with %d results from source workspace", len(results))
			} else {
				runTitle = fmt.Sprintf("Run %d", runID)
				runDescription = "Migrated run"
			}

			// Transform results to target case IDs
			fmt.Printf("Transforming %d results...\n", len(results))
			bulkItems, skipped := transformResults(results, caseMapping, config.StatusMap)

			fmt.Printf("Prepared %d results for posting, skipped %d unmapped results\n", len(bulkItems), skipped)

			// Handle dry run mode
			if config.DryRun {
				fmt.Printf("DRY RUN MODE - Would create run '%s' with %d results\n", runTitle, len(bulkItems))
				resultsChan <- runResult{
					runID: runID, success: true, results: len(bulkItems), skipped: skipped,
					runDuration: time.Since(runStartTime),
				}
				return
			}

			var tgtRun *qase.Run
			var err error

			if config.Idempotent {
				// Create or get existing target run (idempotent)
				fmt.Printf("Creating or finding target run: %s\n", runTitle)
				tgtRun, err = qase.CreateOrGetRun(tgtClient, config.TargetProject, runTitle, runDescription)
				if err != nil {
					log.Printf("Failed to create/get target run for %s: %v", runTitle, err)
					resultsChan <- runResult{runID: runID, success: false, error: err, runDuration: time.Since(runStartTime)}
					return
				}

				// Check if run already has results (idempotent)
				hasResults, err := qase.CheckRunHasResults(tgtClient, config.TargetProject, tgtRun.ID)
				if err != nil {
					log.Printf("Failed to check existing results for run %d: %v", tgtRun.ID, err)
					resultsChan <- runResult{runID: runID, success: false, error: err, runDuration: time.Since(runStartTime)}
					return
				}

				if hasResults {
					fmt.Printf("Run %d already has results, filtering for new ones only...\n", tgtRun.ID)
					// Filter out results that already exist
					bulkItems, err = qase.FilterNewResults(tgtClient, config.TargetProject, tgtRun.ID, bulkItems)
					if err != nil {
						log.Printf("Failed to filter existing results for run %d: %v", tgtRun.ID, err)
						resultsChan <- runResult{runID: runID, success: false, error: err, runDuration: time.Since(runStartTime)}
						return
					}
				}

				if len(bulkItems) == 0 {
					fmt.Printf("No new results to post for run %d (all already exist)\n", tgtRun.ID)
					resultsChan <- runResult{
						runID: runID, success: true, results: 0, skipped: skipped,
						runDuration: time.Since(runStartTime),
					}
					return
				}

				// Post only new results to target run
				fmt.Printf("Posting %d new results to target run %d...\n", len(bulkItems), tgtRun.ID)
			} else {
				// Non-idempotent mode: always create new runs
				fmt.Printf("Creating target run: %s\n", runTitle)
				tgtRun, err = qase.CreateRun(tgtClient, config.TargetProject, runTitle, runDescription)
				if err != nil {
					log.Printf("Failed to create target run for %s: %v", runTitle, err)
					resultsChan <- runResult{runID: runID, success: false, error: err, runDuration: time.Since(runStartTime)}
					return
				}

				// Post all results to target run
				fmt.Printf("Posting %d results to target run %d...\n", len(bulkItems), tgtRun.ID)
			}
			if err := qase.PostBulkResults(tgtClient, config.TargetProject, tgtRun.ID, bulkItems, config.BulkSize); err != nil {
				log.Printf("Failed to post results to run %d: %v", tgtRun.ID, err)
				resultsChan <- runResult{runID: runID, success: false, error: err, runDuration: time.Since(runStartTime)}
				return
			}

			runDuration := time.Since(runStartTime)
			fmt.Printf("Successfully migrated run %d -> %d (took %v)\n", runID, tgtRun.ID, runDuration)
			resultsChan <- runResult{
				runID: runID, success: true, results: len(bulkItems), skipped: skipped,
				runDuration: runDuration,
			}
		}(runID, results, runIndex)
		runIndex++
	}

	// Collect results with timeout
	completed := 0
	for completed < len(resultsByRun) {
		select {
		case result := <-resultsChan:
			completed++
			if result.success {
				successfulRuns++
				totalResults += result.results
				totalSkipped += result.skipped
			} else {
				failedRuns++
			}
			fmt.Printf("Completed %d/%d runs\n", completed, len(resultsByRun))

		case <-timeoutTimer.C:
			fmt.Printf("TIMEOUT: Migration exceeded %v limit. Completed %d/%d runs\n", timeout, completed, len(resultsByRun))
			break
		}
	}

	totalDuration := time.Since(startTime)

	// Print summary
	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Total runs with results: %d\n", len(resultsByRun))
	fmt.Printf("Successful migrations: %d\n", successfulRuns)
	fmt.Printf("Failed migrations: %d\n", failedRuns)
	fmt.Printf("Total results migrated: %d\n", totalResults)
	fmt.Printf("Total results skipped: %d\n", totalSkipped)
	fmt.Printf("Total execution time: %v\n", totalDuration)

	if config.DryRun {
		fmt.Println("\nDRY RUN MODE - No actual changes were made")
	} else {
		fmt.Println("\nMigration completed!")
	}
}

// Config holds all configuration values
type Config struct {
	// Source workspace
	SourceToken   string
	SourceBaseURL string
	SourceProject string

	// Target workspace
	TargetToken   string
	TargetBaseURL string
	TargetProject string

	// Date filtering
	AfterDate time.Time

	// Mapping configuration
	MatchMode     string
	CustomFieldID int
	MappingCSV    string

	// Behavior
	DryRun      bool
	BulkSize    int
	Concurrency int
	StatusMap   map[string]string
	Idempotent  bool
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	config := &Config{
		SourceBaseURL: getEnvDefault("QASE_SOURCE_API_BASE", "https://api.qase.io"),
		TargetBaseURL: getEnvDefault("QASE_TARGET_API_BASE", "https://api.qase.io"),
		MatchMode:     getEnvDefault("QASE_MATCH_MODE", "custom_field"),
		DryRun:        getEnvDefault("QASE_DRY_RUN", "true") == "true",
		BulkSize:      getIntDefault("QASE_BULK_SIZE", 200),
		Concurrency:   getIntDefault("QASE_CONCURRENCY", 2),
		Idempotent:    getEnvDefault("QASE_IDEMPOTENT", "true") == "true",
	}

	// Required environment variables
	config.SourceToken = mustEnv("QASE_SOURCE_API_TOKEN")
	config.SourceProject = mustEnv("QASE_SOURCE_PROJECT")

	config.TargetToken = mustEnv("QASE_TARGET_API_TOKEN")
	config.TargetProject = mustEnv("QASE_TARGET_PROJECT")

	// Date filtering - default to August 18th, 2025
	afterDateStr := getEnvDefault("QASE_AFTER_DATE", "1755500400")
	afterDate, err := utils.ParseUnixTimestamp(afterDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid QASE_AFTER_DATE format (must be Unix timestamp): %w", err)
	}
	config.AfterDate = afterDate

	// Mapping configuration
	if config.MatchMode == "custom_field" {
		config.CustomFieldID = getIntDefault("QASE_CF_ID", 0)
		if config.CustomFieldID == 0 {
			return nil, fmt.Errorf("QASE_CF_ID is required for custom_field mode")
		}
	} else if config.MatchMode == "csv" {
		config.MappingCSV = mustEnv("QASE_MAPPING_CSV")
	} else {
		return nil, fmt.Errorf("unsupported QASE_MATCH_MODE: %s", config.MatchMode)
	}

	// Status mapping
	if statusMapStr := os.Getenv("QASE_STATUS_MAP"); statusMapStr != "" {
		statusMap, err := parseStatusMap(statusMapStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse QASE_STATUS_MAP: %w", err)
		}
		config.StatusMap = statusMap
	}

	return config, nil
}

// transformResults transforms source results to target case IDs
func transformResults(results []qase.Result, caseMapping map[int]int, statusMap map[string]string) ([]qase.BulkItem, int) {
	var bulkItems []qase.BulkItem
	skipped := 0

	// Maximum time allowed by Qase API (1 year in seconds)
	const maxTimeSeconds = 31536000

	for _, result := range results {
		targetCaseID, exists := caseMapping[result.CaseID]
		if !exists {
			skipped++
			continue
		}

		// Apply status mapping if configured
		status := result.Status
		if statusMap != nil {
			if mappedStatus, exists := statusMap[status]; exists {
				status = mappedStatus
			}
		}

		// Validate and cap time value if present
		var timeSeconds *int
		if result.Time != nil && *result.Time > 0 {
			timeInSeconds := *result.Time
			if timeInSeconds > maxTimeSeconds {
				fmt.Printf("Warning: Capping time for case %d from %d seconds to %d seconds (max allowed)\n", 
					result.CaseID, timeInSeconds, maxTimeSeconds)
				timeInSeconds = maxTimeSeconds
			}
			timeSeconds = &timeInSeconds
		}

		bulkItem := qase.BulkItem{
			CaseID:  targetCaseID,
			Status:  status,
			Time:    timeSeconds,
			Comment: result.Comment,
		}

		bulkItems = append(bulkItems, bulkItem)
	}

	return bulkItems, skipped
}

// writeMappingArtifact writes the case mapping to a CSV file
func writeMappingArtifact(caseMapping map[int]int) error {
	file, err := os.Create("case_map.out.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"source_case_id", "target_case_id"}); err != nil {
		return err
	}

	// Write mappings
	for sourceID, targetID := range caseMapping {
		if err := writer.Write([]string{strconv.Itoa(sourceID), strconv.Itoa(targetID)}); err != nil {
			return err
		}
	}

	fmt.Println("Mapping artifact written to case_map.out.csv")
	return nil
}

// parseStatusMap parses status mapping from environment variable
func parseStatusMap(statusMapStr string) (map[string]string, error) {
	statusMap := make(map[string]string)

	pairs := strings.Split(statusMapStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid status mapping pair: %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		statusMap[key] = value
	}

	return statusMap, nil
}

// Helper functions for environment variables
func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}

func mustInt(key string) int {
	value := mustEnv(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Environment variable %s must be an integer, got: %s", key, value)
	}
	return intValue
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// maskToken masks the token for logging (shows first 8 and last 4 characters)
func maskToken(token string) string {
	if token == "" {
		return "<not set>"
	}
	if len(token) <= 12 {
		return "***"
	}
	return token[:8] + "..." + token[len(token)-4:]
}
