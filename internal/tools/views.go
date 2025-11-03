package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListViewsInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter views"`
}

type ViewInfo struct {
	Name   string `json:"name" jsonschema_description:"View name"`
	Schema string `json:"sch" jsonschema_description:"Schema name"`
}

type ListViewsOutput struct {
	Views []ViewInfo `json:"views" jsonschema_description:"Array of view information"`
}

type GetViewDefinitionInput struct {
	ViewName string `json:"view" jsonschema:"required" jsonschema_description:"Name of the view"`
	Schema   string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type GetViewDefinitionOutput struct {
	ViewName   string `json:"view" jsonschema_description:"Name of the view"`
	Schema     string `json:"sch" jsonschema_description:"Schema of the view"`
	Definition string `json:"def" jsonschema_description:"SQL definition of the view"`
}

type ListMaterializedViewsInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter materialized views"`
}

type MaterializedViewInfo struct {
	Name   string `json:"name" jsonschema_description:"Materialized view name"`
	Schema string `json:"sch" jsonschema_description:"Schema name"`
}

type ListMaterializedViewsOutput struct {
	MaterializedViews []MaterializedViewInfo `json:"materialized_views" jsonschema_description:"Array of materialized view information"`
}

func GetListViewsTool() *ToolDefinition[ListViewsInput, ListViewsOutput] {
	return NewToolDefinition[ListViewsInput, ListViewsOutput](
		"list_views",
		"List views in DB or schema. Returns names, schemas. Views are virtual tables. Use get_view_definition for SQL.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListViewsInput) (*mcp.CallToolResult, ListViewsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListViewsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListViewsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
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
							table_schema as schema_name
						FROM information_schema.views
						WHERE table_schema = $1
						ORDER BY table_name`
				} else {
					query = `
						SELECT
							table_name as name,
							table_schema as schema_name
						FROM information_schema.views
						WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
						ORDER BY table_name`
				}
			} else if sessionState.DBType == "mysql" {
				if input.Schema != "" {
					query = `
						SELECT
							table_name as name,
							table_schema as schema_name
						FROM information_schema.views
						WHERE table_schema = ?
						ORDER BY table_name`
				} else {
					query = `
						SELECT
							table_name as name,
							table_schema as schema_name
						FROM information_schema.views
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
				logger.LogDatabaseOperation("LIST_VIEWS", query, 0, err)
				return nil, ListViewsOutput{}, fmt.Errorf("query error: %v", err)
			}
			defer rows.Close()

			var views []ViewInfo
			for rows.Next() {
				var name, schemaName string
				if err := rows.Scan(&name, &schemaName); err != nil {
					return nil, ListViewsOutput{}, fmt.Errorf("scan error: %v", err)
				}

				views = append(views, ViewInfo{
					Name:   name,
					Schema: schemaName,
				})
			}

			if err = rows.Err(); err != nil {
				return nil, ListViewsOutput{}, fmt.Errorf("rows iteration error: %v", err)
			}

			logger.LogDatabaseOperation("LIST_VIEWS", query, int64(len(views)), nil)

			output := ListViewsOutput{Views: views}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListViewsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func GetViewDefinitionTool() *ToolDefinition[GetViewDefinitionInput, GetViewDefinitionOutput] {
	return NewToolDefinition[GetViewDefinitionInput, GetViewDefinitionOutput](
		"get_view_definition",
		"Get view SQL definition. Returns complete SELECT statement showing view construction.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetViewDefinitionInput) (*mcp.CallToolResult, GetViewDefinitionOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetViewDefinitionOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetViewDefinitionOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			var query string
			var definition string

			if sessionState.DBType == "postgres" {
				query = `
					SELECT view_definition
					FROM information_schema.views
					WHERE table_name = $1 AND table_schema = $2`
			} else if sessionState.DBType == "mysql" {
				query = `
					SELECT view_definition
					FROM information_schema.views
					WHERE table_name = ? AND table_schema = ?`
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err = sessionState.Conn.QueryRowContext(ctx, query, input.ViewName, schema).Scan(&definition)
			if err == sql.ErrNoRows {
				return nil, GetViewDefinitionOutput{}, fmt.Errorf("view '%s' not found in schema '%s'", input.ViewName, schema)
			}
			if err != nil {
				logger.LogDatabaseOperation("GET_VIEW_DEFINITION", query, 0, err)
				return nil, GetViewDefinitionOutput{}, fmt.Errorf("query error: %v", err)
			}

			logger.LogDatabaseOperation("GET_VIEW_DEFINITION", query, 1, nil)

			output := GetViewDefinitionOutput{
				ViewName:   input.ViewName,
				Schema:     schema,
				Definition: definition,
			}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, GetViewDefinitionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func GetListMaterializedViewsTool() *ToolDefinition[ListMaterializedViewsInput, ListMaterializedViewsOutput] {
	return NewToolDefinition[ListMaterializedViewsInput, ListMaterializedViewsOutput](
		"list_materialized_views",
		"List materialized views (PostgreSQL only). Store results physically unlike regular views. Note: MySQL not supported.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListMaterializedViewsInput) (*mcp.CallToolResult, ListMaterializedViewsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListMaterializedViewsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListMaterializedViewsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			if sessionState.DBType == "mysql" {
				return nil, ListMaterializedViewsOutput{}, fmt.Errorf("MySQL does not support materialized views. Use list_views for regular views")
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			var query string

			if input.Schema != "" {
				query = `
					SELECT
						matviewname as name,
						schemaname as schema_name
					FROM pg_matviews
					WHERE schemaname = $1
					ORDER BY matviewname`
			} else {
				query = `
					SELECT
						matviewname as name,
						schemaname as schema_name
					FROM pg_matviews
					WHERE schemaname NOT IN ('information_schema', 'pg_catalog')
					ORDER BY matviewname`
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
				logger.LogDatabaseOperation("LIST_MATERIALIZED_VIEWS", query, 0, err)
				return nil, ListMaterializedViewsOutput{}, fmt.Errorf("query error: %v", err)
			}
			defer rows.Close()

			var matViews []MaterializedViewInfo
			for rows.Next() {
				var name, schemaName string
				if err := rows.Scan(&name, &schemaName); err != nil {
					return nil, ListMaterializedViewsOutput{}, fmt.Errorf("scan error: %v", err)
				}

				matViews = append(matViews, MaterializedViewInfo{
					Name:   name,
					Schema: schemaName,
				})
			}

			if err = rows.Err(); err != nil {
				return nil, ListMaterializedViewsOutput{}, fmt.Errorf("rows iteration error: %v", err)
			}

			logger.LogDatabaseOperation("LIST_MATERIALIZED_VIEWS", query, int64(len(matViews)), nil)

			output := ListMaterializedViewsOutput{MaterializedViews: matViews}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListMaterializedViewsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}
