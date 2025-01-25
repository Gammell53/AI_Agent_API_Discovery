package handlers

import (
	"ai-agent-api-discovery/agent"
	"ai-agent-api-discovery/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DiscoverHandler handles the POST /api/discover endpoint
func DiscoverHandler(c *gin.Context) {
	var req models.DiscoverRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default values if not provided
	if req.Method == "" {
		req.Method = "POST"
	}
	if req.MaxIterations == 0 {
		req.MaxIterations = 10
	}

	// Create and run the discovery agent
	discoveryAgent, err := agent.NewDeepseekAgent(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize discovery agent: " + err.Error()})
		return
	}

	schema, err := discoveryAgent.RunDiscovery()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, schema)
}
