package tools

import (
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/driver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, cfg *config.Config, drv driver.Driver) {
	advancedEnabled := cfg.AdvancedToolsEnabled()

	GetListConnectionsTool(cfg).Register(s)
	GetSwitchConnectionTool(cfg).Register(s)
	GetTestConnectionTool(cfg).Register(s)

	GetDbInfoTool().Register(s)
	GetListSchemasTool().Register(s)
	GetListTablesTool().Register(s)
	GetDescribeTableTool().Register(s)
	GetExecuteQueryTool().Register(s)
	GetSelectQueryTool().Register(s)
	GetGenerateIdTool().Register(s)

	if drv != nil && drv.SupportsEnums() {
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

		if drv == nil || drv.SupportsFunctions() {
			GetListFunctionsTool().Register(s)
			GetFunctionDefinitionTool().Register(s)
		}

		GetFindColumnTool().Register(s)
		GetListConstraintsTool().Register(s)

		if drv != nil && drv.SupportsMaterializedViews() {
			GetListMaterializedViewsTool().Register(s)
		}

		if drv != nil && drv.SupportsSequences() {
			GetListSequencesTool().Register(s)
			GetSequenceInfoTool().Register(s)
		}
	}
}
