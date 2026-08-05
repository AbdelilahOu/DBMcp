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

type ShowQueryInput struct {
	Query string `json:"q" jsonschema:"required" jsonschema_description:"SHOW SQL query to execute (e.g., SHOW TABLES, SHOW DATABASES, SHOW COLUMNS, etc.)"`
}

type ShowQueryOutput struct {
	Data    []map[string]interface{} `json:"data" jsonschema_description:"Query results"`
	Message string                   `json:"msg" jsonschema_description:"Success message"`
}

func GetShowQueryTool() *ToolDefinition[ShowQueryInput, ShowQueryOutput] {
	return NewToolDefinition[ShowQueryInput, ShowQueryOutput](
		"show_query",
		"Execute SHOW commands. MySQL: SHOW TABLES/DATABASES/COLUMNS. PostgreSQL: SHOW server_version/search_path. For DB-agnostic ops prefer list_tables/describe_table/get_db_info.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ShowQueryInput) (*mcp.CallToolResult, ShowQueryOutput, error) {
			sessionState, err := state.GetActiveSession()
			if err != nil {
				return nil, ShowQueryOutput{}, err
			}

			if !sessionState.Driver.SupportsShowCommands() {
				return nil, ShowQueryOutput{}, fmt.Errorf("this database does not support SHOW commands")
			}

			queryLower := strings.ToLower(strings.TrimSpace(input.Query))
			if !strings.HasPrefix(queryLower, "show") {
				return nil, ShowQueryOutput{}, fmt.Errorf("only SHOW queries are allowed")
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			rows, err := sessionState.Conn.QueryContext(ctx, input.Query)

			if err != nil {
				logger.LogDatabaseOperation("SHOW", input.Query, 0, err)
				return nil, ShowQueryOutput{}, fmt.Errorf("query execution error: %v", err)
			}
			defer rows.Close()

			columns, err := rows.Columns()
			if err != nil {
				return nil, ShowQueryOutput{}, fmt.Errorf("error getting columns: %v", err)
			}

			results := make([]map[string]interface{}, 0)
			for rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range columns {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					return nil, ShowQueryOutput{}, fmt.Errorf("error scanning row: %v", err)
				}

				row := make(map[string]interface{})
				for i, col := range columns {
					val := values[i]
					if b, ok := val.([]byte); ok {
						row[col] = string(b)
					} else {
						row[col] = val
					}
				}
				results = append(results, row)
			}

			if err = rows.Err(); err != nil {
				return nil, ShowQueryOutput{}, fmt.Errorf("error iterating rows: %v", err)
			}

			logger.LogDatabaseOperation("SHOW", input.Query, int64(len(results)), nil)

			message := fmt.Sprintf("SHOW query completed successfully (%d rows returned)", len(results))

			output := ShowQueryOutput{
				Data:    results,
				Message: message,
			}

			return textResult(output.Message), output, nil
		},
	)
}
