package tools

import (
	"fmt"

	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/state"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func getActiveSession(sessionID string) (*state.DBSessionState, error) {
	if sessionID == "" {
		sessionID = "default"
	}

	sessionState := state.GetSession(sessionID)
	if sessionState == nil {
		sessionState = state.GetOrCreateSession(sessionID, nil)
	}

	if sessionState.Conn == nil {
		return nil, fmt.Errorf("no active DB connection. Use switch_connection tool to connect to a database first")
	}

	return sessionState, nil
}

func RegisterTools(s *mcp.Server, cfg *config.Config) {
	// List Tables Tool
	GetListTablesTool().Register(s)
	// Describe Table Tool
	GetDescribeTableTool().Register(s)
	// Get DB Info Tool
	GetDbInfoTool().Register(s)
	// Execute Query Tool (only if not read-only)
	GetExecuteQueryTool().Register(s)
	// Select Query Tool
	GetSelectQueryTool().Register(s)
	// Explain Query Tool
	GetExplainQueryTool().Register(s)
	// Connection Management Tools (always available)
	GetListConnectionsTool(cfg).Register(s)
	GetSwitchConnectionTool(cfg).Register(s)
	GetTestConnectionTool(cfg).Register(s)
	// Analyze Table Tool
	GetAnalyzeTableTool().Register(s)
}
