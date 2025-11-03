package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, FindColumnOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, FindColumnOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			var columns []ColumnLocation
			var err2 error

			if sessionState.DBType == "postgres" {
				columns, err2 = findPostgresColumns(ctx, sessionState.Conn, input, schema)
			} else {
				columns, err2 = findMySQLColumns(ctx, sessionState.Conn, input, schema)
			}

			if err2 != nil {
				logger.LogDatabaseOperation("FIND_COLUMN", fmt.Sprintf("search for column: %s", input.ColumnName), 0, err2)
				return nil, FindColumnOutput{}, err2
			}

			logger.LogDatabaseOperation("FIND_COLUMN", fmt.Sprintf("search for column: %s", input.ColumnName), int64(len(columns)), nil)

			output := FindColumnOutput{Columns: columns}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, FindColumnOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func findPostgresColumns(ctx context.Context, conn *sql.DB, input FindColumnInput, schema string) ([]ColumnLocation, error) {
	query := `
		SELECT
			table_name,
			table_schema,
			column_name,
			data_type,
			CASE WHEN is_nullable = 'YES' THEN true ELSE false END as is_nullable,
			ordinal_position
		FROM information_schema.columns
		WHERE table_schema NOT IN ('information_schema', 'pg_catalog')`

	var args []interface{}
	argCount := 0

	if input.Schema != "" {
		argCount++
		query += fmt.Sprintf(" AND table_schema = $%d", argCount)
		args = append(args, schema)
	}

	if input.TableName != "" {
		argCount++
		query += fmt.Sprintf(" AND table_name = $%d", argCount)
		args = append(args, input.TableName)
	}

	if input.ExactMatch {
		argCount++
		query += fmt.Sprintf(" AND column_name = $%d", argCount)
		args = append(args, input.ColumnName)
	} else {
		argCount++
		query += fmt.Sprintf(" AND column_name ILIKE $%d", argCount)
		args = append(args, "%"+input.ColumnName+"%")
	}

	query += " ORDER BY table_schema, table_name, ordinal_position"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var columns []ColumnLocation
	for rows.Next() {
		var col ColumnLocation

		err := rows.Scan(
			&col.TableName,
			&col.TableSchema,
			&col.ColumnName,
			&col.DataType,
			&col.IsNullable,
			&col.Position,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func findMySQLColumns(ctx context.Context, conn *sql.DB, input FindColumnInput, schema string) ([]ColumnLocation, error) {
	query := `
		SELECT
			table_name,
			table_schema,
			column_name,
			data_type,
			CASE WHEN is_nullable = 'YES' THEN true ELSE false END as is_nullable,
			ordinal_position
		FROM information_schema.columns
		WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')`

	var args []interface{}

	if input.Schema != "" {
		query += " AND table_schema = ?"
		args = append(args, schema)
	}

	if input.TableName != "" {
		query += " AND table_name = ?"
		args = append(args, input.TableName)
	}

	if input.ExactMatch {
		query += " AND column_name = ?"
		args = append(args, input.ColumnName)
	} else {
		query += " AND column_name LIKE ?"
		args = append(args, "%"+input.ColumnName+"%")
	}

	query += " ORDER BY table_schema, table_name, ordinal_position"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var columns []ColumnLocation
	for rows.Next() {
		var col ColumnLocation

		err := rows.Scan(
			&col.TableName,
			&col.TableSchema,
			&col.ColumnName,
			&col.DataType,
			&col.IsNullable,
			&col.Position,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		if !input.ExactMatch && !strings.EqualFold(col.ColumnName, input.ColumnName) {
			if !strings.Contains(strings.ToLower(col.ColumnName), strings.ToLower(input.ColumnName)) {
				continue
			}
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}
