package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MyGekkoClient struct {
	baseURL    *url.URL
	username   string
	password   string
	httpClient *http.Client
}

func NewMyGekkoClient(cfg MyGekkoConfig) *MyGekkoClient {
	baseURL := &url.URL{
		Scheme: "http",
		Host:   cfg.Host,
		Path:   "/api/v1/",
	}

	return &MyGekkoClient{
		baseURL:  baseURL,
		username: cfg.Username,
		password: cfg.Password,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *MyGekkoClient) buildURL(endpoint string, extraParams url.Values) string {
	u := c.baseURL.JoinPath(endpoint)

	params := url.Values{}
	params.Set("username", c.username)
	params.Set("password", c.password)

	for key, values := range extraParams {
		for _, v := range values {
			params.Add(key, v)
		}
	}

	u.RawQuery = params.Encode()
	return u.String()
}

func (c *MyGekkoClient) Get(endpoint string) (map[string]any, error) {
	resp, err := c.httpClient.Get(c.buildURL(endpoint, nil))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

func (c *MyGekkoClient) GetStatus(categories []string) (map[string]any, error) {
	if len(categories) == 0 {
		return c.Get("var/status")
	}

	// Query each category individually and merge results
	result := make(map[string]any)
	for _, cat := range categories {
		endpoint := fmt.Sprintf("var/%s/status", cat)
		catResult, err := c.Get(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", cat, err)
		}
		result[cat] = catResult
	}

	return result, nil
}

func (c *MyGekkoClient) SetValue(category, item, value string) error {
	endpoint := fmt.Sprintf("var/%s/%s/scmd/set", category, item)
	params := url.Values{}
	params.Set("value", value)

	resp, err := c.httpClient.Get(c.buildURL(endpoint, params))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	bodyStr := strings.TrimSpace(string(body))
	slog.Debug("SetValue response", "category", category, "item", item, "status", resp.StatusCode, "body", bodyStr)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d: %s", resp.StatusCode, bodyStr)
	}

	if bodyStr != "OK" {
		return fmt.Errorf("unexpected response: %s", bodyStr)
	}

	return nil
}

func (c *MyGekkoClient) GetGekkoName() (string, error) {
	result, err := c.Get("var/globals/network/gekkoname/status")
	if err != nil {
		return "", err
	}

	value, ok := result["value"].(string)
	if !ok {
		return "", fmt.Errorf("gekkoname not found in response")
	}

	return value, nil
}

func (c *MyGekkoClient) GetDefinitions() (map[string]any, error) {
	return c.Get("var")
}
