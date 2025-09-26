package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListTablesInput struct {
	Schema string `json:"schema,omitempty" jsonschema_description:"Optional schema name to filter tables (defaults to 'public' for PostgreSQL)"`
}

type TableInfo struct {
	Name   string `json:"name" jsonschema_description:"Table name"`
	Schema string `json:"schema" jsonschema_description:"Schema name"`
	Type   string `json:"type" jsonschema_description:"Table type (table, view, etc.)"`
}

type ListTablesOutput struct {
	Tables []TableInfo `json:"tables" jsonschema_description:"Array of table information"`
}

func GetListTablesTool() *ToolDefinition[ListTablesInput, ListTablesOutput] {
	return NewToolDefinition[ListTablesInput, ListTablesOutput](
		"list_tables",
		"List all tables in the database with metadata.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListTablesInput) (*mcp.CallToolResult, ListTablesOutput, error) {
			return listTablesHandler(ctx, req, input)
		},
	)
}

func listTablesHandler(ctx context.Context, req *mcp.CallToolRequest, input ListTablesInput) (*mcp.CallToolResult, ListTablesOutput, error) {
	sessionState, err := getActiveSession("default")
	if err != nil {
		return nil, ListTablesOutput{}, err
	}

	schema := input.Schema
	if schema == "" {
		schema = "public"
	}

	var query string

	detectQuery := "SELECT 1 FROM information_schema.tables WHERE table_schema = 'information_schema' LIMIT 1"
	_, err = sessionState.Conn.QueryContext(ctx, detectQuery)

	if err != nil {
		query = `
			SELECT
				table_name as name,
				table_schema as schema_name,
				table_type as table_type
			FROM information_schema.tables
			WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
			ORDER BY table_name`

		if input.Schema != "" {
			query = `
				SELECT
					table_name as name,
					table_schema as schema_name,
					table_type as table_type
				FROM information_schema.tables
				WHERE table_schema = ?
				ORDER BY table_name`
		}
	} else {
		if input.Schema != "" {
			query = `
				SELECT
					table_name as name,
					table_schema as schema_name,
					table_type as table_type
				FROM information_schema.tables
				WHERE table_schema = $1
				ORDER BY table_name`
		} else {
			query = `
				SELECT
					table_name as name,
					table_schema as schema_name,
					table_type as table_type
				FROM information_schema.tables
				WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
				ORDER BY table_name`
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var rows *sql.Rows
	if input.Schema != "" && !strings.Contains(query, "NOT IN") {
		rows, err = sessionState.Conn.QueryContext(ctx, query, schema)
	} else {
		rows, err = sessionState.Conn.QueryContext(ctx, query)
	}

	if err != nil {
		logger.LogDatabaseOperation("LIST_TABLES", query, 0, err)
		return nil, ListTablesOutput{}, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name, schemaName, tableType string
		if err := rows.Scan(&name, &schemaName, &tableType); err != nil {
			return nil, ListTablesOutput{}, fmt.Errorf("scan error: %v", err)
		}

		normalizedType := strings.ToLower(tableType)
		if strings.Contains(normalizedType, "base table") {
			normalizedType = "table"
		} else if strings.Contains(normalizedType, "view") {
			normalizedType = "view"
		}

		tables = append(tables, TableInfo{
			Name:   name,
			Schema: schemaName,
			Type:   normalizedType,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, ListTablesOutput{}, fmt.Errorf("rows iteration error: %v", err)
	}

	// Log successful database operation
	logger.LogDatabaseOperation("LIST_TABLES", query, int64(len(tables)), nil)

	output := ListTablesOutput{Tables: tables}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, ListTablesOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}
