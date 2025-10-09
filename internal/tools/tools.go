package tools

import (
	"github.com/AbdelilahOu/DBMcp/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
