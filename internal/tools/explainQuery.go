package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ExplainQueryInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SQL query to explain"`
}

type ExplainQueryOutput struct {
	Plan string `json:"plan" jsonschema_description:"Query execution plan"`
}

func GetExplainQueryTool() *ToolDefinition[ExplainQueryInput, ExplainQueryOutput] {
	return NewToolDefinition[ExplainQueryInput, ExplainQueryOutput](
		"explain_query",
		"Get query execution plan for performance analysis.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ExplainQueryInput) (*mcp.CallToolResult, ExplainQueryOutput, error) {
			return explainQueryHandler(ctx, req, input)
		},
	)
}

func explainQueryHandler(ctx context.Context, req *mcp.CallToolRequest, input ExplainQueryInput) (*mcp.CallToolResult, ExplainQueryOutput, error) {
	sessionState, err := getActiveSession("default")
	if err != nil {
		return nil, ExplainQueryOutput{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	query := strings.TrimSpace(input.Query)
	queryLower := strings.ToLower(query)
	if strings.HasPrefix(queryLower, "explain") {
		parts := strings.SplitN(query, " ", 2)
		if len(parts) > 1 {
			query = strings.TrimSpace(parts[1])
		}
	}

	var explainQuery string
	var plan string

	explainQuery = fmt.Sprintf("EXPLAIN (FORMAT JSON, ANALYZE false) %s", query)
	rows, err := sessionState.Conn.QueryContext(ctx, explainQuery)
	if err != nil {
		explainQuery = fmt.Sprintf("EXPLAIN %s", query)
		rows, err = sessionState.Conn.QueryContext(ctx, explainQuery)
		if err != nil {
			explainQuery = fmt.Sprintf("EXPLAIN FORMAT=JSON %s", query)
			rows, err = sessionState.Conn.QueryContext(ctx, explainQuery)
			if err != nil {
				explainQuery = fmt.Sprintf("EXPLAIN %s", query)
				rows, err = sessionState.Conn.QueryContext(ctx, explainQuery)
				if err != nil {
					return nil, ExplainQueryOutput{}, fmt.Errorf("failed to explain query: %v", err)
				}
			}
		}
	}
	defer rows.Close()

	var planLines []string
	columns, err := rows.Columns()
	if err != nil {
		return nil, ExplainQueryOutput{}, fmt.Errorf("failed to get columns: %v", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, ExplainQueryOutput{}, fmt.Errorf("scan error: %v", err)
		}

		var rowParts []string
		for i, col := range columns {
			val := values[i]
			var strVal string
			if val == nil {
				strVal = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					strVal = string(v)
				default:
					strVal = fmt.Sprintf("%v", v)
				}
			}

			if len(columns) == 1 {
				rowParts = append(rowParts, strVal)
			} else {
				rowParts = append(rowParts, fmt.Sprintf("%s: %s", col, strVal))
			}
		}

		planLines = append(planLines, strings.Join(rowParts, " | "))
	}

	if err = rows.Err(); err != nil {
		return nil, ExplainQueryOutput{}, fmt.Errorf("rows iteration error: %v", err)
	}

	plan = strings.Join(planLines, "\n")

	if strings.HasPrefix(strings.TrimSpace(plan), "[") || strings.HasPrefix(strings.TrimSpace(plan), "{") {
		var planJSON interface{}
		if err := json.Unmarshal([]byte(plan), &planJSON); err == nil {
			if formatted, err := json.MarshalIndent(planJSON, "", "  "); err == nil {
				plan = string(formatted)
			}
		}
	}

	output := ExplainQueryOutput{
		Plan: plan,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, ExplainQueryOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}
