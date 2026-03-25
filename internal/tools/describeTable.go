package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, DescribeTableOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, DescribeTableOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			columns, err := getTableColumns(ctx, sessionState.Conn, sessionState.DBType, input.TableName, schema)
			if err != nil {
				logger.LogDatabaseOperation("DESCRIBE_TABLE", fmt.Sprintf("DESCRIBE %s.%s", schema, input.TableName), 0, err)
				return nil, DescribeTableOutput{}, fmt.Errorf("get columns error: %v", err)
			}

			indexes, err := getTableIndexes(ctx, sessionState.Conn, sessionState.DBType, input.TableName, schema)
			if err != nil {
				logger.LogDatabaseOperation("DESCRIBE_TABLE", fmt.Sprintf("DESCRIBE %s.%s", schema, input.TableName), 0, err)
				return nil, DescribeTableOutput{}, fmt.Errorf("get indexes error: %v", err)
			}

			logger.LogDatabaseOperation("DESCRIBE_TABLE", fmt.Sprintf("DESCRIBE %s.%s", schema, input.TableName), int64(len(columns)), nil)

			output := DescribeTableOutput{
				Columns: columns,
				Indexes: indexes,
			}
			message := fmt.Sprintf(
				"Schema for %s: %d %s, %d %s",
				qualifiedName(schema, input.TableName),
				len(columns),
				pluralize(len(columns), "column", "columns"),
				len(indexes),
				pluralize(len(indexes), "index", "indexes"),
			)

			return textResult(message), output, nil
		},
	)
}

func getTableColumns(ctx context.Context, conn *sql.DB, dbType, tableName, schema string) ([]ColumnInfo, error) {
	var rows *sql.Rows
	var err error

	if dbType == "postgres" {
		pgQuery := `
			SELECT
				c.column_name,
				c.data_type,
				CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END as is_nullable,
				COALESCE(c.column_default, '') as default_value,
				c.character_maximum_length,
				CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT ku.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage ku
					ON tc.constraint_name = ku.constraint_name
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_name = $1
					AND tc.table_schema = $2
			) pk ON c.column_name = pk.column_name
			WHERE c.table_name = $1 AND c.table_schema = $2
			ORDER BY c.ordinal_position`

		rows, err = conn.QueryContext(ctx, pgQuery, tableName, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to query columns: %v", err)
		}
	} else if dbType == "mysql" {
		mysqlQuery := `
			SELECT
				COLUMN_NAME as column_name,
				DATA_TYPE as data_type,
				CASE WHEN IS_NULLABLE = 'YES' THEN true ELSE false END as is_nullable,
				COALESCE(COLUMN_DEFAULT, '') as default_value,
				CHARACTER_MAXIMUM_LENGTH as character_maximum_length,
				CASE WHEN COLUMN_KEY = 'PRI' THEN true ELSE false END as is_primary_key
			FROM information_schema.columns
			WHERE table_name = ? AND table_schema = ?
			ORDER BY ordinal_position`

		rows, err = conn.QueryContext(ctx, mysqlQuery, tableName, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to query columns: %v", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", dbType)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var charMaxLen sql.NullInt64

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.IsNullable,
			&col.DefaultValue,
			&charMaxLen,
			&col.IsPrimaryKey,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		if charMaxLen.Valid {
			length := int(charMaxLen.Int64)
			col.CharMaxLength = &length
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func getTableIndexes(ctx context.Context, conn *sql.DB, dbType, tableName, schema string) ([]IndexInfo, error) {
	var rows *sql.Rows
	var err error

	if dbType == "postgres" {
		pgQuery := `
			SELECT
				i.relname as index_name,
				array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as columns,
				ix.indisunique as is_unique
			FROM pg_class t
			JOIN pg_index ix ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE t.relname = $1 AND n.nspname = $2
			GROUP BY i.relname, ix.indisunique
			ORDER BY i.relname`

		rows, err = conn.QueryContext(ctx, pgQuery, tableName, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to query indexes: %v", err)
		}
	} else if dbType == "mysql" {
		mysqlQuery := `
			SELECT
				INDEX_NAME as index_name,
				GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) as columns,
				CASE WHEN NON_UNIQUE = 0 THEN true ELSE false END as is_unique
			FROM information_schema.statistics
			WHERE table_name = ? AND table_schema = ?
			GROUP BY index_name, non_unique
			ORDER BY index_name`

		rows, err = conn.QueryContext(ctx, mysqlQuery, tableName, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to query indexes: %v", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", dbType)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var indexName, columnsStr string
		var isUnique bool

		err := rows.Scan(&indexName, &columnsStr, &isUnique)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		var columns []string
		if strings.HasPrefix(columnsStr, "{") && strings.HasSuffix(columnsStr, "}") {
			columnsStr = strings.Trim(columnsStr, "{}")
			columns = strings.Split(columnsStr, ",")
		} else {
			columns = strings.Split(columnsStr, ",")
		}

		for i, col := range columns {
			columns[i] = strings.TrimSpace(col)
		}

		indexes = append(indexes, IndexInfo{
			Name:     indexName,
			Columns:  columns,
			IsUnique: isUnique,
		})
	}

	return indexes, rows.Err()
}
