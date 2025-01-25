package agent

import (
	"ai-agent-api-discovery/llm"
	"ai-agent-api-discovery/models"
	"ai-agent-api-discovery/utils"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// FieldTestStatus tracks the testing status of each field
type FieldTestStatus struct {
	IsDiscovered        bool // Field has been found
	IsTypeVerified      bool // Type has been verified
	IsRequired          bool // Whether field is required
	IsOptionalityTested bool // We've tested if it's optional
}

// DeepseekAgent orchestrates the API discovery process using LLM
type DeepseekAgent struct {
	request      models.DiscoverRequest
	conversation []models.Message
	knownFields  map[string]*models.FieldInfo
	fieldStatus  map[string]*FieldTestStatus
	currentBody  map[string]interface{}
	iterations   int
	llmClient    *llm.DeepseekClient
}

// NewDeepseekAgent creates a new instance of DeepseekAgent
func NewDeepseekAgent(req models.DiscoverRequest) (*DeepseekAgent, error) {
	client, err := llm.NewDeepseekClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Deepseek client: %w", err)
	}

	return &DeepseekAgent{
		request:      req,
		conversation: []models.Message{},
		knownFields:  make(map[string]*models.FieldInfo),
		fieldStatus:  make(map[string]*FieldTestStatus),
		currentBody:  req.InitialBody,
		iterations:   0,
		llmClient:    client,
	}, nil
}

// RunDiscovery executes the main discovery loop
func (a *DeepseekAgent) RunDiscovery() (*models.DiscoveredSchema, error) {
	utils.Logger.Printf("Starting discovery for %s %s", a.request.Method, a.request.URL)

	// Initialize with system prompt
	a.addSystemMessage(`You are an AI agent that discovers API schemas through intelligent interaction.
Your goal is to understand the structure and requirements of any API endpoint through systematic testing.

IMPORTANT: Always respond with a valid JSON object in this format:
{
    "action": "modify_fields",
    "body": {
        "field1": "value1",
        "field2": "value2"
    },
    "explanation": "Reasoning behind these changes"
}

Key Strategies:

1. Progressive Discovery:
   - Start with minimal, common fields
   - Add fields based on error messages
   - Use semantic naming to guess related fields
   - Consider API context (e.g., /users endpoint likely needs email/username)

2. Smart Value Selection:
   - Use contextually appropriate test values
   - Match values to field names (e.g., "email" → valid email format)
   - Consider common validation patterns
   - Test edge cases when appropriate

3. Array/Batch Handling:
   - For batch endpoints, try both single and multiple items
   - Test array wrapper keys: "items", "data", "records", etc.
   - Ensure consistent field structure across array items

4. Error Analysis:
   - Extract field names from error messages
   - Identify validation requirements
   - Look for type hints in errors
   - Parse both structured and unstructured errors

5. Type Detection:
   - Infer types from successful responses
   - Consider field name conventions
   - Test multiple value formats
   - Look for format-specific patterns

6. Field Requirements:
   - Mark fields as required only when explicitly indicated
   - Consider API context for likely requirements
   - Test removal of fields to verify requirements
   - Watch for dependent field relationships

When you want to indicate completion:
{
    "action": "complete",
    "body": {},
    "explanation": "Schema discovery is complete"
}

Complete when:
1. We've made at least one successful request
2. We've identified all required fields
3. We understand the basic structure
4. We can handle any array/batch requirements

Remember:
- Different APIs have different patterns
- Error messages vary in format and detail
- Some fields may be server-generated
- Requirements might depend on API context
- Security fields need special handling`)

	// Add initial user message
	initialMsg := fmt.Sprintf("We are calling %s %s. "+
		"We start with %s body. Please propose next steps.",
		a.request.Method, a.request.URL,
		func() string {
			if len(a.currentBody) == 0 {
				return "an empty"
			}
			return "the provided"
		}())
	utils.Logger.Printf("Initial message: %s", initialMsg)
	a.addUserMessage(initialMsg)

	for a.iterations < a.request.MaxIterations {
		utils.Logger.Printf("\n=== Iteration %d/%d ===", a.iterations+1, a.request.MaxIterations)
		a.iterations++

		// Get next action from LLM
		utils.Logger.Printf("Getting next action from LLM...")
		nextAction, err := a.askLLMForNextAction()
		if err != nil {
			utils.Logger.Printf("Error getting next action: %v", err)
			return nil, fmt.Errorf("failed to get next action: %w", err)
		}
		actionBytes, _ := json.MarshalIndent(nextAction, "", "  ")
		utils.Logger.Printf("LLM suggested action:\n%s", string(actionBytes))

		// Check if we're done
		if action, ok := nextAction["action"].(string); ok && action == "complete" {
			if !a.isDiscoveryComplete() {
				incompleteMsg := "Cannot complete yet. Some fields still need testing. " + a.getIncompleteFieldsMessage()
				utils.Logger.Printf("Completion rejected: %s", incompleteMsg)
				a.addSystemMessage(incompleteMsg)
				continue
			}
			utils.Logger.Printf("Discovery complete! Building final schema...")
			return a.buildSchema(), nil
		}

		// Execute the HTTP request
		utils.Logger.Printf("Executing HTTP request with body: %+v", nextAction["body"])
		response, err := a.executeHTTP(nextAction)
		if err != nil {
			errMsg := fmt.Sprintf("HTTP call failed: %v", err)
			utils.Logger.Print(errMsg)
			a.addSystemMessage(errMsg)
			continue
		}

		// Handle the response
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			utils.Logger.Printf("Request succeeded with status %d", response.StatusCode)
			// Try to parse the response body to understand the schema better
			var respJSON map[string]interface{}
			if err := json.Unmarshal(response.ResponseBody, &respJSON); err == nil {
				utils.Logger.Printf("Response body: %+v", respJSON)
				a.updateKnownFields(respJSON)
			}

			schema, done := a.handleSuccess(response)
			if done {
				utils.Logger.Printf("Discovery completed successfully!")
				return schema, nil
			}
		} else {
			utils.Logger.Printf("Request failed with status %d", response.StatusCode)
			a.handleErrorResponse(response)
		}

		// Log current state
		utils.Logger.Printf("\nCurrent known fields:")
		for fieldName, info := range a.knownFields {
			status := a.fieldStatus[fieldName]
			utils.Logger.Printf("- %s: type=%s, required=%v, tested=%v",
				fieldName,
				info.Type,
				status.IsRequired,
				status.IsOptionalityTested)
		}

		// Add status update to conversation
		statusMsg := a.getFieldStatusMessage()
		utils.Logger.Printf("\nStatus update: %s", statusMsg)
		a.addSystemMessage(statusMsg)
	}

	utils.Logger.Printf("Max iterations (%d) reached without completing discovery", a.request.MaxIterations)
	return nil, fmt.Errorf("max iterations (%d) reached without finalizing schema", a.request.MaxIterations)
}

