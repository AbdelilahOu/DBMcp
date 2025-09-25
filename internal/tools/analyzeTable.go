package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func analyzeTableHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.AnalyzeTableInput, dbClient *client.DBClient) (*mcp.CallToolResult, mcpdb.AnalyzeTableOutput, error) {

	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil || sessionState.Conn == nil {
		return nil, mcpdb.AnalyzeTableOutput{}, fmt.Errorf("no active DB connection in session")
	}

	schema := input.Schema
	if schema == "" {
		schema = "public"
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stats, err := getTableStatistics(ctx, sessionState.Conn, input.TableName, schema)
	if err != nil {
		return nil, mcpdb.AnalyzeTableOutput{}, fmt.Errorf("failed to analyze table: %v", err)
	}

	output := mcpdb.AnalyzeTableOutput{
		Stats: *stats,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, mcpdb.AnalyzeTableOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func getTableStatistics(ctx context.Context, conn *sql.DB, tableName, schema string) (*mcpdb.TableStats, error) {
	stats := &mcpdb.TableStats{
		TableName:   tableName,
		ColumnStats: make(map[string]string),
	}

	pgRowCountQuery := "SELECT COUNT(*) FROM \"" + schema + "\".\"" + tableName + "\""
	pgSizeQuery := `
		SELECT
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
			pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) as index_size
		FROM pg_tables
		WHERE schemaname = $1 AND tablename = $2`

	pgStatsQuery := `
		SELECT
			last_analyze,
			last_autoanalyze
		FROM pg_stat_user_tables
		WHERE schemaname = $1 AND relname = $2`

	var err error

	err = conn.QueryRowContext(ctx, pgRowCountQuery).Scan(&stats.RowCount)
	if err != nil {

		mysqlRowCountQuery := "SELECT COUNT(*) FROM `" + schema + "`.`" + tableName + "`"
		mysqlSizeQuery := `
			SELECT
				ROUND(((data_length + index_length) / 1024 / 1024), 2) AS total_size_mb,
				ROUND((data_length / 1024 / 1024), 2) AS table_size_mb,
				ROUND((index_length / 1024 / 1024), 2) AS index_size_mb
			FROM information_schema.tables
			WHERE table_schema = ? AND table_name = ?`

		err = conn.QueryRowContext(ctx, mysqlRowCountQuery).Scan(&stats.RowCount)
		if err != nil {
			return nil, fmt.Errorf("failed to get row count: %v", err)
		}

		var totalSizeMB, tableSizeMB, indexSizeMB sql.NullFloat64
		err = conn.QueryRowContext(ctx, mysqlSizeQuery, schema, tableName).Scan(&totalSizeMB, &tableSizeMB, &indexSizeMB)
		if err != nil {
			return nil, fmt.Errorf("failed to get size information: %v", err)
		}

		if totalSizeMB.Valid {
			stats.TotalSize = fmt.Sprintf("%.2f MB", totalSizeMB.Float64)
		} else {
			stats.TotalSize = "N/A"
		}

		if tableSizeMB.Valid {
			stats.TableSize = fmt.Sprintf("%.2f MB", tableSizeMB.Float64)
		} else {
			stats.TableSize = "N/A"
		}

		if indexSizeMB.Valid {
			stats.IndexSize = fmt.Sprintf("%.2f MB", indexSizeMB.Float64)
		} else {
			stats.IndexSize = "N/A"
		}

		stats.LastAnalyzed = "N/A (MySQL)"

		columnStatsQuery := `
			SELECT
				COLUMN_NAME,
				CASE
					WHEN IS_NULLABLE = 'YES' THEN 'Nullable'
					ELSE 'Not Null'
				END as nullability
			FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ?
			LIMIT 5`

		rows, err := conn.QueryContext(ctx, columnStatsQuery, schema, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var colName, nullability string
				if err := rows.Scan(&colName, &nullability); err == nil {
					stats.ColumnStats[colName] = nullability
				}
			}
		}

	} else {

		err = conn.QueryRowContext(ctx, pgSizeQuery, schema, tableName).Scan(&stats.TotalSize, &stats.TableSize, &stats.IndexSize)
		if err != nil {
			stats.TotalSize = "N/A"
			stats.TableSize = "N/A"
			stats.IndexSize = "N/A"
		}

		var lastAnalyze, lastAutoAnalyze sql.NullTime
		err = conn.QueryRowContext(ctx, pgStatsQuery, schema, tableName).Scan(&lastAnalyze, &lastAutoAnalyze)
		if err == nil {
			if lastAnalyze.Valid {
				stats.LastAnalyzed = lastAnalyze.Time.Format("2006-01-02 15:04:05")
			} else if lastAutoAnalyze.Valid {
				stats.LastAnalyzed = lastAutoAnalyze.Time.Format("2006-01-02 15:04:05") + " (auto)"
			} else {
				stats.LastAnalyzed = "Never"
			}
		} else {
			stats.LastAnalyzed = "N/A"
		}

		columnStatsQuery := `
			SELECT
				attname as column_name,
				CASE
					WHEN attnotnull THEN 'Not Null'
					ELSE 'Nullable'
				END as nullability
			FROM pg_attribute a
			JOIN pg_class t ON a.attrelid = t.oid
			JOIN pg_namespace n ON t.relnamespace = n.oid
			WHERE n.nspname = $1 AND t.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
			LIMIT 5`

		rows, err := conn.QueryContext(ctx, columnStatsQuery, schema, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var colName, nullability string
				if err := rows.Scan(&colName, &nullability); err == nil {
					stats.ColumnStats[colName] = nullability
				}
			}
		}
	}

	return stats, nil
}
