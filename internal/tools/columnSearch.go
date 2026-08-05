package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FindColumnInput struct {
	ColumnName string `json:"col" jsonschema:"required" jsonschema_description:"Column name to search for (supports partial matching)"`
	Schema     string `json:"sch,omitempty" jsonschema_description:"Optional schema name to limit search"`
	TableName  string `json:"tbl,omitempty" jsonschema_description:"Optional table name to limit search to a specific table"`
	ExactMatch bool   `json:"exact_match,omitempty" jsonschema_description:"If true, only exact matches are returned. If false (default), partial matches are included"`
}

type ColumnLocation struct {
	TableName   string `json:"tbl" jsonschema_description:"Table containing the column"`
	TableSchema string `json:"sch" jsonschema_description:"Schema of the table"`
	ColumnName  string `json:"col" jsonschema_description:"Column name"`
	DataType    string `json:"dtype" jsonschema_description:"Data type of the column"`
	IsNullable  bool   `json:"null" jsonschema_description:"Whether the column allows NULL values"`
	Position    int    `json:"pos" jsonschema_description:"Ordinal position of the column in the table"`
}

type FindColumnOutput struct {
	Columns []ColumnLocation `json:"columns" jsonschema_description:"Array of column locations matching the search criteria"`
}

func GetFindColumnTool() *ToolDefinition[FindColumnInput, FindColumnOutput] {
	return NewToolDefinition[FindColumnInput, FindColumnOutput](
		"find_column",
		"Search columns by name across tables. Supports exact/partial match. Returns table, schema, column, type, position. For locating columns in large schemas.",
		func(ctx context.Context, req *mcp.CallToolRequest, input FindColumnInput) (*mcp.CallToolResult, FindColumnOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, FindColumnOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			rows, err := session.Driver.FindColumns(ctx, session.Conn, input.ColumnName, input.TableName, input.Schema, input.ExactMatch)
			if err != nil {
				logger.LogDatabaseOperation("FIND_COLUMN", fmt.Sprintf("search for column: %s", input.ColumnName), 0, err)
				return nil, FindColumnOutput{}, err
			}

			cols := make([]ColumnLocation, len(rows))
			for i, r := range rows {
				cols[i] = ColumnLocation{
					TableName:   r.TableName,
					TableSchema: r.TableSchema,
					ColumnName:  r.ColumnName,
					DataType:    r.DataType,
					IsNullable:  r.IsNullable,
					Position:    r.Position,
				}
			}

			logger.LogDatabaseOperation("FIND_COLUMN", fmt.Sprintf("search for column: %s", input.ColumnName), int64(len(cols)), nil)

			output := FindColumnOutput{Columns: cols}
			message := fmt.Sprintf("Found %d matching %s for column '%s'",
				len(cols), pluralize(len(cols), "column", "columns"), input.ColumnName)

			return textResult(message), output, nil
		},
	)
}