// isDiscoveryComplete checks if we've completed testing all fields
func (a *DeepseekAgent) isDiscoveryComplete() bool {
	// We only care about discovering required fields now
	return len(a.fieldStatus) > 0
}

// getIncompleteFieldsMessage generates a message about incomplete field testing
func (a *DeepseekAgent) getIncompleteFieldsMessage() string {
	var incomplete []string
	for fieldName, status := range a.fieldStatus {
		if !status.IsDiscovered {
			incomplete = append(incomplete, fmt.Sprintf("%s (not discovered)", fieldName))
		}
	}
	return fmt.Sprintf("Fields still being discovered: %v", incomplete)
}

// getFieldStatusMessage generates a message about current field testing status
func (a *DeepseekAgent) getFieldStatusMessage() string {
	var status []string
	for fieldName, info := range a.knownFields {
		fieldStatus := a.fieldStatus[fieldName]
		if fieldStatus == nil {
			continue
		}
		status = append(status, fmt.Sprintf(
			"%s (type: %s, required: %v)",
			fieldName,
			info.Type,
			fieldStatus.IsRequired,
		))
	}
	return fmt.Sprintf("Current field status:\n%v", status)
}

// askLLMForNextAction gets the next action from the LLM
func (a *DeepseekAgent) askLLMForNextAction() (map[string]interface{}, error) {
	// Get completion from Deepseek using R1 model for better reasoning
	response, err := a.llmClient.CompleteWithR1(a.conversation)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM completion: %w", err)
	}

	// Add the response to our conversation
	a.addAssistantMessage(response.Content)

	// Parse the action from the response
	action, err := a.llmClient.ParseAction(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM action: %w", err)
	}

	return action, nil
}

