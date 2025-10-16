package tools

import (
	"github.com/AbdelilahOu/DBMcp/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, cfg *config.Config) {
	GetListConnectionsTool(cfg).Register(s)
	GetSwitchConnectionTool(cfg).Register(s)
	GetTestConnectionTool(cfg).Register(s)

	GetDbInfoTool().Register(s)
	GetListTablesTool().Register(s)
	GetDescribeTableTool().Register(s)
	GetAnalyzeTableTool().Register(s)
	GetExecuteQueryTool().Register(s)
	GetSelectQueryTool().Register(s)
	GetShowQueryTool().Register(s)
	GetExplainQueryTool().Register(s)
	GetGenerateIdTool().Register(s)
}
