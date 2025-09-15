package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// CustomFieldListResponse represents the response from listing custom fields
type CustomFieldListResponse struct {
	Status bool `json:"status"`
	Result struct {
		Entities []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"entities"`
	} `json:"result"`
}

func main() {
	// API credentials from environment variables
	apiToken := os.Getenv("QASE_SOURCE_API_TOKEN")
	if apiToken == "" {
		fmt.Println("Error: QASE_SOURCE_API_TOKEN environment variable is required")
		os.Exit(1)
	}
	
	apiBase := os.Getenv("QASE_SOURCE_API_BASE")
	if apiBase == "" {
		apiBase = "https://api.qase.io"
	}
	
	project := os.Getenv("QASE_SOURCE_PROJECT")
	if project == "" {
		fmt.Println("Error: QASE_SOURCE_PROJECT environment variable is required")
		os.Exit(1)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/custom_field/%s", apiBase, project)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	// Set headers
	req.Header.Set("X-Token", apiToken)
	req.Header.Set("Accept", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		os.Exit(1)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API request failed with status %d\n", resp.StatusCode)
		fmt.Printf("Response: %s\n", string(body))
		os.Exit(1)
	}

	// Parse response
	var response CustomFieldListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		fmt.Printf("Response: %s\n", string(body))
		os.Exit(1)
	}

	if !response.Status {
		fmt.Printf("Failed to list custom fields: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Printf("✅ Found %d custom fields in project %s:\n\n", len(response.Result.Entities), project)

	if len(response.Result.Entities) == 0 {
		fmt.Printf("No custom fields found. You'll need to create one manually in the Qase UI.\n")
		fmt.Printf("Go to: https://app.qase.io/project/%s/settings/custom-fields\n", project)
		fmt.Printf("Create a custom field with type 'Number' and name 'Target Case ID'\n")
	} else {
		for _, field := range response.Result.Entities {
			fmt.Printf("ID: %d | Name: %s | Type: %s\n", field.ID, field.Title, field.Type)
		}
	}
}
