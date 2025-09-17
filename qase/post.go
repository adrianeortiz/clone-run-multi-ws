package qase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/adrianeortiz/clone-run-multi-ws/api"
)

// BulkItem represents a single result item for bulk posting
type BulkItem struct {
	CaseID  int    `json:"case_id"`
	Status  string `json:"status"`
	Time    *int   `json:"time,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// BulkRequest represents the bulk results request
type BulkRequest struct {
	Results []BulkItem `json:"results"`
}

// BulkResponse represents the bulk results response
type BulkResponse struct {
	Status bool `json:"status"`
	Result struct {
		Bulk []struct {
			ID     int  `json:"id"`
			Status bool `json:"status"`
		} `json:"bulk"`
	} `json:"result"`
}

// PostBulkResults posts results in chunks with retries
func PostBulkResults(c *api.Client, project string, runID int, items []BulkItem, chunkSize int) error {
	if len(items) == 0 {
		fmt.Println("No items to post")
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = 200
	}

	totalChunks := (len(items) + chunkSize - 1) / chunkSize
	fmt.Printf("Posting %d items in %d chunks of %d\n", len(items), totalChunks, chunkSize)

	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}

		chunk := items[i:end]
		chunkNum := (i / chunkSize) + 1

		fmt.Printf("Posting chunk %d/%d (%d items)\n", chunkNum, totalChunks, len(chunk))

		if err := postChunkWithRetry(c, project, runID, chunk, chunkNum, totalChunks); err != nil {
			return fmt.Errorf("failed to post chunk %d: %w", chunkNum, err)
		}
	}

	fmt.Println("All chunks posted successfully")
	return nil
}

// postChunkWithRetry posts a single chunk with exponential backoff retries
func postChunkWithRetry(c *api.Client, project string, runID int, chunk []BulkItem, chunkNum, totalChunks int) error {
	backoffDelays := []time.Duration{200 * time.Millisecond, 1 * time.Second, 3 * time.Second, 5 * time.Second}

	for attempt := 0; attempt < len(backoffDelays); attempt++ {
		err := postChunk(c, project, runID, chunk)
		if err == nil {
			return nil
		}

		// Check if it's a retryable error
		if !isRetryableError(err) {
			return err
		}

		if attempt < len(backoffDelays)-1 {
			delay := backoffDelays[attempt]
			fmt.Printf("Chunk %d/%d attempt %d failed, retrying in %v: %v\n", chunkNum, totalChunks, attempt+1, delay, err)
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("chunk %d/%d failed after %d attempts", chunkNum, totalChunks, len(backoffDelays))
}

// postChunk posts a single chunk of results
func postChunk(c *api.Client, project string, runID int, chunk []BulkItem) error {
	reqBody := BulkRequest{Results: chunk}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/result/%s/%d/results", project, runID)
	req, err := c.NewV2Request("POST", path, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response BulkResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Status {
		return fmt.Errorf("bulk request failed: %s", string(body))
	}

	fmt.Printf("Chunk posted successfully: %d results\n", len(chunk))
	return nil
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// Check for HTTP 429 (rate limit) or 5xx errors
	if httpErr, ok := err.(*httpError); ok {
		return httpErr.StatusCode == 429 || (httpErr.StatusCode >= 500 && httpErr.StatusCode < 600)
	}
	return false
}

// httpError represents an HTTP error
type httpError struct {
	StatusCode int
	Message    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}