// executeHTTP performs the HTTP request with the current state
func (a *DeepseekAgent) executeHTTP(action map[string]interface{}) (*models.HTTPResponse, error) {
	body, ok := action["body"].(map[string]interface{})
	if !ok {
		body = a.currentBody
	}

	var requestBody interface{} = body

	// Check if this is a batch/array endpoint
	if strings.Contains(strings.ToLower(a.request.URL), "batch") ||
		strings.Contains(strings.ToLower(a.request.URL), "bulk") {
		// Try to construct an array request
		if singleItem, ok := body["item"].(map[string]interface{}); ok {
			// If "item" is provided, use it as template
			requestBody = []interface{}{singleItem}
		} else {
			// Create an array with the current body as a template
			requestBody = []interface{}{body}
		}
		utils.Logger.Printf("Constructed array request body: %+v", requestBody)
	} else {
		// For non-array endpoints, try both wrapped and direct array if we see array indicators
		if len(body) == 1 {
			for key, v := range body {
				if arr, isArray := v.([]interface{}); isArray {
					utils.Logger.Printf("Detected array payload with key '%s', trying both formats", key)
					// Try direct array first
					resp, err := utils.DoRequest(a.request.Method, a.request.URL, a.request.Headers, arr)
					if err == nil && resp.StatusCode < 400 {
						return resp, nil
					}
					// Fall back to wrapped object
					utils.Logger.Printf("Direct array failed, using wrapped format with key '%s'", key)
					requestBody = body
				}
			}
		}
	}

	// Update current body
	if bodyMap, isMap := requestBody.(map[string]interface{}); isMap {
		if a.currentBody == nil {
			a.currentBody = make(map[string]interface{})
		}
		for k, v := range bodyMap {
			a.currentBody[k] = v
		}
	}

	return utils.DoRequest(
		a.request.Method,
		a.request.URL,
		a.request.Headers,
		requestBody,
	)
}

// handleSuccess processes a successful API response
func (a *DeepseekAgent) handleSuccess(resp *models.HTTPResponse) (*models.DiscoveredSchema, bool) {
	// Parse response body to understand the schema better
	var respJSON map[string]interface{}
	if err := json.Unmarshal(resp.ResponseBody, &respJSON); err == nil {
		utils.Logger.Printf("Response body: %+v", respJSON)
	}

	// Only mark fields as required if they were explicitly required in error messages
	// or if they were part of the minimum successful request
	for fieldName := range a.currentBody {
		if status, exists := a.fieldStatus[fieldName]; exists {
			status.IsDiscovered = true
			status.IsTypeVerified = true
			// Only keep required=true if it was marked as required by error messages
		}
	}

	// Identify server-generated fields
	serverGeneratedFields := []string{"id", "isActive", "createdAt", "updatedAt"}
	for _, field := range serverGeneratedFields {
		if info, exists := a.knownFields[field]; exists {
			info.Required = false
			if status, exists := a.fieldStatus[field]; exists {
				status.IsRequired = false
			}
		}
	}

	// Log success
	utils.Logger.Printf("Successfully discovered all required fields!")
	utils.Logger.Printf("Final request body that succeeded: %+v", a.currentBody)

	// Return schema immediately - we've found all required fields
	return a.buildSchema(), true
}

// handleErrorResponse processes an error response from the API
func (a *DeepseekAgent) handleErrorResponse(resp *models.HTTPResponse) {
	errorText := string(resp.ResponseBody)

	// Try to parse structured error response
	var errorResp struct {
		Error            string            `json:"error"`
		Errors           []string          `json:"errors"`
		ValidationErrors map[string]string `json:"validation_errors"`
	}

	if err := json.Unmarshal(resp.ResponseBody, &errorResp); err == nil {
		// Handle structured error response
		if errorResp.Error != "" {
			a.analyzeErrorMessage(errorResp.Error)
		}
		if len(errorResp.Errors) > 0 {
			for _, err := range errorResp.Errors {
				a.analyzeErrorMessage(err)
			}
		}
		if len(errorResp.ValidationErrors) > 0 {
			for field, err := range errorResp.ValidationErrors {
				a.updateFieldFromError(field, err)
			}
		}
	} else {
		// Handle plain text error
		a.analyzeErrorMessage(errorText)
	}

	a.addSystemMessage(fmt.Sprintf("Got error response (status %d): %s\nAnalyzed error message for field requirements.",
		resp.StatusCode, errorText))
}

