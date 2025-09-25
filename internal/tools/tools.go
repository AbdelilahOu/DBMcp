package tools

import (
	"context"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddDBTools registers DB tools (conditional on toolsets, like GitHub MCP)
func AddDBTools(s *mcp.Server, dbClient *client.DBClient, readOnly bool, toolsets []string) {
	if !contains(toolsets, "db") {
		return
	}

	// execute_select tool: Runs SELECT, uses session for conn
	mcp.AddTool(s, &mcp.Tool{
		Name:        "execute_select",
		Description: "Execute a SELECT query on the database and return JSON results.",
		InputSchema: mcpdb.ExecuteSelectInputSchema(), // From types
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ExecuteSelectInput) (*mcp.CallToolResult, mcpdb.ExecuteSelectOutput, error) {
		return executeSelectHandler(ctx, req, input, dbClient, readOnly)
	})
}
