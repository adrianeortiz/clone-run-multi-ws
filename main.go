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
	fmt.Printf("Building mapping using %s mode...\n", config.MatchMode)
	caseMapping, err := mapping.Build(
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

	// Fetch source runs after the specified date
	fmt.Printf("Fetching runs from source project after %s...\n", config.AfterDate.Format("2006-01-02"))
	srcRuns, err := qase.GetRuns(srcClient, config.SourceProject, config.AfterDate)
	if err != nil {
		log.Fatalf("Failed to fetch source runs: %v", err)
	}

	if len(srcRuns) == 0 {
		fmt.Println("No runs found after the specified date. Nothing to migrate.")
		return
	}

	fmt.Printf("Found %d runs to migrate\n", len(srcRuns))

	// Write mapping artifact
	if err := writeMappingArtifact(caseMapping); err != nil {
		log.Printf("Warning: Failed to write mapping artifact: %v", err)
	}

	// Process each run
	totalResults := 0
	totalSkipped := 0
	successfulRuns := 0
	failedRuns := 0

	for i, srcRun := range srcRuns {
		fmt.Printf("\n--- Processing run %d/%d: %s (ID: %d) ---\n",
			i+1, len(srcRuns), srcRun.Title, srcRun.ID)

		// Fetch results for this run
		fmt.Printf("Fetching results from run %d...\n", srcRun.ID)
		results, err := qase.GetRunResults(srcClient, config.SourceProject, srcRun.ID)
		if err != nil {
			log.Printf("Failed to fetch results for run %d: %v", srcRun.ID, err)
			failedRuns++
			continue
		}

		if len(results) == 0 {
			fmt.Printf("No results found in run %d, skipping\n", srcRun.ID)
			continue
		}

		// Transform results to target case IDs
		fmt.Println("Transforming results...")
		bulkItems, skipped := transformResults(results, caseMapping, config.StatusMap)

		fmt.Printf("Prepared %d results for posting, skipped %d unmapped results\n", len(bulkItems), skipped)
		totalResults += len(bulkItems)
		totalSkipped += skipped

		// Handle dry run mode
		if config.DryRun {
			fmt.Printf("DRY RUN MODE - Would create run '%s' with %d results\n", srcRun.Title, len(bulkItems))
			continue
		}

		// Create target run
		fmt.Printf("Creating target run: %s\n", srcRun.Title)
		tgtRun, err := qase.CreateRun(tgtClient, config.TargetProject, srcRun.Title, srcRun.Description)
		if err != nil {
			log.Printf("Failed to create target run for %s: %v", srcRun.Title, err)
			failedRuns++
			continue
		}

		// Post results to target run
		fmt.Printf("Posting %d results to target run %d...\n", len(bulkItems), tgtRun.ID)
		if err := qase.PostBulkResults(tgtClient, config.TargetProject, tgtRun.ID, bulkItems, config.BulkSize); err != nil {
			log.Printf("Failed to post results to run %d: %v", tgtRun.ID, err)
			failedRuns++
			continue
		}

		successfulRuns++
		fmt.Printf("Successfully migrated run %d -> %d\n", srcRun.ID, tgtRun.ID)
	}

	// Print summary
	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Total runs processed: %d\n", len(srcRuns))
	fmt.Printf("Successful migrations: %d\n", successfulRuns)
	fmt.Printf("Failed migrations: %d\n", failedRuns)
	fmt.Printf("Total results migrated: %d\n", totalResults)
	fmt.Printf("Total results skipped: %d\n", totalSkipped)

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
	DryRun    bool
	BulkSize  int
	StatusMap map[string]string
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	config := &Config{
		SourceBaseURL: getEnvDefault("QASE_SOURCE_API_BASE", "https://api.qase.io"),
		TargetBaseURL: getEnvDefault("QASE_TARGET_API_BASE", "https://api.qase.io"),
		MatchMode:     getEnvDefault("QASE_MATCH_MODE", "custom_field"),
		DryRun:        getEnvDefault("QASE_DRY_RUN", "true") == "true",
		BulkSize:      getIntDefault("QASE_BULK_SIZE", 200),
	}

	// Required environment variables
	config.SourceToken = mustEnv("QASE_SOURCE_API_TOKEN")
	config.SourceProject = mustEnv("QASE_SOURCE_PROJECT")

	config.TargetToken = mustEnv("QASE_TARGET_API_TOKEN")
	config.TargetProject = mustEnv("QASE_TARGET_PROJECT")

	// Date filtering - default to August 18th, 2025
	afterDateStr := getEnvDefault("QASE_AFTER_DATE", "2025-08-18T00:00:00Z")
	afterDate, err := time.Parse(time.RFC3339, afterDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid QASE_AFTER_DATE format (use RFC3339): %w", err)
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

		bulkItem := qase.BulkItem{
			CaseID:  targetCaseID,
			Status:  status,
			Time:    result.Time,
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
