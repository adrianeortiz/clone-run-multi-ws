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
	offset := 0
	limit := 100
	maxPages := 1000 // Safety limit to prevent infinite loops

	fmt.Printf("Fetching cases for project %s...\n", project)

	for page := 1; page <= maxPages; page++ {
		// Build URL with offset-based pagination
		u := fmt.Sprintf("/case/%s?limit=%d&offset=%d", project, limit, offset)

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

		// Check if we got any new cases
		newCasesCount := 0
		for _, case_ := range response.Result.Entities {
			if _, exists := cases[case_.ID]; !exists {
				cases[case_.ID] = case_
				newCasesCount++
			}
		}

		fmt.Printf("Page %d (offset %d): %d cases returned, %d new cases (total unique: %d)\n",
			page, offset, len(response.Result.Entities), newCasesCount, len(cases))

		// Check if we've fetched all cases
		if len(response.Result.Entities) < limit {
			fmt.Printf("Reached end of cases (got %d < limit %d)\n", len(response.Result.Entities), limit)
			break
		}

		// Safety check: if we got no new cases, we might be in a loop
		if newCasesCount == 0 {
			fmt.Printf("Warning: No new cases found on page %d, stopping to prevent infinite loop\n", page)
			break
		}

		offset += limit
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no cases found for project %s", project)
	}

	fmt.Printf("Total unique cases fetched: %d\n", len(cases))
	return cases, nil
}
