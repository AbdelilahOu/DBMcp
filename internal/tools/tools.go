package tools

import (
	"github.com/AbdelilahOu/DBMcp/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, cfg *config.Config, dbType string) {
	advancedEnabled := cfg.AdvancedToolsEnabled()

	GetListConnectionsTool(cfg).Register(s)
	GetSwitchConnectionTool(cfg).Register(s)
	GetTestConnectionTool(cfg).Register(s)

	GetDbInfoTool().Register(s)
	GetListTablesTool().Register(s)
	GetDescribeTableTool().Register(s)
	GetExecuteQueryTool().Register(s)
	GetSelectQueryTool().Register(s)
	GetGenerateIdTool().Register(s)

	if dbType == "" || dbType == "postgres" {
		GetListEnumsTool().Register(s)
		GetEnumValuesTool().Register(s)
	}

	if advancedEnabled {
		GetShowQueryTool().Register(s)
		GetAnalyzeTableTool().Register(s)

		GetListForeignKeysTool().Register(s)
		GetTableRelationshipsTool().Register(s)
		GetListViewsTool().Register(s)
		GetViewDefinitionTool().Register(s)

		GetListTriggersTool().Register(s)
		GetTriggerDefinitionTool().Register(s)

		GetListFunctionsTool().Register(s)
		GetFunctionDefinitionTool().Register(s)

		GetFindColumnTool().Register(s)

		GetListConstraintsTool().Register(s)

		if dbType == "" || dbType == "postgres" {
			GetListMaterializedViewsTool().Register(s)
			GetListSequencesTool().Register(s)
			GetSequenceInfoTool().Register(s)
		}
	}
}
