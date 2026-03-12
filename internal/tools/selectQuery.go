package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SelectQueryInput struct {
	Query string `json:"q" jsonschema:"required" jsonschema_description:"SELECT SQL query to execute"`
}

type SelectQueryOutput struct {
	Data    []map[string]interface{} `json:"data" jsonschema_description:"Query results"`
	Message string                   `json:"msg" jsonschema_description:"Success message"`
}

func GetSelectQueryTool() *ToolDefinition[SelectQueryInput, SelectQueryOutput] {
	return NewToolDefinition[SelectQueryInput, SelectQueryOutput](
		"select_query",
		"Execute SELECT queries for actual data rows. Supports WHERE, joins. DO NOT use for row counts (analyze_table) or metadata (describe_table). Data content only.",
		func(ctx context.Context, req *mcp.CallToolRequest, input SelectQueryInput) (*mcp.CallToolResult, SelectQueryOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, SelectQueryOutput{}, err
			}

			queryLower := strings.ToLower(strings.TrimSpace(input.Query))
			if !strings.HasPrefix(queryLower, "select") || !strings.HasPrefix(queryLower, "with") {
				return nil, SelectQueryOutput{}, fmt.Errorf("only SELECT queries are allowed")
			}

			// Add default LIMIT if not specified
			query := input.Query
			if !strings.Contains(queryLower, "limit") {
				query = strings.TrimSpace(query)
				if strings.HasSuffix(query, ";") {
					query = strings.TrimSuffix(query, ";")
				}
				query += " LIMIT 100"
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			rows, err := sessionState.Conn.QueryContext(ctx, query)

			if err != nil {
				logger.LogDatabaseOperation("SELECT", input.Query, 0, err)
				return nil, SelectQueryOutput{}, fmt.Errorf("query execution error: %v", err)
			}
			defer rows.Close()

			columns, err := rows.Columns()
			if err != nil {
				return nil, SelectQueryOutput{}, fmt.Errorf("error getting columns: %v", err)
			}

			results := make([]map[string]interface{}, 0)
			for rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range columns {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					return nil, SelectQueryOutput{}, fmt.Errorf("error scanning row: %v", err)
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
				return nil, SelectQueryOutput{}, fmt.Errorf("error iterating rows: %v", err)
			}

			logger.LogDatabaseOperation("SELECT", input.Query, int64(len(results)), nil)

			message := fmt.Sprintf("OK (%d rows)", len(results))

			output := SelectQueryOutput{
				Data:    results,
				Message: message,
			}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, SelectQueryOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}
