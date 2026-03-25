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

type GetEnumValuesInput struct {
	EnumName string `json:"enum" jsonschema:"required" jsonschema_description:"Name of the enum type"`
	Schema   string `json:"sch,omitempty" jsonschema_description:"Optional schema name where the enum is defined"`
}

type GetEnumValuesOutput struct {
	EnumName string   `json:"enum" jsonschema_description:"Name of the enum type"`
	Schema   string   `json:"sch" jsonschema_description:"Schema where the enum is defined"`
	Values   []string `json:"values" jsonschema_description:"Array of enum values in order"`
}

func GetEnumValuesTool() *ToolDefinition[GetEnumValuesInput, GetEnumValuesOutput] {
	return NewToolDefinition[GetEnumValuesInput, GetEnumValuesOutput](
		"get_enum_values",
		"Get enum values for type. Returns name, schema, ordered list. Note: MySQL has no standalone ENUMs - use describe_table for column-level ENUMs.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetEnumValuesInput) (*mcp.CallToolResult, GetEnumValuesOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetEnumValuesOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetEnumValuesOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			if sessionState.DBType == "mysql" {
				return nil, GetEnumValuesOutput{}, fmt.Errorf("MySQL does not support standalone ENUM types. ENUM constraints are defined at the column level. Use describe_table to see column ENUM definitions")
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			query := `
				SELECT
					e.enumlabel as enum_value
				FROM pg_type t
				JOIN pg_enum e ON t.oid = e.enumtypid
				JOIN pg_namespace n ON t.typnamespace = n.oid
				WHERE t.typname = $1
					AND n.nspname = $2
				ORDER BY e.enumsortorder`

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := sessionState.Conn.QueryContext(ctx, query, input.EnumName, schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_ENUM_VALUES", query, 0, err)
				return nil, GetEnumValuesOutput{}, fmt.Errorf("query error: %v", err)
			}
			defer rows.Close()

			var values []string
			for rows.Next() {
				var value string
				if err := rows.Scan(&value); err != nil {
					return nil, GetEnumValuesOutput{}, fmt.Errorf("scan error: %v", err)
				}
				values = append(values, value)
			}

			if err = rows.Err(); err != nil {
				return nil, GetEnumValuesOutput{}, fmt.Errorf("rows iteration error: %v", err)
			}

			if len(values) == 0 {
				return nil, GetEnumValuesOutput{}, fmt.Errorf("enum type '%s' not found in schema '%s'", input.EnumName, schema)
			}

			logger.LogDatabaseOperation("GET_ENUM_VALUES", query, int64(len(values)), nil)

			output := GetEnumValuesOutput{
				EnumName: input.EnumName,
				Schema:   schema,
				Values:   values,
			}
			message := fmt.Sprintf(
				"Enum %s has %d %s",
				qualifiedName(output.Schema, output.EnumName),
				len(values),
				pluralize(len(values), "value", "values"),
			)

			return textResult(message), output, nil
		},
	)
}

type ListEnumsInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter enums"`
}

type EnumInfo struct {
	Name   string `json:"name" jsonschema_description:"Enum type name"`
	Schema string `json:"sch" jsonschema_description:"Schema name where the enum is defined"`
}

type ListEnumsOutput struct {
	Enums []EnumInfo `json:"enums" jsonschema_description:"Array of enum information"`
}

func GetListEnumsTool() *ToolDefinition[ListEnumsInput, ListEnumsOutput] {
	return NewToolDefinition[ListEnumsInput, ListEnumsOutput](
		"list_enums",
		"List enum types in DB or schema. Returns names, schemas. Use get_enum_values for values. Note: MySQL has no standalone ENUMs.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListEnumsInput) (*mcp.CallToolResult, ListEnumsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListEnumsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListEnumsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			if sessionState.DBType == "mysql" {
				return nil, ListEnumsOutput{}, fmt.Errorf("MySQL does not support standalone ENUM types. ENUM constraints are defined at the column level. Use describe_table to see column ENUM definitions")
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			var query string

			if input.Schema != "" {
				query = `
					SELECT
						t.typname as enum_name,
						n.nspname as schema_name
					FROM pg_type t
					JOIN pg_namespace n ON t.typnamespace = n.oid
					WHERE t.typtype = 'e'
						AND n.nspname = $1
					ORDER BY n.nspname, t.typname`
			} else {
				query = `
					SELECT
						t.typname as enum_name,
						n.nspname as schema_name
					FROM pg_type t
					JOIN pg_namespace n ON t.typnamespace = n.oid
					WHERE t.typtype = 'e'
						AND n.nspname NOT IN ('information_schema', 'pg_catalog')
					ORDER BY n.nspname, t.typname`
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var rows *sql.Rows
			if input.Schema != "" {
				rows, err = sessionState.Conn.QueryContext(ctx, query, schema)
			} else {
				rows, err = sessionState.Conn.QueryContext(ctx, query)
			}

			if err != nil {
				logger.LogDatabaseOperation("LIST_ENUMS", query, 0, err)
				return nil, ListEnumsOutput{}, fmt.Errorf("query error: %v", err)
			}
			defer rows.Close()

			var enums []EnumInfo
			for rows.Next() {
				var name, schemaName string
				if err := rows.Scan(&name, &schemaName); err != nil {
					return nil, ListEnumsOutput{}, fmt.Errorf("scan error: %v", err)
				}

				enums = append(enums, EnumInfo{
					Name:   name,
					Schema: schemaName,
				})
			}

			if err = rows.Err(); err != nil {
				return nil, ListEnumsOutput{}, fmt.Errorf("rows iteration error: %v", err)
			}

			logger.LogDatabaseOperation("LIST_ENUMS", query, int64(len(enums)), nil)

			output := ListEnumsOutput{Enums: enums}
			message := fmt.Sprintf("Found %d %s", len(enums), pluralize(len(enums), "enum", "enums"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", schema)
			}

			return textResult(message), output, nil
		},
	)
}
