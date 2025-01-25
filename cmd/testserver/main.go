package main

import (
	"ai-agent-api-discovery/testapi"
	"log"
)

func main() {
	// Start test server on port 8081 (different from our main API)
	router := testapi.StartTestServer(8081)
	
	log.Println("Starting test API server on :8081")
	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start test server: %v", err)
	}
} 