package tools

import (
	"context"
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
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListViewsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListViews(ctx, session.Conn, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_VIEWS", "list views", 0, err)
				return nil, ListViewsOutput{}, fmt.Errorf("query error: %v", err)
			}

			views := make([]ViewInfo, len(rows))
			for i, r := range rows {
				views[i] = ViewInfo{Name: r.Name, Schema: r.Schema}
			}

			logger.LogDatabaseOperation("LIST_VIEWS", "list views", int64(len(views)), nil)

			output := ListViewsOutput{Views: views}
			message := fmt.Sprintf("Found %d %s", len(views), pluralize(len(views), "view", "views"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}

func GetViewDefinitionTool() *ToolDefinition[GetViewDefinitionInput, GetViewDefinitionOutput] {
	return NewToolDefinition[GetViewDefinitionInput, GetViewDefinitionOutput](
		"get_view_definition",
		"Get view SQL definition. Returns complete SELECT statement showing view construction.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetViewDefinitionInput) (*mcp.CallToolResult, GetViewDefinitionOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetViewDefinitionOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result, err := session.Driver.GetViewDefinition(ctx, session.Conn, input.ViewName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_VIEW_DEFINITION", "get view definition", 0, err)
				return nil, GetViewDefinitionOutput{}, err
			}

			logger.LogDatabaseOperation("GET_VIEW_DEFINITION", "get view definition", 1, nil)

			output := GetViewDefinitionOutput{ViewName: result.ViewName, Schema: result.Schema, Definition: result.Definition}
			message := fmt.Sprintf("Loaded definition for view %s", qualifiedName(input.Schema, input.ViewName))

			return textResult(message), output, nil
		},
	)
}

func GetListMaterializedViewsTool() *ToolDefinition[ListMaterializedViewsInput, ListMaterializedViewsOutput] {
	return NewToolDefinition[ListMaterializedViewsInput, ListMaterializedViewsOutput](
		"list_materialized_views",
		"List materialized views. Store results physically unlike regular views. Note: MySQL/SQLite not supported.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListMaterializedViewsInput) (*mcp.CallToolResult, ListMaterializedViewsOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListMaterializedViewsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListMaterializedViews(ctx, session.Conn, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_MATERIALIZED_VIEWS", "list materialized views", 0, err)
				return nil, ListMaterializedViewsOutput{}, err
			}

			matViews := make([]MaterializedViewInfo, len(rows))
			for i, r := range rows {
				matViews[i] = MaterializedViewInfo{Name: r.Name, Schema: r.Schema}
			}

			logger.LogDatabaseOperation("LIST_MATERIALIZED_VIEWS", "list materialized views", int64(len(matViews)), nil)

			output := ListMaterializedViewsOutput{MaterializedViews: matViews}
			message := fmt.Sprintf("Found %d materialized %s", len(matViews), pluralize(len(matViews), "view", "views"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}
