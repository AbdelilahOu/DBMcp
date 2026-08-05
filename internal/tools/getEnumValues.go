package tools

import (
	"context"
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
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, GetEnumValuesOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			values, err := session.Driver.GetEnumValues(ctx, session.Conn, input.EnumName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_ENUM_VALUES", "get enum values", 0, err)
				return nil, GetEnumValuesOutput{}, err
			}

			if len(values) == 0 {
				return nil, GetEnumValuesOutput{}, fmt.Errorf("enum type '%s' not found in schema '%s'", input.EnumName, input.Schema)
			}

			logger.LogDatabaseOperation("GET_ENUM_VALUES", "get enum values", int64(len(values)), nil)

			output := GetEnumValuesOutput{EnumName: input.EnumName, Schema: input.Schema, Values: values}
			message := fmt.Sprintf("Enum %s has %d %s",
				qualifiedName(output.Schema, output.EnumName), len(values), pluralize(len(values), "value", "values"))

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
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, ListEnumsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListEnums(ctx, session.Conn, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_ENUMS", "list enums", 0, err)
				return nil, ListEnumsOutput{}, err
			}

			enums := make([]EnumInfo, len(rows))
			for i, r := range rows {
				enums[i] = EnumInfo{Name: r.Name, Schema: r.Schema}
			}

			logger.LogDatabaseOperation("LIST_ENUMS", "list enums", int64(len(enums)), nil)

			output := ListEnumsOutput{Enums: enums}
			message := fmt.Sprintf("Found %d %s", len(enums), pluralize(len(enums), "enum", "enum types"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}
