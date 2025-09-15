package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// CustomFieldRequest represents the request to create a custom field
type CustomFieldRequest struct {
	Title        string `json:"title"`
	Type         string `json:"type"`
	Placeholder  string `json:"placeholder,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	IsFilterable bool   `json:"is_filterable"`
	IsVisible    bool   `json:"is_visible"`
	IsRequired   bool   `json:"is_required"`
	ProjectCode  string `json:"project_code"`
}

// CustomFieldResponse represents the response from creating a custom field
type CustomFieldResponse struct {
	Status bool `json:"status"`
	Result struct {
		ID int `json:"id"`
	} `json:"result"`
}

func main() {
	// API credentials
	apiToken := "192913a10e3eef195106f3c619ff7a4b1293cafbc17b3c147e9c5e4d9f374366"
	apiBase := "https://api.qase.io"
	project := "INTEGRATIO"

	// Create custom field request
	customField := CustomFieldRequest{
		Title:        "Target Case ID",
		Type:         "number",
		Placeholder:  "Enter the corresponding target case ID",
		IsFilterable: true,
		IsVisible:    true,
		IsRequired:   false,
		ProjectCode:  project,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(customField)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/custom_field", apiBase)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	// Set headers
	req.Header.Set("X-Token", apiToken)
	req.Header.Set("Content-Type", "application/json")
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
	var response CustomFieldResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		fmt.Printf("Response: %s\n", string(body))
		os.Exit(1)
	}

	if !response.Status {
		fmt.Printf("Custom field creation failed: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Printf("✅ Custom field created successfully!\n")
	fmt.Printf("Field ID: %d\n", response.Result.ID)
	fmt.Printf("Field Name: %s\n", customField.Title)
	fmt.Printf("Field Type: %s\n", customField.Type)
	fmt.Printf("\nUse this Field ID in your GitHub Actions workflow:\n")
	fmt.Printf("QASE_CF_ID: %d\n", response.Result.ID)
}
