package qase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
)

// Run represents a test run
type Run struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
}

// RunListResponse represents the API response for run list
type RunListResponse struct {
	Status bool `json:"status"`
	Result struct {
		Total    int   `json:"total"`
		Entities []Run `json:"entities"`
	} `json:"result"`
}

// CreateRunRequest represents a request to create a new run
type CreateRunRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Include     string `json:"include"`
}

// CreateRunResponse represents the response from creating a run
type CreateRunResponse struct {
	Status bool `json:"status"`
	Result struct {
		ID int `json:"id"`
	} `json:"result"`
}

// GetRuns fetches all runs for a project with pagination and date filtering
func GetRuns(c *api.Client, project string, afterDate time.Time) ([]Run, error) {
	var allRuns []Run
	page := 1
	limit := 100

	for {
		// Build URL with pagination
		u := fmt.Sprintf("/run/%s?limit=%d&page=%d", project, limit, page)

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

		var response RunListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Filter runs by date
		for _, run := range response.Result.Entities {
			if run.CreatedAt.After(afterDate) {
				allRuns = append(allRuns, run)
			}
		}

		fmt.Printf("Fetched page %d: %d runs (filtered: %d after %s)\n",
			page, len(response.Result.Entities), len(allRuns), afterDate.Format("2006-01-02"))

		// Check if we've fetched all runs
		if len(response.Result.Entities) < limit {
			break
		}

		page++
	}

	fmt.Printf("Total runs found after %s: %d\n", afterDate.Format("2006-01-02"), len(allRuns))
	return allRuns, nil
}

// CreateRun creates a new test run in the target project
func CreateRun(c *api.Client, project string, title, description string) (*Run, error) {
	reqBody := CreateRunRequest{
		Title:       title,
		Description: description,
		Include:     "cases",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/run/%s", project)
	req, err := c.NewRequest("POST", path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CreateRunResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Status {
		return nil, fmt.Errorf("run creation failed: %s", string(body))
	}

	// Fetch the created run details
	run, err := GetRunByID(c, project, response.Result.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created run: %w", err)
	}

	fmt.Printf("Created run: %s (ID: %d)\n", run.Title, run.ID)
	return run, nil
}

// GetRunByID fetches a specific run by ID
func GetRunByID(c *api.Client, project string, runID int) (*Run, error) {
	path := fmt.Sprintf("/run/%s/%d", project, runID)

	req, err := c.NewRequest("GET", path, nil)
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

	var response struct {
		Status bool `json:"status"`
		Result Run  `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Status {
		return nil, fmt.Errorf("failed to fetch run: %s", string(body))
	}

	return &response.Result, nil
}
