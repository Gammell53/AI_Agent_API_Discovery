# API Discovery Agent Design

## Core Objective
To discover and validate required fields for API endpoints quickly and efficiently.

## Discovery Strategy

### 1. Required Fields First Approach
- Start with empty request body
- Analyze error messages to identify required fields
- Add fields one by one based on error messages
- Stop and return schema once a successful request is made

### 2. Field Discovery Process
1. Initial Request:
   - Send empty request to get initial error message
   - Parse error message for required field information

2. Iterative Discovery:
   - Add missing required fields based on error messages
   - Use smart field type detection from error messages
   - Track discovered required fields

3. Success Criteria:
   - A successful API response (2xx) indicates all required fields are present
   - Immediately return the discovered schema with required fields
   - No need to test for optional fields or field removal

### 3. Error Message Analysis
- Parse structured error responses (JSON)
- Use regex patterns to extract field names
- Detect field types from error messages
- Handle both single and nested field requirements

### 4. Schema Output
```json
{
    "fields": [
        {
            "name": "string",
            "type": "string",
            "required": true,
            "format": "email|date|etc",  // if detected
            "description": "Field purpose based on name"
        }
    ]
}
```

## Implementation Details

### 1. Core Components
- DeepseekAgent: Orchestrates the discovery process
- Error Analyzer: Extracts field information from errors
- Type Detector: Determines field types
- Schema Builder: Constructs the final output

### 2. Error Handling
- Parse multiple error formats
- Handle rate limiting
- Manage timeouts
- Track failed attempts

### 3. Performance Considerations
- Stop immediately on success
- No optional field testing
- Minimize API calls
- Smart error message caching

## Success Metrics
1. Speed: Minimize number of API calls needed
2. Accuracy: Correctly identify all required fields
3. Reliability: Consistent results across different API styles
4. Efficiency: Stop as soon as successful request is made