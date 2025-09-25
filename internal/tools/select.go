package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func executeSelectHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ExecuteSelectInput, dbClient *client.DBClient, readOnly bool) (*mcp.CallToolResult, mcpdb.ExecuteSelectOutput, error) {
	// Validate read-only
	queryLower := strings.ToLower(input.Query)
	if readOnly && !strings.HasPrefix(queryLower, "select") {
		return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("read-only mode: only SELECT queries allowed")
	}

	// Get/create session state (key use case: reuse conn across multi-turn queries)
	// Use a default session for simplicity
	sessionID := "default"
	state := state.GetOrCreateSession(sessionID, dbClient)
	if state == nil || state.Conn == nil {
		return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("no active DB connection in session")
	}

	// Timeout context (5s for safety)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Exec query
	rows, err := state.Conn.QueryContext(ctx, input.Query)
	if err != nil {
		return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	// Scan results to JSON (handles dynamic columns)
	columns, err := rows.Columns()
	if err != nil {
		return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("columns error: %v", err)
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		// Use interface{} to handle different types dynamically
		vals := make([]interface{}, len(columns))
		valPtrs := make([]interface{}, len(columns))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("scan error: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := vals[i]
			// Handle different SQL types
			switch v := val.(type) {
			case []byte:
				// Convert byte arrays to strings (common for text fields)
				row[col] = string(v)
			case nil:
				row[col] = nil
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	// Marshal output
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return nil, mcpdb.ExecuteSelectOutput{}, fmt.Errorf("JSON error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, mcpdb.ExecuteSelectOutput{Results: string(jsonBytes)}, nil
}

// Helper: contains for toolsets
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
