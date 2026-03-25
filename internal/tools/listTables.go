package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListTablesInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter tables (defaults to 'public' for PostgreSQL)"`
}

type TableInfo struct {
	Name   string `json:"name" jsonschema_description:"Table name"`
	Schema string `json:"sch" jsonschema_description:"Schema name"`
	Type   string `json:"type,omitempty" jsonschema_description:"Table type (table, view, etc.)"`
}

type ListTablesOutput struct {
	Tables []TableInfo `json:"tbls" jsonschema_description:"Array of table information"`
}

func GetListTablesTool() *ToolDefinition[ListTablesInput, ListTablesOutput] {
	return NewToolDefinition[ListTablesInput, ListTablesOutput](
		"list_tables",
		"List all tables/views in DB or schema. Returns names, schemas, types. Use describe_table/analyze_table for details.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListTablesInput) (*mcp.CallToolResult, ListTablesOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListTablesOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListTablesOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			var query string

			if sessionState.DBType == "postgres" {
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
			} else if sessionState.DBType == "mysql" {
				if input.Schema != "" {
					query = `
						SELECT
							table_name as name,
							table_schema as schema_name,
							table_type as table_type
						FROM information_schema.tables
						WHERE table_schema = ?
						ORDER BY table_name`
				} else {
					query = `
						SELECT
							table_name as name,
							table_schema as schema_name,
							table_type as table_type
						FROM information_schema.tables
						WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
						ORDER BY table_name`
				}
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var rows *sql.Rows
			if input.Schema != "" {
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

			logger.LogDatabaseOperation("LIST_TABLES", query, int64(len(tables)), nil)

			output := ListTablesOutput{Tables: tables}
			message := fmt.Sprintf("Found %d %s", len(tables), pluralize(len(tables), "table/view", "tables/views"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", schema)
			}

			return textResult(message), output, nil
		},
	)
}
