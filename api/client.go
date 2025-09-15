package api

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Client wraps HTTP client with Qase API configuration
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient creates a new Qase API client
func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "https://api.qase.io"
	}

	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewRequest creates a new HTTP request with Qase API headers
func (c *Client) NewRequest(method, path string, body []byte) (*http.Request, error) {
	url := fmt.Sprintf("%s/v1%s", c.BaseURL, path)

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}
