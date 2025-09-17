package qase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
)

// Case represents a Qase test case
type Case struct {
	ID           int           `json:"id"`
	Title        string        `json:"title"`
	CustomFields []CustomField `json:"custom_fields"`
}

// CustomField represents a custom field in a Qase case
type CustomField struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

// CaseListResponse represents the API response for case list
type CaseListResponse struct {
	Status bool `json:"status"`
	Result struct {
		Total    int    `json:"total"`
		Entities []Case `json:"entities"`
	} `json:"result"`
}

// GetCases fetches all cases for a project with pagination
func GetCases(c *api.Client, project string) (map[int]Case, error) {
	cases := make(map[int]Case)
	page := 1
	limit := 1000

	for {
		// Build URL with pagination
		u := fmt.Sprintf("/case/%s?limit=%d&page=%d", project, limit, page)

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

		var response CaseListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Add cases to map
		for _, case_ := range response.Result.Entities {
			cases[case_.ID] = case_
		}

		fmt.Printf("Fetched page %d: %d cases (total so far: %d)\n", page, len(response.Result.Entities), len(cases))

		// Check if we've fetched all cases
		if len(response.Result.Entities) < limit {
			break
		}

		page++
	}

	fmt.Printf("Total cases fetched: %d\n", len(cases))
	return cases, nil
}
