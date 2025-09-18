package qase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
)

// Result represents a test result
type Result struct {
	Hash      string `json:"hash"`
	Comment   string `json:"comment,omitempty"`
	RunID     int    `json:"run_id"`
	CaseID    int    `json:"case_id"`
	Status    string `json:"status"`
	Time      *int   `json:"time,omitempty"`
	Steps     []Step `json:"steps,omitempty"`
	IsAPIResult bool `json:"is_api_result"`
	TimeSpentMs int  `json:"time_spent_ms"`
	EndTime   string `json:"end_time"`
}

// Step represents a test step
type Step struct {
	Status     int    `json:"status"`
	Comment    string `json:"comment,omitempty"`
	Attachments []interface{} `json:"attachments,omitempty"`
	Position   int    `json:"position"`
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
	page := 1
	limit := 100

	fmt.Printf("Fetching all results for project %s after %s...\n", project, afterDate.Format("2006-01-02"))

	for {
		// Build URL with pagination and date filter
		u := fmt.Sprintf("/result/%s?limit=%d&page=%d&created_after=%s", project, limit, page, afterDate.Format("2006-01-02T15:04:05Z"))

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

	fmt.Printf("Total results fetched after %s: %d\n", afterDate.Format("2006-01-02"), len(allResults))
	return allResults, nil
}
