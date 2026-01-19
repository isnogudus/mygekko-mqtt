package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestNewMyGekkoClient(t *testing.T) {
	cfg := MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "testuser",
		Password: "testpass",
	}

	client := NewMyGekkoClient(cfg)

	if client.baseURL.Host != "mygekko.example.com" {
		t.Errorf("expected host 'mygekko.example.com', got '%s'", client.baseURL.Host)
	}
	if client.baseURL.Scheme != "http" {
		t.Errorf("expected scheme 'http', got '%s'", client.baseURL.Scheme)
	}
	if client.baseURL.Path != "/api/v1/" {
		t.Errorf("expected path '/api/v1/', got '%s'", client.baseURL.Path)
	}
	if client.username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", client.username)
	}
	if client.password != "testpass" {
		t.Errorf("expected password 'testpass', got '%s'", client.password)
	}
}

func TestBuildURL_Basic(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user",
		Password: "pass",
	})

	result := client.buildURL("var/status", nil)

	if !strings.Contains(result, "http://mygekko.example.com/api/v1/var/status") {
		t.Errorf("unexpected URL base: %s", result)
	}
	if !strings.Contains(result, "username=user") {
		t.Errorf("URL should contain username parameter: %s", result)
	}
	if !strings.Contains(result, "password=pass") {
		t.Errorf("URL should contain password parameter: %s", result)
	}
}

func TestBuildURL_WithExtraParams(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user",
		Password: "pass",
	})

	params := url.Values{}
	params.Set("value", "42")

	result := client.buildURL("var/blinds/item0/scmd/set", params)

	if !strings.Contains(result, "value=42") {
		t.Errorf("URL should contain value parameter: %s", result)
	}
	if !strings.Contains(result, "username=user") {
		t.Errorf("URL should contain username parameter: %s", result)
	}
}

func TestBuildURL_SpecialCharacters(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user@domain",
		Password: "p@ss&word=special",
	})

	result := client.buildURL("var/status", nil)

	// Check that special characters are URL-encoded
	if !strings.Contains(result, "username=user%40domain") {
		t.Errorf("username with @ should be URL-encoded: %s", result)
	}
	if !strings.Contains(result, "password=p%40ss%26word%3Dspecial") {
		t.Errorf("password with special chars should be URL-encoded: %s", result)
	}
}

func TestBuildURL_MultipleExtraParams(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user",
		Password: "pass",
	})

	params := url.Values{}
	params.Set("value", "100")
	params.Add("option", "fast")

	result := client.buildURL("endpoint", params)

	if !strings.Contains(result, "value=100") {
		t.Errorf("URL should contain value parameter: %s", result)
	}
	if !strings.Contains(result, "option=fast") {
		t.Errorf("URL should contain option parameter: %s", result)
	}
}

func TestBuildURL_EndpointPath(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user",
		Password: "pass",
	})

	testCases := []struct {
		endpoint string
		expected string
	}{
		{"var", "/api/v1/var"},
		{"var/status", "/api/v1/var/status"},
		{"var/blinds/item0/status", "/api/v1/var/blinds/item0/status"},
		{"var/globals/network/gekkoname/status", "/api/v1/var/globals/network/gekkoname/status"},
	}

	for _, tc := range testCases {
		result := client.buildURL(tc.endpoint, nil)
		if !strings.Contains(result, tc.expected) {
			t.Errorf("buildURL(%s): expected path '%s' in URL, got: %s", tc.endpoint, tc.expected, result)
		}
	}
}

func TestBuildURL_ValueWithSpaces(t *testing.T) {
	client := NewMyGekkoClient(MyGekkoConfig{
		Host:     "mygekko.example.com",
		Username: "user",
		Password: "pass",
	})

	params := url.Values{}
	params.Set("value", "hello world")

	result := client.buildURL("endpoint", params)

	// Spaces should be encoded as + or %20
	if !strings.Contains(result, "value=hello+world") && !strings.Contains(result, "value=hello%20world") {
		t.Errorf("value with space should be URL-encoded: %s", result)
	}
}
