package main

import (
	"flag"
	"log"
	"os"

	"ai-agent-api-discovery/handlers"
	"ai-agent-api-discovery/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// Define command-line flags
	apiKey := flag.String("api-key", "", "Deepseek API key (required)")
	port := flag.String("port", "8080", "Port to run the server on")
	flag.Parse()

	// Validate API key
	if *apiKey == "" {
		log.Fatal("Deepseek API key is required. Use -api-key flag to provide it.")
	}

	// Initialize logger
	if err := utils.InitLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer utils.CloseLogger()

	// Set API key in environment
	os.Setenv("DEEPSEEK_API_KEY", *apiKey)

	// Initialize Gin router
	router := gin.Default()

	// Setup routes
	setupRoutes(router)

	// Start server
	utils.Logger.Printf("Starting server on :%s", *port)
	if err := router.Run(":" + *port); err != nil {
		utils.Logger.Fatalf("Failed to start server: %v", err)
	}
}

func setupRoutes(router *gin.Engine) {
	// API group
	api := router.Group("/api")
	{
		api.POST("/discover", handlers.DiscoverHandler)
	}
}
