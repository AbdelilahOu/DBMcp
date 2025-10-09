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

	GetListTablesTool().Register(s)

	GetDescribeTableTool().Register(s)

	GetDbInfoTool().Register(s)

	GetExecuteQueryTool().Register(s)

	GetSelectQueryTool().Register(s)

	GetShowQueryTool().Register(s)

	GetExplainQueryTool().Register(s)

	GetListConnectionsTool(cfg).Register(s)
	GetSwitchConnectionTool(cfg).Register(s)
	GetTestConnectionTool(cfg).Register(s)

	GetAnalyzeTableTool().Register(s)
}
