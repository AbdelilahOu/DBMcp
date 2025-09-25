package mcpdb

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecuteSelectInput: Tool input (auto-generates JSON schema via tags)
type ExecuteSelectInput struct {
	Query string `json:"query" jsonschema:"required=true,description=SELECT SQL query to execute (e.g., 'SELECT * FROM users LIMIT 5')"`
}

// ExecuteSelectOutput: Tool output
type ExecuteSelectOutput struct {
	Results string `json:"results" jsonschema:"description=JSON array of query results"`
}

// InputSchema: For MCP tool registration (manual if needed; SDK auto-generates from struct)
func ExecuteSelectInputSchema() *mcp.JSONSchema {
	// SDK generates from struct tags; this is a stub
	return &mcp.JSONSchema{
		Type: "object",
		Properties: map[string]*mcp.JSONSchema{
			"query": {Type: "string", Description: "SELECT SQL query"},
		},
		Required: []string{"query"},
	}
}
