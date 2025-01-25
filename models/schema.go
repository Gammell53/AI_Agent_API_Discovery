package models

// DiscoverRequest represents the input request to discover an API's schema
type DiscoverRequest struct {
	Method        string                 `json:"method"` // e.g., "POST"
	URL           string                 `json:"url" binding:"required,url"`
	Headers       map[string]string      `json:"headers"`       // e.g., {"Authorization": "Bearer ..."}
	InitialBody   map[string]interface{} `json:"initialBody"`   // Optional: initial guess at fields
	MaxIterations int                    `json:"maxIterations"` // Safety limit for iterations
}

// DiscoveredSchema represents the final output of field discovery
type DiscoveredSchema struct {
	Fields             []FieldInfo            `json:"fields"`
	MinimalRequestBody map[string]interface{} `json:"minimalRequestBody"`
}

// FieldInfo represents information about a discovered field
type FieldInfo struct {
	Name           string       `json:"name"`
	Type           string       `json:"type"`              // string, integer, boolean, object, array, etc.
	Format         string       `json:"format,omitempty"`  // email, date, uuid, etc.
	Pattern        string       `json:"pattern,omitempty"` // regex pattern if applicable
	MinLength      *int         `json:"minLength,omitempty"`
	MaxLength      *int         `json:"maxLength,omitempty"`
	Minimum        *float64     `json:"minimum,omitempty"`
	Maximum        *float64     `json:"maximum,omitempty"`
	Enum           []string     `json:"enum,omitempty"` // possible values if field is enumerated
	Children       []FieldInfo  `json:"children,omitempty"`
	SampleValue    interface{}  `json:"sampleValue,omitempty"`
	Description    string       `json:"description,omitempty"`
	IsInMinimalSet bool         `json:"isInMinimalSet"`        // whether this field is part of minimal set
	TestResults    *TestResults `json:"testResults,omitempty"` // results of field testing
}

// TestResults represents the results of field testing
type TestResults struct {
	SuccessfulTests int           `json:"successfulTests"`
	FailedTests     int           `json:"failedTests"`
	TestedValues    []interface{} `json:"testedValues,omitempty"`
	FailedValues    []interface{} `json:"failedValues,omitempty"`
	ErrorMessages   []string      `json:"errorMessages,omitempty"`
}

// FieldTestStatus tracks the testing status of each field
type FieldTestStatus struct {
	IsDiscovered        bool
	IsTypeVerified      bool
	IsRequired          bool
	IsOptionalityTested bool
	TestedValues        []interface{} // Track values we've tried
	FailedValues        []interface{} // Track values that failed
	ValidationErrors    []string      // Collection of validation errors received
	SuccessfulTests     int           // Count of successful tests
	FailedTests         int           // Count of failed tests
}

// ValidationResult represents the result of testing a field value
type ValidationResult struct {
	IsValid      bool
	ErrorMessage string
	Constraints  map[string]interface{} // Discovered constraints from validation
}

// HTTPResponse represents a response from the target API
type HTTPResponse struct {
	StatusCode   int                 `json:"statusCode"`
	ResponseBody []byte              `json:"responseBody"`
	Headers      map[string][]string `json:"headers"`
}

// Message represents a message in the LLM conversation
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // the actual message content
}
