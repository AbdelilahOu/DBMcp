package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetDBInfoInput struct{}

type GetDBInfoOutput struct {
	DatabaseName string   `json:"database_name" jsonschema_description:"Name of the database"`
	Version      string   `json:"version" jsonschema_description:"Database version"`
	Schemas      []string `json:"schemas" jsonschema_description:"Available schemas"`
	TableCount   int      `json:"table_count" jsonschema_description:"Total number of tables"`
}

func GetDbInfoTool() *ToolDefinition[GetDBInfoInput, GetDBInfoOutput] {
	return NewToolDefinition[GetDBInfoInput, GetDBInfoOutput](
		"get_db_info",
		"Get general database information and statistics.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetDBInfoInput) (*mcp.CallToolResult, GetDBInfoOutput, error) {
			return getDBInfoHandler(ctx, req, input)
		},
	)
}

func getDBInfoHandler(ctx context.Context, req *mcp.CallToolRequest, input GetDBInfoInput) (*mcp.CallToolResult, GetDBInfoOutput, error) {
	sessionState, err := getActiveSession("default")
	if err != nil {
		return nil, GetDBInfoOutput{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var dbName, version string
	var schemas []string
	var tableCount int

	pgDbNameQuery := "SELECT current_database()"
	pgVersionQuery := "SELECT version()"
	pgSchemasQuery := "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')"
	pgTableCountQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast')"

	err = sessionState.Conn.QueryRowContext(ctx, pgDbNameQuery).Scan(&dbName)
	if err != nil {
		mysqlDbNameQuery := "SELECT DATABASE()"
		mysqlVersionQuery := "SELECT VERSION()"
		mysqlSchemasQuery := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')"
		mysqlTableCountQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')"

		err = sessionState.Conn.QueryRowContext(ctx, mysqlDbNameQuery).Scan(&dbName)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get database name: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, mysqlVersionQuery).Scan(&version)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get version: %v", err)
		}

		schemas, err = getStringSliceFromQuery(ctx, sessionState.Conn, mysqlSchemasQuery)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get schemas: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, mysqlTableCountQuery).Scan(&tableCount)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get table count: %v", err)
		}

		version = "MySQL " + version
	} else {
		err = sessionState.Conn.QueryRowContext(ctx, pgVersionQuery).Scan(&version)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get version: %v", err)
		}

		if strings.Contains(version, "PostgreSQL") {
			parts := strings.Fields(version)
			if len(parts) >= 2 {
				version = "PostgreSQL " + parts[1]
			}
		}

		schemas, err = getStringSliceFromQuery(ctx, sessionState.Conn, pgSchemasQuery)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get schemas: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, pgTableCountQuery).Scan(&tableCount)
		if err != nil {
			return nil, GetDBInfoOutput{}, fmt.Errorf("failed to get table count: %v", err)
		}
	}

	output := GetDBInfoOutput{
		DatabaseName: dbName,
		Version:      version,
		Schemas:      schemas,
		TableCount:   tableCount,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, GetDBInfoOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func getStringSliceFromQuery(ctx context.Context, conn *sql.DB, query string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, rows.Err()
}
