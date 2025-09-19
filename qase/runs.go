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
	ID             int                     `json:"id"`
	Title          string                  `json:"title"`
	Description    *string                 `json:"description"`
	Status         int                     `json:"status"`
	StatusText     string                  `json:"status_text"`
	StartTime      time.Time               `json:"start_time"`
	EndTime        time.Time               `json:"end_time"`
	Public         bool                    `json:"public"`
	Stats          map[string]interface{}  `json:"stats"`
	TimeSpent      int                     `json:"time_spent"`
	ElapsedTime    int                     `json:"elapsed_time"`
	UserID         int                     `json:"user_id"`
	Environment    *string                 `json:"environment"`
	Milestone      *map[string]interface{} `json:"milestone"`
	CustomFields   []interface{}           `json:"custom_fields"`
	Tags           []interface{}           `json:"tags"`
	Configurations []interface{}           `json:"configurations"`
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
