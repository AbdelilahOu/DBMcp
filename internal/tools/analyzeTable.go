package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AnalyzeTableInput struct {
	TableName string `json:"tbl" jsonschema:"required" jsonschema_description:"Name of the table to analyze"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type TableStats struct {
	TableName    string            `json:"tbl" jsonschema_description:"Table name"`
	RowCount     int64             `json:"rows" jsonschema_description:"Approximate number of rows"`
	TableSize    string            `json:"tbl_size" jsonschema_description:"Table size in human-readable format"`
	IndexSize    string            `json:"idx_size,omitempty" jsonschema_description:"Index size in human-readable format"`
	TotalSize    string            `json:"total,omitempty" jsonschema_description:"Total size (table + indexes)"`
	ColumnStats  map[string]string `json:"col_stats,omitempty" jsonschema_description:"Basic statistics for columns"`
	LastAnalyzed string            `json:"analyzed,omitempty" jsonschema_description:"When the table was last analyzed"`
}

type AnalyzeTableOutput struct {
	Stats TableStats `json:"stats" jsonschema_description:"Table statistics"`
}

func GetAnalyzeTableTool() *ToolDefinition[AnalyzeTableInput, AnalyzeTableOutput] {
	return NewToolDefinition[AnalyzeTableInput, AnalyzeTableOutput](
		"analyze_table",
		"PREFERRED for table metadata: row count, sizes, column info. Use instead of SELECT COUNT(*). Optimized, no full table scans.",
		func(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeTableInput) (*mcp.CallToolResult, AnalyzeTableOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, AnalyzeTableOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			stats, err := session.Driver.AnalyzeTable(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("ANALYZE_TABLE", fmt.Sprintf("ANALYZE %s.%s", input.Schema, input.TableName), 0, err)
				return nil, AnalyzeTableOutput{}, fmt.Errorf("failed to analyze table: %v", err)
			}

			logger.LogDatabaseOperation("ANALYZE_TABLE", fmt.Sprintf("ANALYZE %s.%s", input.Schema, input.TableName), stats.RowCount, nil)

			output := AnalyzeTableOutput{
				Stats: TableStats{
					TableName:    stats.TableName,
					RowCount:     stats.RowCount,
					TableSize:    stats.TableSize,
					IndexSize:    stats.IndexSize,
					TotalSize:    stats.TotalSize,
					ColumnStats:  stats.ColumnStats,
					LastAnalyzed: stats.LastAnalyzed,
				},
			}

			message := fmt.Sprintf(
				"Stats for %s: %d rows, table size %s, total size %s",
				qualifiedName(input.Schema, input.TableName),
				output.Stats.RowCount, output.Stats.TableSize, output.Stats.TotalSize,
			)

			return textResult(message), output, nil
		},
	)
}
