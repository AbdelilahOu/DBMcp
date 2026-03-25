package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ExecuteQueryInput struct {
	Query string `json:"q" jsonschema:"required" jsonschema_description:"SQL query to execute (INSERT, UPDATE, DELETE, etc.)"`
}

type ExecuteQueryOutput struct {
	RowsAffected int64  `json:"rows,omitempty" jsonschema_description:"Number of rows affected by the query"`
	Message      string `json:"msg" jsonschema_description:"Success message"`
}

func GetExecuteQueryTool() *ToolDefinition[ExecuteQueryInput, ExecuteQueryOutput] {
	return NewToolDefinition[ExecuteQueryInput, ExecuteQueryOutput](
		"execute_query",
		"Execute INSERT, UPDATE, DELETE, CREATE, ALTER, DROP. Changes DB state. DO NOT use for reading (select_query) or metadata (describe_table, analyze_table, list_tables).",
		func(ctx context.Context, req *mcp.CallToolRequest, input ExecuteQueryInput) (*mcp.CallToolResult, ExecuteQueryOutput, error) {
			sessionState, err := state.GetActiveSession("default")
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

			logger.LogDatabaseOperation(operation, input.Query, rowsAffected, nil)

			message := fmt.Sprintf("%s OK", operation)
			if rowsAffected > 0 {
				message = fmt.Sprintf("%s OK (%d rows)", operation, rowsAffected)
			}

			output := ExecuteQueryOutput{
				RowsAffected: rowsAffected,
				Message:      message,
			}

			return textResult(output.Message), output, nil
		},
	)
}
