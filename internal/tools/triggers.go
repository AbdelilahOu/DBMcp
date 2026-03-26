package tools

import (
	"context"
	"database/sql"
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
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListTriggersOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListTriggersOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var triggers []TriggerInfo

			if sessionState.DBType == "postgres" {
				triggers, err = getPostgresTriggers(ctx, sessionState.Conn, input.TableName, schema)
			} else {
				triggers, err = getMySQLTriggers(ctx, sessionState.Conn, input.TableName, schema)
			}

			if err != nil {
				logger.LogDatabaseOperation("LIST_TRIGGERS", "list triggers", 0, err)
				return nil, ListTriggersOutput{}, err
			}

			logger.LogDatabaseOperation("LIST_TRIGGERS", "list triggers", int64(len(triggers)), nil)

			output := ListTriggersOutput{Triggers: triggers}
			message := fmt.Sprintf("Found %d %s", len(triggers), pluralize(len(triggers), "trigger", "triggers"))
			if input.TableName != "" {
				message += fmt.Sprintf(" for %s", qualifiedName(schema, input.TableName))
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
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetTriggerDefinitionOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetTriggerDefinitionOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var output GetTriggerDefinitionOutput

			if sessionState.DBType == "postgres" {
				output, err = getPostgresTriggerDefinition(ctx, sessionState.Conn, input.TriggerName, schema)
			} else {
				output, err = getMySQLTriggerDefinition(ctx, sessionState.Conn, input.TriggerName, schema)
			}

			if err != nil {
				logger.LogDatabaseOperation("GET_TRIGGER_DEFINITION", "get trigger definition", 0, err)
				return nil, GetTriggerDefinitionOutput{}, err
			}

			logger.LogDatabaseOperation("GET_TRIGGER_DEFINITION", "get trigger definition", 1, nil)
			message := fmt.Sprintf(
				"Loaded trigger %s on %s (%s %s)",
				qualifiedName(output.Schema, output.TriggerName),
				output.TableName,
				output.Timing,
				output.Event,
			)

			return textResult(message), output, nil
		},
	)
}

func getPostgresTriggers(ctx context.Context, conn *sql.DB, tableName, schema string) ([]TriggerInfo, error) {
	query := `
		SELECT
			t.trigger_name,
			t.trigger_schema,
			t.event_object_table,
			t.event_manipulation,
			t.action_timing,
			CASE WHEN t.action_orientation = 'ROW' THEN true ELSE false END as for_each_row
		FROM information_schema.triggers t
		WHERE t.trigger_schema = $1`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND t.event_object_table = $2"
		args = append(args, tableName)
	}

	query += " ORDER BY t.event_object_table, t.trigger_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var triggers []TriggerInfo
	for rows.Next() {
		var trigger TriggerInfo

		err := rows.Scan(
			&trigger.Name,
			&trigger.Schema,
			&trigger.TableName,
			&trigger.Event,
			&trigger.Timing,
			&trigger.ForEachRow,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		triggers = append(triggers, trigger)
	}

	return triggers, rows.Err()
}

func getMySQLTriggers(ctx context.Context, conn *sql.DB, tableName, schema string) ([]TriggerInfo, error) {
	query := `
		SELECT
			trigger_name,
			trigger_schema,
			event_object_table,
			event_manipulation,
			action_timing,
			true as for_each_row
		FROM information_schema.triggers
		WHERE trigger_schema = ?`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND event_object_table = ?"
		args = append(args, tableName)
	}

	query += " ORDER BY event_object_table, trigger_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var triggers []TriggerInfo
	for rows.Next() {
		var trigger TriggerInfo

		err := rows.Scan(
			&trigger.Name,
			&trigger.Schema,
			&trigger.TableName,
			&trigger.Event,
			&trigger.Timing,
			&trigger.ForEachRow,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		triggers = append(triggers, trigger)
	}

	return triggers, rows.Err()
}

func getPostgresTriggerDefinition(ctx context.Context, conn *sql.DB, triggerName, schema string) (GetTriggerDefinitionOutput, error) {
	query := `
		SELECT
			t.trigger_name,
			t.trigger_schema,
			t.event_object_table,
			t.event_manipulation,
			t.action_timing,
			pg_get_triggerdef(pg_trigger.oid) as definition
		FROM information_schema.triggers t
		JOIN pg_trigger ON pg_trigger.tgname = t.trigger_name
		JOIN pg_class ON pg_class.oid = pg_trigger.tgrelid
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE t.trigger_name = $1
			AND t.trigger_schema = $2
			AND pg_namespace.nspname = $2`

	var output GetTriggerDefinitionOutput

	err := conn.QueryRowContext(ctx, query, triggerName, schema).Scan(
		&output.TriggerName,
		&output.Schema,
		&output.TableName,
		&output.Event,
		&output.Timing,
		&output.Definition,
	)

	if err == sql.ErrNoRows {
		return GetTriggerDefinitionOutput{}, fmt.Errorf("trigger '%s' not found in schema '%s'", triggerName, schema)
	}
	if err != nil {
		return GetTriggerDefinitionOutput{}, fmt.Errorf("query error: %v", err)
	}

	return output, nil
}

func getMySQLTriggerDefinition(ctx context.Context, conn *sql.DB, triggerName, schema string) (GetTriggerDefinitionOutput, error) {
	query := `
		SELECT
			trigger_name,
			trigger_schema,
			event_object_table,
			event_manipulation,
			action_timing,
			action_statement as definition
		FROM information_schema.triggers
		WHERE trigger_name = ?
			AND trigger_schema = ?`

	var output GetTriggerDefinitionOutput

	err := conn.QueryRowContext(ctx, query, triggerName, schema).Scan(
		&output.TriggerName,
		&output.Schema,
		&output.TableName,
		&output.Event,
		&output.Timing,
		&output.Definition,
	)

	if err == sql.ErrNoRows {
		return GetTriggerDefinitionOutput{}, fmt.Errorf("trigger '%s' not found in schema '%s'", triggerName, schema)
	}
	if err != nil {
		return GetTriggerDefinitionOutput{}, fmt.Errorf("query error: %v", err)
	}

	return output, nil
}
