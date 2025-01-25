package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Test endpoints to discover
	endpoints := []struct {
		name string
		url  string
	}{
		{"Simple User Creation", "http://localhost:8081/api/users"},
		{"Complex User Creation", "http://localhost:8081/api/users/complex"},
		{"Product Creation", "http://localhost:8081/api/products"},
		{"Batch User Creation", "http://localhost:8081/api/batch/users"},
	}

	// Try to discover each endpoint
	for _, endpoint := range endpoints {
		fmt.Printf("\n=== Testing %s ===\n", endpoint.name)
		schema, err := discoverEndpoint(endpoint.url)
		if err != nil {
			log.Printf("Failed to discover %s: %v\n", endpoint.name, err)
			continue
		}

		// Pretty print the schema
		prettySchema, _ := json.MarshalIndent(schema, "", "  ")
		fmt.Printf("Discovered Schema:\n%s\n", string(prettySchema))
	}
}

func discoverEndpoint(url string) (map[string]interface{}, error) {
	// Create discovery request
	reqBody := map[string]interface{}{
		"url":           url,
		"method":        "POST",
		"maxIterations": 10,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Send request to our discovery API
	resp, err := http.Post("http://localhost:8080/api/discover", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery failed: %s", string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
