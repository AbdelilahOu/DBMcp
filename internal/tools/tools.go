package tools

import (
	"context"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func AddDBTools(s *mcp.Server, dbClient *client.DBClient, readOnly bool, toolsets []string) {
	if !contains(toolsets, "db") {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "execute_select",
		Description: "Execute a SELECT query on the database and return JSON results.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ExecuteSelectInput) (*mcp.CallToolResult, mcpdb.ExecuteSelectOutput, error) {
		return executeSelectHandler(ctx, req, input, dbClient, readOnly)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tables",
		Description: "List all tables in the database with metadata.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ListTablesInput) (*mcp.CallToolResult, mcpdb.ListTablesOutput, error) {
		return listTablesHandler(ctx, req, input, dbClient)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "describe_table",
		Description: "Get detailed information about table structure, columns, and indexes.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.DescribeTableInput) (*mcp.CallToolResult, mcpdb.DescribeTableOutput, error) {
		return describeTableHandler(ctx, req, input, dbClient)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_db_info",
		Description: "Get general database information and statistics.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.GetDBInfoInput) (*mcp.CallToolResult, mcpdb.GetDBInfoOutput, error) {
		return getDBInfoHandler(ctx, req, input, dbClient)
	})

	if !readOnly {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "execute_query",
			Description: "Execute any SQL query (INSERT, UPDATE, DELETE, etc.) with proper permissions.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ExecuteQueryInput) (*mcp.CallToolResult, mcpdb.ExecuteQueryOutput, error) {
			return executeQueryHandler(ctx, req, input, dbClient, readOnly)
		})
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "explain_query",
		Description: "Get query execution plan for performance analysis.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ExplainQueryInput) (*mcp.CallToolResult, mcpdb.ExplainQueryOutput, error) {
		return explainQueryHandler(ctx, req, input, dbClient)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_connections",
		Description: "List all available named connections from config.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ListConnectionsInput) (*mcp.CallToolResult, mcpdb.ListConnectionsOutput, error) {
		return listConnectionsHandler(ctx, req, input)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "switch_connection",
		Description: "Switch to a different database connection during the session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.SwitchConnectionInput) (*mcp.CallToolResult, mcpdb.SwitchConnectionOutput, error) {
		return switchConnectionHandler(ctx, req, input)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "test_connection",
		Description: "Test connectivity to a database before executing queries.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.TestConnectionInput) (*mcp.CallToolResult, mcpdb.TestConnectionOutput, error) {
		return testConnectionHandler(ctx, req, input, dbClient)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "analyze_table",
		Description: "Get table statistics (row count, size, column stats).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.AnalyzeTableInput) (*mcp.CallToolResult, mcpdb.AnalyzeTableOutput, error) {
		return analyzeTableHandler(ctx, req, input, dbClient)
	})
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