// analyzeErrorMessage tries to extract field information from error messages
func (a *DeepseekAgent) analyzeErrorMessage(errMsg string) {
	// Common patterns for required field errors
	patterns := []struct {
		regex   *regexp.Regexp
		handler func(matches []string)
	}{
		{
			regex: regexp.MustCompile(`(?i)(field|parameter) ['"]?(\w+)['"]? is required`),
			handler: func(matches []string) {
				field := matches[2]
				a.markFieldRequired(field)
			},
		},
		{
			regex: regexp.MustCompile(`(?i)missing (?:required )?(?:field|parameter) ['"]?(\w+)['"]?`),
			handler: func(matches []string) {
				field := matches[1]
				a.markFieldRequired(field)
			},
		},
		{
			regex: regexp.MustCompile(`(?i)invalid (?:value|type) for ['"]?(\w+)['"]?`),
			handler: func(matches []string) {
				field := matches[1]
				a.markFieldTypeInvalid(field)
			},
		},
	}

	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(errMsg); matches != nil {
			pattern.handler(matches)
		}
	}
}

// updateFieldFromError updates field information based on validation error
func (a *DeepseekAgent) updateFieldFromError(field, errMsg string) {
	if strings.Contains(strings.ToLower(errMsg), "required") {
		a.markFieldRequired(field)
	}

	// Try to infer type from error message
	typePatterns := map[string]string{
		"must be a number":  "number",
		"must be a string":  "string",
		"must be a boolean": "boolean",
		"must be an array":  "array",
		"must be an object": "object",
		"invalid email":     "email",
		"invalid date":      "date",
	}

	for pattern, fieldType := range typePatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			if info, exists := a.knownFields[field]; exists {
				info.Type = fieldType
			} else {
				a.knownFields[field] = &models.FieldInfo{
					Name:     field,
					Type:     fieldType,
					Required: true,
				}
			}
			break
		}
	}
}

// markFieldRequired marks a field as required
func (a *DeepseekAgent) markFieldRequired(field string) {
	if info, exists := a.knownFields[field]; exists {
		info.Required = true
	} else {
		a.knownFields[field] = &models.FieldInfo{
			Name:     field,
			Required: true,
		}
	}

	if status, exists := a.fieldStatus[field]; exists {
		status.IsRequired = true
	} else {
		a.fieldStatus[field] = &FieldTestStatus{
			IsRequired:   true,
			IsDiscovered: true,
		}
	}
}

// markFieldTypeInvalid marks that we need to try a different type for the field
func (a *DeepseekAgent) markFieldTypeInvalid(field string) {
	if status, exists := a.fieldStatus[field]; exists {
		status.IsTypeVerified = false
	}
}

// updateKnownFields updates the known fields based on the response
func (a *DeepseekAgent) updateKnownFields(respJSON map[string]interface{}) {
	for key, value := range respJSON {
		// Skip server-generated fields in response
		if isServerGeneratedField(key) {
			continue
		}

		// Update or create field info
		if _, exists := a.knownFields[key]; !exists {
			a.knownFields[key] = &models.FieldInfo{
				Name:        key,
				Type:        inferType(value),
				Required:    false, // Default to false unless explicitly required
				SampleValue: value,
			}
		}

		// Update or create field status
		if _, exists := a.fieldStatus[key]; !exists {
			a.fieldStatus[key] = &FieldTestStatus{
				IsDiscovered:   true,
				IsTypeVerified: true,
				IsRequired:     false, // Default to false unless explicitly required
			}
		}
	}
}

