package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ExecuteQueryInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SQL query to execute (INSERT, UPDATE, DELETE, etc.)"`
}

type ExecuteQueryOutput struct {
	RowsAffected int64  `json:"rows_affected" jsonschema_description:"Number of rows affected by the query"`
	Message      string `json:"message" jsonschema_description:"Success message"`
}

func GetExecuteQueryTool() *ToolDefinition[ExecuteQueryInput, ExecuteQueryOutput] {
	return NewToolDefinition[ExecuteQueryInput, ExecuteQueryOutput](
		"execute_query",
		"Execute any SQL query (INSERT, UPDATE, DELETE, etc.) with proper permissions.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ExecuteQueryInput) (*mcp.CallToolResult, ExecuteQueryOutput, error) {
			return executeQueryHandler(ctx, req, input)
		},
	)
}

func executeQueryHandler(ctx context.Context, req *mcp.CallToolRequest, input ExecuteQueryInput) (*mcp.CallToolResult, ExecuteQueryOutput, error) {
	sessionState, err := getActiveSession("default")
	if err != nil {
		return nil, ExecuteQueryOutput{}, err
	}

	queryLower := strings.ToLower(strings.TrimSpace(input.Query))
	dangerousOperations := []string{"drop database", "drop schema", "truncate"}
	for _, dangerous := range dangerousOperations {
		if strings.Contains(queryLower, dangerous) {
			return nil, ExecuteQueryOutput{}, fmt.Errorf("dangerous operation detected: %s", dangerous)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := sessionState.Conn.ExecContext(ctx, input.Query)

	if err != nil {
		logger.LogDatabaseOperation("EXECUTE", input.Query, 0, err)
		return nil, ExecuteQueryOutput{}, fmt.Errorf("query execution error: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}

	var operation string
	switch {
	case strings.HasPrefix(queryLower, "insert"):
		operation = "INSERT"
	case strings.HasPrefix(queryLower, "update"):
		operation = "UPDATE"
	case strings.HasPrefix(queryLower, "delete"):
		operation = "DELETE"
	case strings.HasPrefix(queryLower, "create"):
		operation = "CREATE"
	case strings.HasPrefix(queryLower, "alter"):
		operation = "ALTER"
	case strings.HasPrefix(queryLower, "drop"):
		operation = "DROP"
	default:
		operation = "QUERY"
	}

	// Log successful database operation
	logger.LogDatabaseOperation(operation, input.Query, rowsAffected, nil)

	message := fmt.Sprintf("%s operation completed successfully", operation)
	if rowsAffected > 0 {
		message = fmt.Sprintf("%s operation completed successfully (%d rows affected)", operation, rowsAffected)
	}

	output := ExecuteQueryOutput{
		RowsAffected: rowsAffected,
		Message:      message,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, ExecuteQueryOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}
