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

// ModelType represents different DeepSeek model types
type ModelType string

const (
	ModelChat ModelType = "deepseek-chat"
	ModelR1   ModelType = "deepseek-reasoner"
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

// CompleteWithModel sends a completion request to the Deepseek API using the specified model
func (c *DeepseekClient) CompleteWithModel(messages []models.Message, model ModelType) (*models.Message, error) {
	utils.Logger.Printf("Sending completion request to Deepseek API with %d messages using model %s", len(messages), model)

	// For the reasoner model, we need special message ordering
	if model == ModelR1 {
		var systemMsgs, userMsgs, assistantMsgs []models.Message
		for _, msg := range messages {
			switch msg.Role {
			case "system":
				systemMsgs = append(systemMsgs, msg)
			case "user":
				userMsgs = append(userMsgs, msg)
			case "assistant":
				assistantMsgs = append(assistantMsgs, msg)
			}
		}

		// Reconstruct message sequence:
		// 1. System messages first
		// 2. First non-system message must be user
		// 3. Then strictly alternating user/assistant messages
		// 4. Ensure last message is from user
		messages = systemMsgs

		// If we have no user messages, add a starter one
		if len(userMsgs) == 0 {
			messages = append(messages, models.Message{
				Role:    "user",
				Content: "Please help me discover the API schema.",
			})
		} else {
			// Add first user message
			messages = append(messages, userMsgs[0])
			userMsgs = userMsgs[1:]

			// Strictly alternate between assistant and user messages
			minLen := len(assistantMsgs)
			if len(userMsgs) < minLen {
				minLen = len(userMsgs)
			}

			for i := 0; i < minLen; i++ {
				messages = append(messages, assistantMsgs[i])
				messages = append(messages, userMsgs[i])
			}

			// If we have leftover messages, add at most one more of either type
			if len(assistantMsgs) > minLen {
				messages = append(messages, assistantMsgs[minLen])
				messages = append(messages, models.Message{
					Role:    "user",
					Content: "Please continue with the next step.",
				})
			} else if len(userMsgs) > minLen {
				// If we have a leftover user message, use it as the last message
				messages = append(messages, userMsgs[minLen])
			}
		}

		// Final check: ensure last message is from user
		if len(messages) > 0 && messages[len(messages)-1].Role != "user" {
			messages = append(messages, models.Message{
				Role:    "user",
				Content: "Please continue with the next step.",
			})
		}
	}

	reqBody := DeepseekRequest{
		Model:       string(model),
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      false,
	}

	// Adjust parameters for R1 model
	if model == ModelR1 {
		reqBody.Temperature = 0.3 // Lower temperature for more focused reasoning
		reqBody.MaxTokens = 2000  // Increased context for complex reasoning
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

// Complete sends a completion request to the Deepseek API using the default chat model
func (c *DeepseekClient) Complete(messages []models.Message) (*models.Message, error) {
	return c.CompleteWithModel(messages, ModelChat)
}

// CompleteWithR1 sends a completion request using the DeepSeek R1 model
func (c *DeepseekClient) CompleteWithR1(messages []models.Message) (*models.Message, error) {
	return c.CompleteWithModel(messages, ModelR1)
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
