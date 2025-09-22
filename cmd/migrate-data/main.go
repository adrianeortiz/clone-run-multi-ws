package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
	"github.com/adrianeortiz/clone-run-multi-ws/mapping"
	"github.com/adrianeortiz/clone-run-multi-ws/qase"
	"github.com/adrianeortiz/clone-run-multi-ws/utils"
)

type MigrationResults struct {
	SourceProject string    `json:"source_project"`
	TargetProject string    `json:"target_project"`
	AfterDate     time.Time `json:"after_date"`
	MigrationTime time.Time `json:"migration_time"`
	DryRun        bool      `json:"dry_run"`

	// Statistics
	TotalRuns      int `json:"total_runs"`
	SuccessfulRuns int `json:"successful_runs"`
	FailedRuns     int `json:"failed_runs"`
	TotalResults   int `json:"total_results"`
	TotalSkipped   int `json:"total_skipped"`

	// Timing
	TotalDuration     time.Duration `json:"total_duration"`
	RunsDuration      time.Duration `json:"runs_duration"`
	ResultsDuration   time.Duration `json:"results_duration"`
	MigrationDuration time.Duration `json:"migration_duration"`
}

func main() {
	// Load configuration
	config := loadConfig()

	fmt.Printf("=== Migrate Data ===\n")
	fmt.Printf("Source Project: %s\n", config.SourceProject)
	fmt.Printf("Target Project: %s\n", config.TargetProject)
	fmt.Printf("After Date: %s\n", config.AfterDate.Format("2006-01-02"))
	fmt.Printf("Match Mode: %s\n", config.MatchMode)
	fmt.Printf("Dry Run: %t\n", config.DryRun)
	fmt.Printf("Idempotent: %t\n", config.Idempotent)

	// Create API clients
	srcClient := api.NewClient(config.SourceBaseURL, config.SourceToken)
	tgtClient := api.NewClient(config.TargetBaseURL, config.TargetToken)

	startTime := time.Now()

	// Step 1: Fetch results
	fmt.Printf("\n--- Step 1: Fetching Test Results ---\n")
	runsStartTime := time.Now()

	allResults, err := qase.GetResultsAfterDate(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch results: %v", err)
	}

	resultsDuration := time.Since(runsStartTime)
	fmt.Printf("Fetched %d results in %v\n", len(allResults), resultsDuration)

	if len(allResults) == 0 {
		fmt.Println("No results found for the specified date. Nothing to migrate.")
		return
	}

	// Group results by run ID
	resultsByRun := make(map[int][]qase.Result)
	for _, result := range allResults {
		resultsByRun[result.RunID] = append(resultsByRun[result.RunID], result)
	}

	fmt.Printf("Grouped results into %d runs\n", len(resultsByRun))

	// Auto-disable detailed idempotency for large migrations to prevent timeouts
	if config.Idempotent && len(resultsByRun) > 20 {
		fmt.Printf("Large migration detected (%d runs), using fast mode (run deduplication only)\n", len(resultsByRun))
	}

	// Step 2: Build case mapping
	fmt.Printf("\n--- Step 2: Building Case Mapping ---\n")

	var caseMapping map[int]int

	if config.SourceProject == config.TargetProject {
		// Direct mapping for same project
		fmt.Printf("Using direct case ID mapping (same project)\n")
		caseMapping = make(map[int]int)
		for _, result := range allResults {
			caseMapping[result.CaseID] = result.CaseID
		}
	} else {
		// Build mapping based on match mode
		// First, we need to fetch cases from both projects
		fmt.Printf("Fetching source cases...\n")
		srcCases, err := qase.GetCases(srcClient, config.SourceProject)
		if err != nil {
			log.Fatalf("Failed to fetch source cases: %v", err)
		}

		fmt.Printf("Fetching target cases...\n")
		tgtCases, err := qase.GetCases(tgtClient, config.TargetProject)
		if err != nil {
			log.Fatalf("Failed to fetch target cases: %v", err)
		}

		// Build mapping
		switch config.MatchMode {
		case "custom_field":
			fmt.Printf("Building case mapping using custom field %d\n", config.CFID)
			caseMapping, err = mapping.Build(mapping.ModeCF, srcCases, tgtCases, config.CFID, "")
		case "csv":
			fmt.Printf("Building case mapping from CSV file\n")
			caseMapping, err = mapping.Build(mapping.ModeCSV, srcCases, tgtCases, 0, config.CSVFile)
		default:
			log.Fatalf("Unknown match mode: %s", config.MatchMode)
		}

		if err != nil {
			log.Fatalf("Failed to build case mapping: %v", err)
		}
	}

	fmt.Printf("Built mapping for %d cases\n", len(caseMapping))

	// Step 3: Perform migration
	fmt.Printf("\n--- Step 3: Performing Migration ---\n")
	migrationStartTime := time.Now()

	// Process each run that has results
	totalResults := 0
	totalSkipped := 0
	successfulRuns := 0
	failedRuns := 0

	for runID, runResults := range resultsByRun {
		// Create run details from results data
		runTitle := fmt.Sprintf("Migrated Run %d", runID)
		runDescription := fmt.Sprintf("Migrated run with %d results from source workspace", len(runResults))

		// Use the first result's end time to create a meaningful run title
		if len(runResults) > 0 {
			if endTime, err := time.Parse("2006-01-02T15:04:05-07:00", runResults[0].EndTime); err == nil {
				runTitle = fmt.Sprintf("Migrated Run %d (%s)", runID, endTime.Format("2006-01-02 15:04"))
			}
		}

		fmt.Printf("\nProcessing run %d: %s (%d results)\n", runID, runTitle, len(runResults))

		// Transform results to target case IDs
		bulkItems, skipped := transformResults(runResults, caseMapping, config.StatusMap)
		totalSkipped += skipped

		fmt.Printf("Prepared %d results for posting, skipped %d unmapped results\n", len(bulkItems), skipped)

		if len(bulkItems) == 0 {
			fmt.Printf("No results to migrate for run %d\n", runID)
			continue
		}

		// Handle dry run mode
		if config.DryRun {
			fmt.Printf("DRY RUN MODE - Would create run '%s' with %d results\n", runTitle, len(bulkItems))
			successfulRuns++
			totalResults += len(bulkItems)
			continue
		}

		var tgtRun *qase.Run
		var err error

		if config.Idempotent {
			// Create or get existing target run (idempotent)
			fmt.Printf("Creating or finding target run: %s\n", runTitle)
			tgtRun, err = qase.CreateOrGetRun(tgtClient, config.TargetProject, runTitle, runDescription)
			if err != nil {
				fmt.Printf("Failed to create/get target run for %s: %v\n", runTitle, err)
				failedRuns++
				continue
			}

			// For efficiency, skip detailed idempotency checks if we have many runs
			// Just check if run exists and has any results
			if len(resultsByRun) <= 20 {
				// Detailed idempotency check for small number of runs
				hasResults, err := qase.CheckRunHasResults(tgtClient, config.TargetProject, tgtRun.ID)
				if err != nil {
					fmt.Printf("Failed to check existing results for run %d: %v\n", tgtRun.ID, err)
					failedRuns++
					continue
				}

				if hasResults {
					fmt.Printf("Run %d already has results, filtering for new ones only...\n", tgtRun.ID)
					// Filter out results that already exist
					bulkItems, err = qase.FilterNewResults(tgtClient, config.TargetProject, tgtRun.ID, bulkItems)
					if err != nil {
						fmt.Printf("Failed to filter existing results for run %d: %v\n", tgtRun.ID, err)
						failedRuns++
						continue
					}
				}

				if len(bulkItems) == 0 {
					fmt.Printf("No new results to post for run %d (all already exist)\n", tgtRun.ID)
					successfulRuns++
					continue
				}

				// Post only new results to target run
				fmt.Printf("Posting %d new results to target run %d...\n", len(bulkItems), tgtRun.ID)
			} else {
				// For many runs, just post all results (less efficient but faster)
				fmt.Printf("Posting %d results to target run %d (bulk mode for %d runs)...\n", len(bulkItems), tgtRun.ID, len(resultsByRun))
			}
		} else {
			// Non-idempotent mode: always create new runs
			fmt.Printf("Creating target run: %s\n", runTitle)
			tgtRun, err = qase.CreateRun(tgtClient, config.TargetProject, runTitle, runDescription)
			if err != nil {
				fmt.Printf("Failed to create target run for %s: %v\n", runTitle, err)
				failedRuns++
				continue
			}

			// Post all results to target run
			fmt.Printf("Posting %d results to target run %d...\n", len(bulkItems), tgtRun.ID)
		}
		if err := qase.PostBulkResults(tgtClient, config.TargetProject, tgtRun.ID, bulkItems, config.BulkSize); err != nil {
			fmt.Printf("Failed to post results to run %d: %v\n", tgtRun.ID, err)
			failedRuns++
			continue
		}

		fmt.Printf("Successfully migrated run %d -> %d\n", runID, tgtRun.ID)
		successfulRuns++
		totalResults += len(bulkItems)
	}

	migrationDuration := time.Since(migrationStartTime)
	totalDuration := time.Since(startTime)

	// Create migration results
	migrationResults := MigrationResults{
		SourceProject:     config.SourceProject,
		TargetProject:     config.TargetProject,
		AfterDate:         config.AfterDate,
		MigrationTime:     time.Now(),
		DryRun:            config.DryRun,
		TotalRuns:         len(resultsByRun),
		SuccessfulRuns:    successfulRuns,
		FailedRuns:        failedRuns,
		TotalResults:      totalResults,
		TotalSkipped:      totalSkipped,
		TotalDuration:     totalDuration,
		RunsDuration:      resultsDuration,
		ResultsDuration:   resultsDuration,
		MigrationDuration: migrationDuration,
	}

	// Save migration results
	resultsJSON, err := json.MarshalIndent(migrationResults, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal migration results: %v", err)
	}

	if err := os.WriteFile("migration-results.json", resultsJSON, 0644); err != nil {
		log.Fatalf("Failed to write migration results: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== Migration Complete ===\n")
	fmt.Printf("Total runs processed: %d\n", len(resultsByRun))
	fmt.Printf("Successful migrations: %d\n", successfulRuns)
	fmt.Printf("Failed migrations: %d\n", failedRuns)
	fmt.Printf("Total results migrated: %d\n", totalResults)
	fmt.Printf("Total results skipped: %d\n", totalSkipped)
	fmt.Printf("Total execution time: %v\n", totalDuration)

	if config.DryRun {
		fmt.Println("\nDRY RUN MODE - No actual changes were made")
	} else {
		fmt.Println("\nMigration completed successfully!")
	}
}

func transformResults(results []qase.Result, caseMapping map[int]int, statusMap map[string]string) ([]qase.BulkItem, int) {
	var bulkItems []qase.BulkItem
	skipped := 0

	// Maximum time allowed by Qase API (1 year in seconds)
	const maxTimeSeconds = 31536000

	for _, result := range results {
		// Map case ID
		targetCaseID, exists := caseMapping[result.CaseID]
		if !exists {
			skipped++
			continue
		}

		// Map status if needed
		status := result.Status
		if mappedStatus, exists := statusMap[result.Status]; exists {
			status = mappedStatus
		}

		// Convert time from milliseconds to seconds and cap at maximum allowed
		var timeSeconds *int
		if result.TimeSpentMs > 0 {
			timeInSeconds := result.TimeSpentMs / 1000
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
			Comment: result.Comment,
			Time:    timeSeconds,
		}

		bulkItems = append(bulkItems, bulkItem)
	}

	return bulkItems, skipped
}

type Config struct {
	SourceToken   string
	SourceBaseURL string
	TargetToken   string
	TargetBaseURL string
	SourceProject string
	TargetProject string
	AfterDate     time.Time
	MatchMode     string
	CFID          int
	CSVFile       string
	DryRun        bool
	BulkSize      int
	StatusMap     map[string]string
	Idempotent    bool
}

func loadConfig() Config {
	config := Config{
		SourceToken:   getEnv("QASE_SOURCE_API_TOKEN", ""),
		SourceBaseURL: getEnv("QASE_SOURCE_API_BASE", "https://api.qase.io"),
		TargetToken:   getEnv("QASE_TARGET_API_TOKEN", ""),
		TargetBaseURL: getEnv("QASE_TARGET_API_BASE", "https://api.qase.io"),
		SourceProject: getEnv("QASE_SOURCE_PROJECT", ""),
		TargetProject: getEnv("QASE_TARGET_PROJECT", ""),
		MatchMode:     getEnv("QASE_MATCH_MODE", "custom_field"),
		CSVFile:       getEnv("QASE_CSV_FILE", "mapping.csv"),
		DryRun:        getEnv("QASE_DRY_RUN", "false") == "true",
		BulkSize:      100,
		StatusMap:     make(map[string]string),
		Idempotent:    getEnv("QASE_IDEMPOTENT", "true") == "true",
	}

	if config.SourceToken == "" {
		log.Fatal("QASE_SOURCE_API_TOKEN is required")
	}
	if config.TargetToken == "" {
		log.Fatal("QASE_TARGET_API_TOKEN is required")
	}
	if config.SourceProject == "" {
		log.Fatal("QASE_SOURCE_PROJECT is required")
	}
	if config.TargetProject == "" {
		log.Fatal("QASE_TARGET_PROJECT is required")
	}

	// Parse after date (Unix timestamp)
	afterDateStr := getEnv("QASE_AFTER_DATE", "1755500400")
	afterDate, err := utils.ParseUnixTimestamp(afterDateStr)
	if err != nil {
		log.Fatalf("Invalid QASE_AFTER_DATE format (must be Unix timestamp): %v", err)
	}
	config.AfterDate = afterDate

	// Parse CF ID
	if config.MatchMode == "custom_field" {
		cfIDStr := getEnv("QASE_CF_ID", "2")
		if cfIDStr != "" {
			if _, err := fmt.Sscanf(cfIDStr, "%d", &config.CFID); err != nil {
				log.Fatalf("Invalid QASE_CF_ID: %s", cfIDStr)
			}
		}
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
