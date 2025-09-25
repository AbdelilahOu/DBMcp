package mcpdb

// ExecuteSelectInput: Tool input (auto-generates JSON schema via tags)
type ExecuteSelectInput struct {
	Query string `json:"query" jsonschema:"required=true,description=SELECT SQL query to execute (e.g., 'SELECT * FROM users LIMIT 5')"`
}

// ExecuteSelectOutput: Tool output
type ExecuteSelectOutput struct {
	Results string `json:"results" jsonschema:"description=JSON array of query results"`
}

// Note: MCP SDK auto-generates schemas from struct tags
