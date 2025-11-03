package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListConstraintsInput struct {
	TableName string `json:"tbl,omitempty" jsonschema_description:"Optional table name to filter constraints"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type ConstraintInfo struct {
	ConstraintName   string   `json:"cname" jsonschema_description:"Name of the constraint"`
	ConstraintType   string   `json:"constraint_type" jsonschema_description:"Type of constraint (PRIMARY KEY, FOREIGN KEY, UNIQUE, CHECK)"`
	TableName        string   `json:"tbl" jsonschema_description:"Table the constraint is on"`
	TableSchema      string   `json:"sch" jsonschema_description:"Schema of the table"`
	Columns          []string `json:"columns" jsonschema_description:"Columns involved in the constraint"`
	CheckClause      string   `json:"check_clause,omitempty" jsonschema_description:"CHECK constraint expression (if applicable)"`
	ReferencedTable  string   `json:"referenced_table,omitempty" jsonschema_description:"Referenced table (for foreign keys)"`
	ReferencedSchema string   `json:"referenced_schema,omitempty" jsonschema_description:"Referenced schema (for foreign keys)"`
}

type ListConstraintsOutput struct {
	Constraints []ConstraintInfo `json:"constraints" jsonschema_description:"Array of constraint information"`
}

func GetListConstraintsTool() *ToolDefinition[ListConstraintsInput, ListConstraintsOutput] {
	return NewToolDefinition[ListConstraintsInput, ListConstraintsOutput](
		"list_constraints",
		"List constraints in DB or table. Returns PKs, FKs, unique, check constraints. Shows data integrity rules. Use list_foreign_keys for detailed FK info.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListConstraintsInput) (*mcp.CallToolResult, ListConstraintsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListConstraintsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListConstraintsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			var constraints []ConstraintInfo
			var err2 error

			if sessionState.DBType == "postgres" {
				constraints, err2 = getPostgresConstraints(ctx, sessionState.Conn, input.TableName, schema)
			} else {
				constraints, err2 = getMySQLConstraints(ctx, sessionState.Conn, input.TableName, schema)
			}

			if err2 != nil {
				logger.LogDatabaseOperation("LIST_CONSTRAINTS", "list constraints", 0, err2)
				return nil, ListConstraintsOutput{}, err2
			}

			logger.LogDatabaseOperation("LIST_CONSTRAINTS", "list constraints", int64(len(constraints)), nil)

			output := ListConstraintsOutput{Constraints: constraints}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListConstraintsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func getPostgresConstraints(ctx context.Context, conn *sql.DB, tableName, schema string) ([]ConstraintInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			tc.table_schema,
			tc.table_name,
			array_agg(DISTINCT kcu.column_name ORDER BY kcu.column_name) as columns,
			COALESCE(cc.check_clause, '') as check_clause,
			COALESCE(ccu.table_schema, '') as referenced_schema,
			COALESCE(ccu.table_name, '') as referenced_table
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.check_constraints cc
			ON tc.constraint_name = cc.constraint_name
			AND tc.constraint_schema = cc.constraint_schema
		LEFT JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.table_schema = $1`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND tc.table_name = $2"
		args = append(args, tableName)
	}

	query += `
		GROUP BY tc.constraint_name, tc.constraint_type, tc.table_schema,
				 tc.table_name, cc.check_clause, ccu.table_schema, ccu.table_name
		ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var constraints []ConstraintInfo
	for rows.Next() {
		var constraint ConstraintInfo
		var columnsArray string

		err := rows.Scan(
			&constraint.ConstraintName,
			&constraint.ConstraintType,
			&constraint.TableSchema,
			&constraint.TableName,
			&columnsArray,
			&constraint.CheckClause,
			&constraint.ReferencedSchema,
			&constraint.ReferencedTable,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		constraint.Columns = parsePostgresArray(columnsArray)

		constraints = append(constraints, constraint)
	}

	return constraints, rows.Err()
}

func getMySQLConstraints(ctx context.Context, conn *sql.DB, tableName, schema string) ([]ConstraintInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			tc.table_schema,
			tc.table_name,
			GROUP_CONCAT(DISTINCT kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			COALESCE(cc.check_clause, '') as check_clause,
			COALESCE(kcu.referenced_table_schema, '') as referenced_schema,
			COALESCE(kcu.referenced_table_name, '') as referenced_table
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
		LEFT JOIN information_schema.check_constraints cc
			ON tc.constraint_name = cc.constraint_name
			AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.table_schema = ?`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND tc.table_name = ?"
		args = append(args, tableName)
	}

	query += `
		GROUP BY tc.constraint_name, tc.constraint_type, tc.table_schema,
				 tc.table_name, cc.check_clause, kcu.referenced_table_schema,
				 kcu.referenced_table_name
		ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var constraints []ConstraintInfo
	for rows.Next() {
		var constraint ConstraintInfo
		var columnsStr sql.NullString

		err := rows.Scan(
			&constraint.ConstraintName,
			&constraint.ConstraintType,
			&constraint.TableSchema,
			&constraint.TableName,
			&columnsStr,
			&constraint.CheckClause,
			&constraint.ReferencedSchema,
			&constraint.ReferencedTable,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		if columnsStr.Valid {
			constraint.Columns = parseMySQLArray(columnsStr.String)
		}

		constraints = append(constraints, constraint)
	}

	return constraints, rows.Err()
}
