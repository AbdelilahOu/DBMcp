package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AnalyzeTableInput struct {
	TableName string `json:"table_name" jsonschema:"required" jsonschema_description:"Name of the table to analyze"`
	Schema    string `json:"schema,omitempty" jsonschema_description:"Optional schema name"`
}

type TableStats struct {
	TableName    string            `json:"table_name" jsonschema_description:"Table name"`
	RowCount     int64             `json:"row_count" jsonschema_description:"Approximate number of rows"`
	TableSize    string            `json:"table_size" jsonschema_description:"Table size in human-readable format"`
	IndexSize    string            `json:"index_size" jsonschema_description:"Index size in human-readable format"`
	TotalSize    string            `json:"total_size" jsonschema_description:"Total size (table + indexes)"`
	ColumnStats  map[string]string `json:"column_stats,omitempty" jsonschema_description:"Basic statistics for columns"`
	LastAnalyzed string            `json:"last_analyzed,omitempty" jsonschema_description:"When the table was last analyzed"`
}

type AnalyzeTableOutput struct {
	Stats TableStats `json:"stats" jsonschema_description:"Table statistics"`
}

func GetAnalyzeTableTool() *ToolDefinition[AnalyzeTableInput, AnalyzeTableOutput] {
	return NewToolDefinition[AnalyzeTableInput, AnalyzeTableOutput](
		"analyze_table",
		"Get table statistics (row count, size, column stats).",
		func(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeTableInput) (*mcp.CallToolResult, AnalyzeTableOutput, error) {
			return analyzeTableHandler(ctx, req, input)
		},
	)
}

func analyzeTableHandler(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeTableInput) (*mcp.CallToolResult, AnalyzeTableOutput, error) {

	sessionState, err := getActiveSession("default")
	if err != nil {
		return nil, AnalyzeTableOutput{}, err
	}

	schema := input.Schema
	if schema == "" {
		schema = "public"
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stats, err := getTableStatistics(ctx, sessionState.Conn, input.TableName, schema)

	if err != nil {
		logger.LogDatabaseOperation("ANALYZE_TABLE", fmt.Sprintf("ANALYZE %s.%s", schema, input.TableName), 0, err)
		return nil, AnalyzeTableOutput{}, fmt.Errorf("failed to analyze table: %v", err)
	}

	logger.LogDatabaseOperation("ANALYZE_TABLE", fmt.Sprintf("ANALYZE %s.%s", schema, input.TableName), stats.RowCount, nil)

	output := AnalyzeTableOutput{
		Stats: *stats,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, AnalyzeTableOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func getTableStatistics(ctx context.Context, conn *sql.DB, tableName, schema string) (*TableStats, error) {
	stats := &TableStats{
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
