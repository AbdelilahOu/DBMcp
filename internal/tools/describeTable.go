package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DescribeTableInput struct {
	TableName string `json:"tbl" jsonschema:"required" jsonschema_description:"Name of the table to describe"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name (defaults to 'public' for PostgreSQL)"`
}

type ColumnInfo struct {
	Name          string `json:"col" jsonschema_description:"Column name"`
	DataType      string `json:"dtype" jsonschema_description:"Data type of the column"`
	IsNullable    bool   `json:"null,omitempty" jsonschema_description:"Whether the column can contain NULL values"`
	IsPrimaryKey  bool   `json:"pk,omitempty" jsonschema_description:"Whether the column is part of the primary key"`
	DefaultValue  string `json:"def,omitempty" jsonschema_description:"Default value for the column"`
	CharMaxLength *int   `json:"maxlen,omitempty" jsonschema_description:"Maximum length for character types"`
}

type IndexInfo struct {
	Name     string   `json:"name" jsonschema_description:"Index name"`
	Columns  []string `json:"cols" jsonschema_description:"Columns included in the index"`
	IsUnique bool     `json:"uniq,omitempty" jsonschema_description:"Whether the index is unique"`
}

type DescribeTableOutput struct {
	Columns []ColumnInfo `json:"cols" jsonschema_description:"Array of column information"`
	Indexes []IndexInfo  `json:"idxs,omitempty" jsonschema_description:"Array of index information"`
}

func GetDescribeTableTool() *ToolDefinition[DescribeTableInput, DescribeTableOutput] {
	return NewToolDefinition[DescribeTableInput, DescribeTableOutput](
		"describe_table",
		"Get table schema: columns (names, types, nullability, PKs, defaults), indexes. For schema design, not data/stats. Use analyze_table for row counts/sizes.",
		func(ctx context.Context, req *mcp.CallToolRequest, input DescribeTableInput) (*mcp.CallToolResult, DescribeTableOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, DescribeTableOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result, err := session.Driver.DescribeTable(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("DESCRIBE_TABLE", fmt.Sprintf("DESCRIBE %s.%s", input.Schema, input.TableName), 0, err)
				return nil, DescribeTableOutput{}, err
			}

			columns := make([]ColumnInfo, len(result.Columns))
			for i, c := range result.Columns {
				columns[i] = ColumnInfo{
					Name:          c.Name,
					DataType:      c.DataType,
					IsNullable:    c.IsNullable,
					IsPrimaryKey:  c.IsPrimaryKey,
					DefaultValue:  c.DefaultValue,
					CharMaxLength: c.CharMaxLength,
				}
			}

			indexes := make([]IndexInfo, len(result.Indexes))
			for i, idx := range result.Indexes {
				indexes[i] = IndexInfo{Name: idx.Name, Columns: idx.Columns, IsUnique: idx.IsUnique}
			}

			logger.LogDatabaseOperation("DESCRIBE_TABLE", fmt.Sprintf("DESCRIBE %s.%s", input.Schema, input.TableName), int64(len(columns)), nil)

			output := DescribeTableOutput{Columns: columns, Indexes: indexes}
			message := fmt.Sprintf(
				"Schema for %s: %d %s, %d %s",
				qualifiedName(input.Schema, input.TableName),
				len(columns), pluralize(len(columns), "column", "columns"),
				len(indexes), pluralize(len(indexes), "index", "indexes"),
			)

			return textResult(message), output, nil
		},
	)
}
