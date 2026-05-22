package tools

import (
	"context"
	"fmt"
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
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListTablesOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListTables(ctx, session.Conn, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_TABLES", "list tables", 0, err)
				return nil, ListTablesOutput{}, fmt.Errorf("query error: %v", err)
			}

			tables := make([]TableInfo, len(rows))
			for i, r := range rows {
				tables[i] = TableInfo{Name: r.Name, Schema: r.Schema, Type: r.Type}
			}

			logger.LogDatabaseOperation("LIST_TABLES", "list tables", int64(len(tables)), nil)

			output := ListTablesOutput{Tables: tables}
			message := fmt.Sprintf("Found %d %s", len(tables), pluralize(len(tables), "table/view", "tables/views"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}
