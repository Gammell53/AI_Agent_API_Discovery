# AI Agent API Discovery

A Go-based API that discovers and infers the schema of JSON responses from other APIs using Deepseek LLM.

## Running the Servers

1. Start the test API server (provides endpoints to test against):
```bash
go run cmd/testserver/main.go
```

2. Start the main discovery server (requires Deepseek API key):
```bash
go run main.go -api-key="your-deepseek-api-key-here"
```

3. Run the test script to try discovering schemas:
```bash
go run cmd/test/main.go
```

## Test Endpoints Available

The test server provides several endpoints to test the discovery agent:

1. **Simple User Creation** (`POST /api/users`)
   - Required fields: email, password
   - Optional fields: name, age, etc.

2. **Complex User Creation** (`POST /api/users/complex`)
   - Required fields: email, password, profile.firstName, profile.lastName
   - Optional fields: name, age, profile.hobbies, etc.

3. **Product Creation** (`POST /api/products`)
   - Required fields: name, price, sku
   - Optional fields: inStock, categories

4. **Batch User Creation** (`POST /api/batch/users`)
   - Accepts an array of users
   - Each user requires email and password

## Example Discovery Request

```bash
curl -X POST http://localhost:8080/api/discover \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://localhost:8081/api/users",
    "method": "POST",
    "maxIterations": 10
  }'
```

## Response Format

The discovery API returns a schema describing the fields:

```json
{
  "fields": [
    {
      "name": "email",
      "type": "string",
      "required": true
    },
    {
      "name": "password",
      "type": "string",
      "required": true
    },
    {
      "name": "name",
      "type": "string",
      "required": false
    }
  ]
}
```