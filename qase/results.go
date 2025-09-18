package qase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
)

// Result represents a test result
type Result struct {
	Hash        string `json:"hash"`
	Comment     string `json:"comment,omitempty"`
	RunID       int    `json:"run_id"`
	CaseID      int    `json:"case_id"`
	Status      string `json:"status"`
	Time        *int   `json:"time,omitempty"`
	Steps       []Step `json:"steps,omitempty"`
	IsAPIResult bool   `json:"is_api_result"`
	TimeSpentMs int    `json:"time_spent_ms"`
	EndTime     string `json:"end_time"`
}

// Step represents a test step
type Step struct {
	Status      int           `json:"status"`
	Comment     string        `json:"comment,omitempty"`
	Attachments []interface{} `json:"attachments,omitempty"`
	Position    int           `json:"position"`
}

// ResultListResponse represents the API response for result list
type ResultListResponse struct {
	Status bool `json:"status"`
	Result struct {
		Total    int      `json:"total"`
		Entities []Result `json:"entities"`
	} `json:"result"`
}

// GetRunResults fetches all results for a specific run with pagination
func GetRunResults(c *api.Client, project string, runID int) ([]Result, error) {
	var allResults []Result
	page := 1
	limit := 100

	for {
		// Build URL with pagination and run filter
		u := fmt.Sprintf("/result/%s?limit=%d&page=%d&run_id[]=%d", project, limit, page, runID)

		req, err := c.NewRequest("GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var response ResultListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Add results to slice
		allResults = append(allResults, response.Result.Entities...)

		fmt.Printf("Fetched page %d: %d results (total so far: %d)\n", page, len(response.Result.Entities), len(allResults))

		// Check if we've fetched all results
		if len(response.Result.Entities) < limit {
			break
		}

		page++
	}

	fmt.Printf("Total results fetched: %d\n", len(allResults))
	return allResults, nil
}

// GetResultsAfterDate fetches all results after a specific date using the bulk API
func GetResultsAfterDate(c *api.Client, project string, afterDate time.Time) ([]Result, error) {
	var allResults []Result
	offset := 0
	limit := 100

	fmt.Printf("Fetching all results for project %s after %s...\n", project, afterDate.Format("2006-01-02"))

	pageCount := 0
	for {
		pageCount++
		// Build URL with pagination and date filter using offset instead of page
		u := fmt.Sprintf("/result/%s?limit=%d&offset=%d&created_after=%s", 
			project, limit, offset, afterDate.Format("2006-01-02T15:04:05Z"))

		fmt.Printf("API Call %d: %s\n", pageCount, u)

		req, err := c.NewRequest("GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		start := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		apiDuration := time.Since(start)
		fmt.Printf("API call %d completed in %v\n", pageCount, apiDuration)

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var response ResultListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Add results to slice
		allResults = append(allResults, response.Result.Entities...)

		fmt.Printf("Page %d: %d results (total: %d) - API took %v\n", 
			pageCount, len(response.Result.Entities), len(allResults), apiDuration)

		// Check if we've fetched all results
		if len(response.Result.Entities) < limit {
			fmt.Printf("Reached end of results (got %d < limit %d)\n", len(response.Result.Entities), limit)
			break
		}

		offset += limit
		
		// Add a small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Total results fetched after %s: %d (in %d API calls)\n", afterDate.Format("2006-01-02"), len(allResults), pageCount)
	return allResults, nil
}

// GetResultsForRuns fetches results for specific run IDs in one API call
func GetResultsForRuns(c *api.Client, project string, runIDs []int) ([]Result, error) {
	var allResults []Result
	offset := 0
	limit := 100

	fmt.Printf("Fetching results for %d runs in project %s...\n", len(runIDs), project)

	// Build run_id filter parameter
	var runIDParams []string
	for _, runID := range runIDs {
		runIDParams = append(runIDParams, fmt.Sprintf("run_id[]=%d", runID))
	}
	runIDFilter := strings.Join(runIDParams, "&")

	pageCount := 0
	for {
		pageCount++
		// Build URL with pagination and run ID filters
		u := fmt.Sprintf("/result/%s?limit=%d&offset=%d&%s",
			project, limit, offset, runIDFilter)

		fmt.Printf("API Call %d: %s\n", pageCount, u)

		req, err := c.NewRequest("GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		start := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		apiDuration := time.Since(start)
		fmt.Printf("API call %d completed in %v\n", pageCount, apiDuration)

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var response ResultListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Add results to slice
		allResults = append(allResults, response.Result.Entities...)

		fmt.Printf("Page %d: %d results (total: %d) - API took %v\n",
			pageCount, len(response.Result.Entities), len(allResults), apiDuration)

		// Check if we've fetched all results
		if len(response.Result.Entities) < limit {
			fmt.Printf("Reached end of results (got %d < limit %d)\n", len(response.Result.Entities), limit)
			break
		}

		offset += limit

		// Add a small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Total results fetched for %d runs: %d (in %d API calls)\n", len(runIDs), len(allResults), pageCount)
	return allResults, nil
}
