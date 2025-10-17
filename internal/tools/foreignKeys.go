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

type ListForeignKeysInput struct {
	TableName string `json:"table_name,omitempty" jsonschema_description:"Optional table name to filter foreign keys"`
	Schema    string `json:"schema,omitempty" jsonschema_description:"Optional schema name"`
}

type ForeignKeyInfo struct {
	ConstraintName    string   `json:"constraint_name" jsonschema_description:"Name of the foreign key constraint"`
	TableName         string   `json:"table_name" jsonschema_description:"Table containing the foreign key"`
	TableSchema       string   `json:"table_schema" jsonschema_description:"Schema of the table"`
	Columns           []string `json:"columns" jsonschema_description:"Columns in the foreign key"`
	ReferencedTable   string   `json:"referenced_table" jsonschema_description:"Table being referenced"`
	ReferencedSchema  string   `json:"referenced_schema" jsonschema_description:"Schema of the referenced table"`
	ReferencedColumns []string `json:"referenced_columns" jsonschema_description:"Columns being referenced"`
	OnDelete          string   `json:"on_delete" jsonschema_description:"Action on delete (CASCADE, SET NULL, etc.)"`
	OnUpdate          string   `json:"on_update" jsonschema_description:"Action on update (CASCADE, SET NULL, etc.)"`
}

type ListForeignKeysOutput struct {
	ForeignKeys []ForeignKeyInfo `json:"foreign_keys" jsonschema_description:"Array of foreign key information"`
}

type GetTableRelationshipsInput struct {
	TableName string `json:"table_name" jsonschema:"required" jsonschema_description:"Name of the table"`
	Schema    string `json:"schema,omitempty" jsonschema_description:"Optional schema name"`
}

type RelationshipInfo struct {
	Type           string   `json:"type" jsonschema_description:"Relationship type: 'outgoing' (references another table) or 'incoming' (referenced by another table)"`
	ConstraintName string   `json:"constraint_name" jsonschema_description:"Name of the foreign key constraint"`
	RelatedTable   string   `json:"related_table" jsonschema_description:"The other table in the relationship"`
	RelatedSchema  string   `json:"related_schema" jsonschema_description:"Schema of the related table"`
	Columns        []string `json:"columns" jsonschema_description:"Columns involved in this table"`
	RelatedColumns []string `json:"related_columns" jsonschema_description:"Columns in the related table"`
	OnDelete       string   `json:"on_delete" jsonschema_description:"Action on delete"`
	OnUpdate       string   `json:"on_update" jsonschema_description:"Action on update"`
}

type GetTableRelationshipsOutput struct {
	TableName     string             `json:"table_name" jsonschema_description:"Name of the table"`
	TableSchema   string             `json:"table_schema" jsonschema_description:"Schema of the table"`
	Relationships []RelationshipInfo `json:"relationships" jsonschema_description:"Array of relationships (both incoming and outgoing)"`
}