// isServerGeneratedField checks if a field is typically server-generated using pattern matching
func isServerGeneratedField(field string) bool {
	// Common patterns for server-generated fields
	patterns := []struct {
		pattern string
		match   func(string) bool
	}{
		// ID patterns
		{pattern: "id", match: func(f string) bool {
			return strings.EqualFold(f, "id") || strings.HasSuffix(strings.ToLower(f), "_id") || strings.HasSuffix(strings.ToLower(f), "Id")
		}},
		// Timestamp patterns
		{pattern: "timestamp", match: func(f string) bool {
			f = strings.ToLower(f)
			return strings.Contains(f, "timestamp") || strings.Contains(f, "date") ||
				strings.Contains(f, "created") || strings.Contains(f, "updated") ||
				strings.Contains(f, "modified") || strings.Contains(f, "deleted")
		}},
		// Status flags often set by server
		{pattern: "status", match: func(f string) bool {
			f = strings.ToLower(f)
			return strings.Contains(f, "status") || strings.Contains(f, "state") ||
				strings.Contains(f, "active") || strings.Contains(f, "enabled") ||
				strings.Contains(f, "deleted") || strings.Contains(f, "archived")
		}},
		// System fields
		{pattern: "system", match: func(f string) bool {
			f = strings.ToLower(f)
			return strings.HasPrefix(f, "sys_") || strings.HasPrefix(f, "_") ||
				strings.Contains(f, "hash") || strings.Contains(f, "checksum")
		}},
	}

	field = strings.TrimSpace(field)
	for _, p := range patterns {
		if p.match(field) {
			utils.Logger.Printf("Field '%s' matches server-generated pattern '%s'", field, p.pattern)
			return true
		}
	}
	return false
}

// Helper functions for type detection
func isEmailPattern(s string) bool {
	return strings.Contains(s, "@") && strings.Contains(s, ".")
}

func isDatePattern(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return true
	}
	_, err = time.Parse("2006-01-02", s)
	return err == nil
}

func isUUIDPattern(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}

func isURLPattern(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), "http") || strings.HasPrefix(strings.ToLower(s), "https")
}

func isPhonePattern(s string) bool {
	phonePattern := regexp.MustCompile(`^\+?[\d\s-()]{10,}$`)
	return phonePattern.MatchString(s)
}

func isColorPattern(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "rgb") || strings.HasPrefix(s, "hsl")
}

func isIPPattern(s string) bool {
	ipPattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	return ipPattern.MatchString(s)
}

func isTimestampRange(n float64) bool {
	return n > 1000000000 && n < 2000000000
}

func isYearRange(n float64) bool {
	return n >= 1900 && n <= 2100
}

func isCurrencyRange(n float64) bool {
	// Typical currency values (0.01 to 1,000,000)
	return n > 0 && n <= 1000000
}

func isPercentageRange(n float64) bool {
	return n >= 0 && n <= 100
}

// inferType determines the type of a value with improved pattern detection
func inferType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case string:
		// Enhanced string pattern detection
		if isEmailPattern(v) {
			return "email"
		}
		if isDatePattern(v) {
			return "date"
		}
		if isUUIDPattern(v) {
			return "uuid"
		}
		if isURLPattern(v) {
			return "url"
		}
		if isPhonePattern(v) {
			return "phone"
		}
		if isColorPattern(v) {
			return "color"
		}
		if isIPPattern(v) {
			return "ip"
		}
		return "string"
	case float64:
		if v == float64(int64(v)) {
			if isTimestampRange(v) {
				return "timestamp"
			}
			if isYearRange(v) {
				return "year"
			}
			return "integer"
		}
		if isCurrencyRange(v) {
			return "currency"
		}
		if isPercentageRange(v) {
			return "percentage"
		}
		return "float"
	case bool:
		return "boolean"
	case []interface{}:
		if len(v) > 0 {
			elemType := inferType(v[0])
			return fmt.Sprintf("array<%s>", elemType)
		}
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// buildSchema creates the final schema from discovered fields
func (a *DeepseekAgent) buildSchema() *models.DiscoveredSchema {
	var fields []models.FieldInfo
	for _, info := range a.knownFields {
		fields = append(fields, *info)
	}
	return &models.DiscoveredSchema{Fields: fields}
}

// addSystemMessage adds a system message to the conversation
func (a *DeepseekAgent) addSystemMessage(content string) {
	a.conversation = append(a.conversation, models.Message{
		Role:    "system",
		Content: content,
	})
}

// addUserMessage adds a user message to the conversation
func (a *DeepseekAgent) addUserMessage(content string) {
	a.conversation = append(a.conversation, models.Message{
		Role:    "user",
		Content: content,
	})
}

// addAssistantMessage adds an assistant message to the conversation
func (a *DeepseekAgent) addAssistantMessage(content string) {
	a.conversation = append(a.conversation, models.Message{
		Role:    "assistant",
		Content: content,
	})
}
