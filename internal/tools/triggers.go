package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListTriggersInput struct {
	TableName string `json:"tbl,omitempty" jsonschema_description:"Optional table name to filter triggers"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type TriggerInfo struct {
	Name       string `json:"name" jsonschema_description:"Trigger name"`
	Schema     string `json:"sch" jsonschema_description:"Schema name"`
	TableName  string `json:"tbl" jsonschema_description:"Table the trigger is attached to"`
	Event      string `json:"event" jsonschema_description:"Trigger event (INSERT, UPDATE, DELETE)"`
	Timing     string `json:"timing" jsonschema_description:"When trigger fires (BEFORE, AFTER, INSTEAD OF)"`
	ForEachRow bool   `json:"for_each_row,omitempty" jsonschema_description:"Whether trigger fires for each row or once per statement"`
}

type ListTriggersOutput struct {
	Triggers []TriggerInfo `json:"triggers" jsonschema_description:"Array of trigger information"`
}

type GetTriggerDefinitionInput struct {
	TriggerName string `json:"trg" jsonschema:"required" jsonschema_description:"Name of the trigger"`
	Schema      string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type GetTriggerDefinitionOutput struct {
	TriggerName string `json:"trg" jsonschema_description:"Name of the trigger"`
	Schema      string `json:"sch" jsonschema_description:"Schema of the trigger"`
	TableName   string `json:"tbl" jsonschema_description:"Table the trigger is attached to"`
	Event       string `json:"event" jsonschema_description:"Trigger event"`
	Timing      string `json:"timing" jsonschema_description:"When trigger fires"`
	Definition  string `json:"def" jsonschema_description:"SQL definition or body of the trigger"`
}

func GetListTriggersTool() *ToolDefinition[ListTriggersInput, ListTriggersOutput] {
	return NewToolDefinition[ListTriggersInput, ListTriggersOutput](
		"list_triggers",
		"List triggers in DB or table. Automated actions on INSERT/UPDATE/DELETE. Returns names, tables, events, timing (BEFORE/AFTER). Use get_trigger_definition for SQL.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListTriggersInput) (*mcp.CallToolResult, ListTriggersOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, ListTriggersOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListTriggers(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_TRIGGERS", "list triggers", 0, err)
				return nil, ListTriggersOutput{}, err
			}

			triggers := make([]TriggerInfo, len(rows))
			for i, r := range rows {
				triggers[i] = TriggerInfo{Name: r.Name, Schema: r.Schema, TableName: r.TableName, Event: r.Event, Timing: r.Timing, ForEachRow: r.ForEachRow}
			}

			logger.LogDatabaseOperation("LIST_TRIGGERS", "list triggers", int64(len(triggers)), nil)

			output := ListTriggersOutput{Triggers: triggers}
			message := fmt.Sprintf("Found %d %s", len(triggers), pluralize(len(triggers), "trigger", "triggers"))
			if input.TableName != "" {
				message += fmt.Sprintf(" for %s", qualifiedName(input.Schema, input.TableName))
			}

			return textResult(message), output, nil
		},
	)
}

func GetTriggerDefinitionTool() *ToolDefinition[GetTriggerDefinitionInput, GetTriggerDefinitionOutput] {
	return NewToolDefinition[GetTriggerDefinitionInput, GetTriggerDefinitionOutput](
		"get_trigger_definition",
		"Get trigger SQL definition. Returns complete definition: function/body, event type, timing, table. Shows implementation.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetTriggerDefinitionInput) (*mcp.CallToolResult, GetTriggerDefinitionOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, GetTriggerDefinitionOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result, err := session.Driver.GetTriggerDefinition(ctx, session.Conn, input.TriggerName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_TRIGGER_DEFINITION", "get trigger definition", 0, err)
				return nil, GetTriggerDefinitionOutput{}, err
			}

			logger.LogDatabaseOperation("GET_TRIGGER_DEFINITION", "get trigger definition", 1, nil)

			output := GetTriggerDefinitionOutput{
				TriggerName: result.TriggerName, Schema: result.Schema, TableName: result.TableName,
				Event: result.Event, Timing: result.Timing, Definition: result.Definition,
			}
			message := fmt.Sprintf("Loaded trigger %s on %s (%s %s)",
				qualifiedName(output.Schema, output.TriggerName), output.TableName, output.Timing, output.Event)

			return textResult(message), output, nil
		},
	)
}