func GetListForeignKeysTool() *ToolDefinition[ListForeignKeysInput, ListForeignKeysOutput] {
	return NewToolDefinition[ListForeignKeysInput, ListForeignKeysOutput](
		"list_foreign_keys",
		"List all foreign key constraints in the database or for a specific table. Returns detailed information about foreign key relationships including columns, referenced tables, and referential actions (CASCADE, SET NULL, etc.). Use this to understand table relationships and referential integrity constraints.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListForeignKeysInput) (*mcp.CallToolResult, ListForeignKeysOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListForeignKeysOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListForeignKeysOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var foreignKeys []ForeignKeyInfo
			var err2 error

			if sessionState.DBType == "postgres" {
				foreignKeys, err2 = getPostgresForeignKeys(ctx, sessionState.Conn, input.TableName, schema)
			} else {
				foreignKeys, err2 = getMySQLForeignKeys(ctx, sessionState.Conn, input.TableName, schema)
			}

			if err2 != nil {
				logger.LogDatabaseOperation("LIST_FOREIGN_KEYS", "list foreign keys", 0, err2)
				return nil, ListForeignKeysOutput{}, err2
			}

			logger.LogDatabaseOperation("LIST_FOREIGN_KEYS", "list foreign keys", int64(len(foreignKeys)), nil)

			output := ListForeignKeysOutput{ForeignKeys: foreignKeys}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListForeignKeysOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func GetTableRelationshipsTool() *ToolDefinition[GetTableRelationshipsInput, GetTableRelationshipsOutput] {
	return NewToolDefinition[GetTableRelationshipsInput, GetTableRelationshipsOutput](
		"get_table_relationships",
		"Get all relationships for a specific table, including both outgoing foreign keys (tables this table references) and incoming foreign keys (tables that reference this table). Provides a complete view of how a table relates to other tables in the database schema.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetTableRelationshipsInput) (*mcp.CallToolResult, GetTableRelationshipsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetTableRelationshipsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetTableRelationshipsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var relationships []RelationshipInfo
			var err2 error

			if sessionState.DBType == "postgres" {
				relationships, err2 = getPostgresRelationships(ctx, sessionState.Conn, input.TableName, schema)
			} else {
				relationships, err2 = getMySQLRelationships(ctx, sessionState.Conn, input.TableName, schema)
			}

			if err2 != nil {
				logger.LogDatabaseOperation("GET_TABLE_RELATIONSHIPS", fmt.Sprintf("get relationships for %s", input.TableName), 0, err2)
				return nil, GetTableRelationshipsOutput{}, err2
			}

			logger.LogDatabaseOperation("GET_TABLE_RELATIONSHIPS", fmt.Sprintf("get relationships for %s", input.TableName), int64(len(relationships)), nil)

			output := GetTableRelationshipsOutput{
				TableName:     input.TableName,
				TableSchema:   schema,
				Relationships: relationships,
			}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, GetTableRelationshipsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func getPostgresForeignKeys(ctx context.Context, conn *sql.DB, tableName, schema string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.table_schema,
			tc.table_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			ccu.table_schema AS referenced_schema,
			ccu.table_name AS referenced_table,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position) as referenced_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND tc.table_name = $2"
		args = append(args, tableName)
	}

	query += `
		GROUP BY tc.constraint_name, tc.table_schema, tc.table_name,
				 ccu.table_schema, ccu.table_name, rc.delete_rule, rc.update_rule
		ORDER BY tc.table_name, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		var columnsArray, refColumnsArray string

		err := rows.Scan(
			&fk.ConstraintName,
			&fk.TableSchema,
			&fk.TableName,
			&columnsArray,
			&fk.ReferencedSchema,
			&fk.ReferencedTable,
			&refColumnsArray,
			&fk.OnDelete,
			&fk.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		fk.Columns = parsePostgresArray(columnsArray)
		fk.ReferencedColumns = parsePostgresArray(refColumnsArray)

		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

func getMySQLForeignKeys(ctx context.Context, conn *sql.DB, tableName, schema string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			kcu.constraint_name,
			kcu.table_schema,
			kcu.table_name,
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			kcu.referenced_table_schema,
			kcu.referenced_table_name,
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position) as referenced_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON kcu.constraint_name = rc.constraint_name
			AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name IS NOT NULL
			AND kcu.table_schema = ?`

	var args []interface{}
	args = append(args, schema)

	if tableName != "" {
		query += " AND kcu.table_name = ?"
		args = append(args, tableName)
	}

	query += `
		GROUP BY kcu.constraint_name, kcu.table_schema, kcu.table_name,
				 kcu.referenced_table_schema, kcu.referenced_table_name,
				 rc.delete_rule, rc.update_rule
		ORDER BY kcu.table_name, kcu.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		var columnsStr, refColumnsStr string

		err := rows.Scan(
			&fk.ConstraintName,
			&fk.TableSchema,
			&fk.TableName,
			&columnsStr,
			&fk.ReferencedSchema,
			&fk.ReferencedTable,
			&refColumnsStr,
			&fk.OnDelete,
			&fk.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		fk.Columns = parseMySQLArray(columnsStr)
		fk.ReferencedColumns = parseMySQLArray(refColumnsStr)

		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

func getPostgresRelationships(ctx context.Context, conn *sql.DB, tableName, schema string) ([]RelationshipInfo, error) {
	outgoingQuery := `
		SELECT
			'outgoing' as type,
			tc.constraint_name,
			ccu.table_schema AS related_schema,
			ccu.table_name AS related_table,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position) as related_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND tc.table_schema = $2
		GROUP BY tc.constraint_name, ccu.table_schema, ccu.table_name,
				 rc.delete_rule, rc.update_rule`

	incomingQuery := `
		SELECT
			'incoming' as type,
			tc.constraint_name,
			tc.table_schema AS related_schema,
			tc.table_name AS related_table,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position) as columns,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position) as related_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND ccu.table_name = $1
			AND ccu.table_schema = $2
		GROUP BY tc.constraint_name, tc.table_schema, tc.table_name,
				 rc.delete_rule, rc.update_rule`

	var relationships []RelationshipInfo

	rows, err := conn.QueryContext(ctx, outgoingQuery, tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("outgoing query error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rel RelationshipInfo
		var columnsArray, relColumnsArray string

		err := rows.Scan(
			&rel.Type,
			&rel.ConstraintName,
			&rel.RelatedSchema,
			&rel.RelatedTable,
			&columnsArray,
			&relColumnsArray,
			&rel.OnDelete,
			&rel.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		rel.Columns = parsePostgresArray(columnsArray)
		rel.RelatedColumns = parsePostgresArray(relColumnsArray)

		relationships = append(relationships, rel)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := conn.QueryContext(ctx, incomingQuery, tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("incoming query error: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var rel RelationshipInfo
		var columnsArray, relColumnsArray string

		err := rows2.Scan(
			&rel.Type,
			&rel.ConstraintName,
			&rel.RelatedSchema,
			&rel.RelatedTable,
			&columnsArray,
			&relColumnsArray,
			&rel.OnDelete,
			&rel.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		rel.Columns = parsePostgresArray(columnsArray)
		rel.RelatedColumns = parsePostgresArray(relColumnsArray)

		relationships = append(relationships, rel)
	}

	return relationships, rows2.Err()
}

func parsePostgresArray(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
	s = s[1 : len(s)-1]
	var result []string
	for _, item := range splitByComma(s) {
		result = append(result, item)
	}
	return result
}

func parseMySQLArray(s string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	for _, item := range splitByComma(s) {
		result = append(result, item)
	}
	return result
}

func splitByComma(s string) []string {
	var result []string
	var current string
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func getMySQLRelationships(ctx context.Context, conn *sql.DB, tableName, schema string) ([]RelationshipInfo, error) {
	outgoingQuery := `
		SELECT
			'outgoing' as type,
			kcu.constraint_name,
			kcu.referenced_table_schema,
			kcu.referenced_table_name,
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position) as related_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON kcu.constraint_name = rc.constraint_name
			AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name IS NOT NULL
			AND kcu.table_name = ?
			AND kcu.table_schema = ?
		GROUP BY kcu.constraint_name, kcu.referenced_table_schema, kcu.referenced_table_name,
				 rc.delete_rule, rc.update_rule`

	incomingQuery := `
		SELECT
			'incoming' as type,
			kcu.constraint_name,
			kcu.table_schema,
			kcu.table_name,
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position) as columns,
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position) as related_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON kcu.constraint_name = rc.constraint_name
			AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name = ?
			AND kcu.referenced_table_schema = ?
		GROUP BY kcu.constraint_name, kcu.table_schema, kcu.table_name,
				 rc.delete_rule, rc.update_rule`

	var relationships []RelationshipInfo

	rows, err := conn.QueryContext(ctx, outgoingQuery, tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("outgoing query error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rel RelationshipInfo
		var columnsStr, relColumnsStr string

		err := rows.Scan(
			&rel.Type,
			&rel.ConstraintName,
			&rel.RelatedSchema,
			&rel.RelatedTable,
			&columnsStr,
			&relColumnsStr,
			&rel.OnDelete,
			&rel.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		rel.Columns = parseMySQLArray(columnsStr)
		rel.RelatedColumns = parseMySQLArray(relColumnsStr)

		relationships = append(relationships, rel)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := conn.QueryContext(ctx, incomingQuery, tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("incoming query error: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var rel RelationshipInfo
		var columnsStr, relColumnsStr string

		err := rows2.Scan(
			&rel.Type,
			&rel.ConstraintName,
			&rel.RelatedSchema,
			&rel.RelatedTable,
			&columnsStr,
			&relColumnsStr,
			&rel.OnDelete,
			&rel.OnUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		rel.Columns = parseMySQLArray(columnsStr)
		rel.RelatedColumns = parseMySQLArray(relColumnsStr)

		relationships = append(relationships, rel)
	}

	return relationships, rows2.Err()
}
