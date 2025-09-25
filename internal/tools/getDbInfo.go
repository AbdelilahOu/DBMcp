package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func getDBInfoHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.GetDBInfoInput, dbClient *client.DBClient) (*mcp.CallToolResult, mcpdb.GetDBInfoOutput, error) {
	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil || sessionState.Conn == nil {
		return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("no active DB connection in session")
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

	err := sessionState.Conn.QueryRowContext(ctx, pgDbNameQuery).Scan(&dbName)
	if err != nil {
		mysqlDbNameQuery := "SELECT DATABASE()"
		mysqlVersionQuery := "SELECT VERSION()"
		mysqlSchemasQuery := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')"
		mysqlTableCountQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')"

		err = sessionState.Conn.QueryRowContext(ctx, mysqlDbNameQuery).Scan(&dbName)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get database name: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, mysqlVersionQuery).Scan(&version)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get version: %v", err)
		}

		schemas, err = getStringSliceFromQuery(ctx, sessionState.Conn, mysqlSchemasQuery)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get schemas: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, mysqlTableCountQuery).Scan(&tableCount)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get table count: %v", err)
		}

		version = "MySQL " + version
	} else {
		err = sessionState.Conn.QueryRowContext(ctx, pgVersionQuery).Scan(&version)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get version: %v", err)
		}

		if strings.Contains(version, "PostgreSQL") {
			parts := strings.Fields(version)
			if len(parts) >= 2 {
				version = "PostgreSQL " + parts[1]
			}
		}

		schemas, err = getStringSliceFromQuery(ctx, sessionState.Conn, pgSchemasQuery)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get schemas: %v", err)
		}

		err = sessionState.Conn.QueryRowContext(ctx, pgTableCountQuery).Scan(&tableCount)
		if err != nil {
			return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("failed to get table count: %v", err)
		}
	}

	output := mcpdb.GetDBInfoOutput{
		DatabaseName: dbName,
		Version:      version,
		Schemas:      schemas,
		TableCount:   tableCount,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, mcpdb.GetDBInfoOutput{}, fmt.Errorf("JSON marshal error: %v", err)
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
