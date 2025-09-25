package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/state"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ExecuteSelectInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SELECT SQL query to execute (e.g. 'SELECT * FROM users LIMIT 5')"`
}

type ExecuteSelectOutput struct {
	Results string `json:"results" jsonschema_description:"JSON array of query results"`
}

func GetExecuteSelectTool(dbClient *client.DBClient, readOnly bool) *ToolDefinition[ExecuteSelectInput, ExecuteSelectOutput] {
	return NewToolDefinition[ExecuteSelectInput, ExecuteSelectOutput](
		"execute_select",
		"Execute a SELECT query on the database and return JSON results.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ExecuteSelectInput) (*mcp.CallToolResult, ExecuteSelectOutput, error) {
			return executeSelectHandler(ctx, req, input, dbClient, readOnly)
		},
	)
}

func executeSelectHandler(ctx context.Context, req *mcp.CallToolRequest, input ExecuteSelectInput, dbClient *client.DBClient, readOnly bool) (*mcp.CallToolResult, ExecuteSelectOutput, error) {
	queryLower := strings.ToLower(input.Query)
	if readOnly && !strings.HasPrefix(queryLower, "select") {
		return nil, ExecuteSelectOutput{}, fmt.Errorf("read-only mode: only SELECT queries allowed")
	}

	sessionID := "default"
	state := state.GetOrCreateSession(sessionID, dbClient)
	if state == nil || state.Conn == nil {
		return nil, ExecuteSelectOutput{}, fmt.Errorf("no active DB connection in session")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := state.Conn.QueryContext(ctx, input.Query)
	if err != nil {
		return nil, ExecuteSelectOutput{}, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, ExecuteSelectOutput{}, fmt.Errorf("columns error: %v", err)
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		vals := make([]interface{}, len(columns))
		valPtrs := make([]interface{}, len(columns))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, ExecuteSelectOutput{}, fmt.Errorf("scan error: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := vals[i]

			switch v := val.(type) {
			case []byte:

				row[col] = string(v)
			case nil:
				row[col] = nil
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return nil, ExecuteSelectOutput{}, fmt.Errorf("JSON error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, ExecuteSelectOutput{Results: string(jsonBytes)}, nil
}
