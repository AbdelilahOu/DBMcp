package tools

import (
	"context"
	"database/sql"
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
		return mcp.NewErrorResult("read_only_violation", "Read-only mode: only SELECT queries allowed"), mcpdb.ExecuteSelectOutput{}, nil
	}

	// Get/create session state (key use case: reuse conn across multi-turn queries)
	state := state.GetOrCreateSession(req.SessionID, dbClient)
	if state.Conn == nil {
		return mcp.NewErrorResult("no_session_conn", "No active DB connection in session"), mcpdb.ExecuteSelectOutput{}, nil
	}

	// Timeout context (5s for safety)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Exec query
	rows, err := state.Conn.QueryContext(ctx, input.Query)
	if err != nil {
		return mcp.NewErrorResult("query_failed", fmt.Sprintf("Query error: %v", err)), mcpdb.ExecuteSelectOutput{}, nil
	}
	defer rows.Close()

	// Scan results to JSON (handles dynamic columns)
	columns, err := rows.Columns()
	if err != nil {
		return mcp.NewErrorResult("scan_failed", fmt.Sprintf("Columns error: %v", err)), mcpdb.ExecuteSelectOutput{}, nil
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		vals := make([]interface{}, len(columns))
		for i := range vals {
			vals[i] = new(sql.NullString) // Handle nulls/strings; extend for types
		}
		if err := rows.Scan(vals...); err != nil {
			return mcp.NewErrorResult("row_scan_failed", fmt.Sprintf("Scan error: %v", err)), mcpdb.ExecuteSelectOutput{}, nil
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := vals[i].(*sql.NullString)
			if val.Valid {
				row[col] = val.String
			} else {
				row[col] = nil
			}
		}
		results = append(results, row)
	}

	// Marshal output
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewErrorResult("marshal_failed", fmt.Sprintf("JSON error: %v", err)), mcpdb.ExecuteSelectOutput{}, nil
	}

	return nil, mcpdb.ExecuteSelectOutput{Results: string(jsonBytes)}, nil
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
