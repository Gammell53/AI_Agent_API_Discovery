package llm

import (
	"ai-agent-api-discovery/models"
	"ai-agent-api-discovery/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// DeepseekClient handles communication with the Deepseek API
type DeepseekClient struct {
	apiKey     string
	apiBaseURL string
	client     *http.Client
}

// DeepseekRequest represents a request to the Deepseek API
type DeepseekRequest struct {
	Model       string           `json:"model"`
	Messages    []models.Message `json:"messages"`
	Temperature float64          `json:"temperature"`
	MaxTokens   int              `json:"max_tokens"`
	Stream      bool             `json:"stream"`
}

// DeepseekResponse represents a response from the Deepseek API
type DeepseekResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewDeepseekClient creates a new Deepseek API client
func NewDeepseekClient() (*DeepseekClient, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY environment variable is not set")
	}

	return &DeepseekClient{
		apiKey:     apiKey,
		apiBaseURL: "https://api.deepseek.com", // No /v1 needed per docs
		client:     &http.Client{},
	}, nil
}

// Complete sends a completion request to the Deepseek API
func (c *DeepseekClient) Complete(messages []models.Message) (*models.Message, error) {
	utils.Logger.Printf("Sending completion request to Deepseek API with %d messages", len(messages))

	reqBody := DeepseekRequest{
		Model:       "deepseek-chat",
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	utils.Logger.Printf("Request body:\n%s", string(jsonData))

	req, err := http.NewRequest("POST", c.apiBaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	utils.Logger.Printf("Sending request to %s", req.URL)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	utils.Logger.Printf("Raw response from API:\n%s", string(body))

	var deepseekResp DeepseekResponse
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if deepseekResp.Error != nil {
		utils.Logger.Printf("API returned error: %s", deepseekResp.Error.Message)
		return nil, fmt.Errorf("Deepseek API error: %s", deepseekResp.Error.Message)
	}

	if len(deepseekResp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	response := &models.Message{
		Role:    deepseekResp.Choices[0].Message.Role,
		Content: deepseekResp.Choices[0].Message.Content,
	}
	utils.Logger.Printf("Parsed response:\nRole: %s\nContent: %s", response.Role, response.Content)

	return response, nil
}

// ParseAction parses the LLM response into an action map
func (c *DeepseekClient) ParseAction(content string) (map[string]interface{}, error) {
	utils.Logger.Printf("Parsing action from content:\n%s", content)

	// Find the first { and last } to extract JSON
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start == -1 || end == -1 || end <= start {
		utils.Logger.Printf("No valid JSON found in content")
		return nil, fmt.Errorf("no valid JSON found in content: %s", content)
	}

	jsonStr := content[start : end+1]
	utils.Logger.Printf("Extracted JSON:\n%s", jsonStr)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		utils.Logger.Printf("Failed to parse JSON: %v", err)
		return nil, fmt.Errorf("failed to parse action: %w", err)
	}

	// Validate required fields
	if result["action"] == nil {
		utils.Logger.Printf("Missing 'action' field in response")
		return nil, fmt.Errorf("missing 'action' field in response")
	}
	if result["body"] == nil {
		utils.Logger.Printf("Missing 'body' field in response")
		return nil, fmt.Errorf("missing 'body' field in response")
	}

	utils.Logger.Printf("Successfully parsed action:\n%+v", result)
	return result, nil
}
