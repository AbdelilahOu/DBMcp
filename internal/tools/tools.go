package tools

import (
	"github.com/AbdelilahOu/DBMcp/internal/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, dbClient *client.DBClient, readOnly bool) {
	// Execute Select Tool
	GetExecuteSelectTool(dbClient, readOnly).Register(s)
	// List Tables Tool
	GetListTablesTool(dbClient).Register(s)
	// Describe Table Tool
	GetDescribeTableTool(dbClient).Register(s)
	// Get DB Info Tool
	GetDbInfoTool(dbClient).Register(s)
	// Execute Query Tool (only if not read-only)
	if !readOnly {
		GetExecuteQueryTool(dbClient, readOnly).Register(s)
	}
	// Explain Query Tool
	GetExplainQueryTool(dbClient).Register(s)
	// List Connections Tool
	GetListConnectionsTool().Register(s)
	// Switch Connection Tool
	GetSwitchConnectionTool().Register(s)
	// Test Connection Tool
	GetTestConnectionTool(dbClient).Register(s)
	// Analyze Table Tool
	GetAnalyzeTableTool(dbClient).Register(s)
}
