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

type ListSequencesInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter sequences"`
}

type SequenceInfo struct {
	Name   string `json:"name" jsonschema_description:"Sequence name"`
	Schema string `json:"sch" jsonschema_description:"Schema name"`
}

type ListSequencesOutput struct {
	Sequences []SequenceInfo `json:"sequences" jsonschema_description:"Array of sequence information"`
}

type GetSequenceInfoInput struct {
	SequenceName string `json:"seq" jsonschema:"required" jsonschema_description:"Name of the sequence"`
	Schema       string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type GetSequenceInfoOutput struct {
	SequenceName string `json:"seq" jsonschema_description:"Name of the sequence"`
	Schema       string `json:"sch" jsonschema_description:"Schema of the sequence"`
	LastValue    int64  `json:"last_value" jsonschema_description:"Last value returned by the sequence"`
	StartValue   int64  `json:"start_value" jsonschema_description:"Start value of the sequence"`
	IncrementBy  int64  `json:"increment_by" jsonschema_description:"Increment value"`
	MinValue     *int64 `json:"min_value,omitempty" jsonschema_description:"Minimum value (null if no minimum)"`
	MaxValue     *int64 `json:"max_value,omitempty" jsonschema_description:"Maximum value (null if no maximum)"`
	CacheSize    int64  `json:"cache_size" jsonschema_description:"Number of sequence values to cache"`
	Cycle        bool   `json:"cycle,omitempty" jsonschema_description:"Whether the sequence cycles when reaching max/min"`
}

func GetListSequencesTool() *ToolDefinition[ListSequencesInput, ListSequencesOutput] {
	return NewToolDefinition[ListSequencesInput, ListSequencesOutput](
		"list_sequences",
		"List sequences (PostgreSQL only). For auto-increment values. Returns names, schemas. Use get_sequence_info for details. Note: MySQL uses AUTO_INCREMENT.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListSequencesInput) (*mcp.CallToolResult, ListSequencesOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListSequencesOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListSequencesOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			if sessionState.DBType == "mysql" {
				return nil, ListSequencesOutput{}, fmt.Errorf("MySQL does not support sequences. MySQL uses AUTO_INCREMENT for auto-incrementing columns. Use describe_table to see AUTO_INCREMENT columns")
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			var query string

			if input.Schema != "" {
				query = `
					SELECT
						sequencename as name,
						schemaname as schema_name
					FROM pg_sequences
					WHERE schemaname = $1
					ORDER BY sequencename`
			} else {
				query = `
					SELECT
						sequencename as name,
						schemaname as schema_name
					FROM pg_sequences
					WHERE schemaname NOT IN ('information_schema', 'pg_catalog')
					ORDER BY sequencename`
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var rows *sql.Rows
			if input.Schema != "" {
				rows, err = sessionState.Conn.QueryContext(ctx, query, schema)
			} else {
				rows, err = sessionState.Conn.QueryContext(ctx, query)
			}

			if err != nil {
				logger.LogDatabaseOperation("LIST_SEQUENCES", query, 0, err)
				return nil, ListSequencesOutput{}, fmt.Errorf("query error: %v", err)
			}
			defer rows.Close()

			var sequences []SequenceInfo
			for rows.Next() {
				var name, schemaName string
				if err := rows.Scan(&name, &schemaName); err != nil {
					return nil, ListSequencesOutput{}, fmt.Errorf("scan error: %v", err)
				}

				sequences = append(sequences, SequenceInfo{
					Name:   name,
					Schema: schemaName,
				})
			}

			if err = rows.Err(); err != nil {
				return nil, ListSequencesOutput{}, fmt.Errorf("rows iteration error: %v", err)
			}

			logger.LogDatabaseOperation("LIST_SEQUENCES", query, int64(len(sequences)), nil)

			output := ListSequencesOutput{Sequences: sequences}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListSequencesOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func GetSequenceInfoTool() *ToolDefinition[GetSequenceInfoInput, GetSequenceInfoOutput] {
	return NewToolDefinition[GetSequenceInfoInput, GetSequenceInfoOutput](
		"get_sequence_info",
		"Get sequence details (PostgreSQL only). Returns current/start value, increment, min/max, cache, cycle. For troubleshooting auto-increment.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetSequenceInfoInput) (*mcp.CallToolResult, GetSequenceInfoOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetSequenceInfoOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetSequenceInfoOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			if sessionState.DBType == "mysql" {
				return nil, GetSequenceInfoOutput{}, fmt.Errorf("MySQL does not support sequences. MySQL uses AUTO_INCREMENT for auto-incrementing columns")
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			query := `
				SELECT
					last_value,
					start_value,
					increment_by,
					min_value,
					max_value,
					cache_size,
					cycle
				FROM pg_sequences
				WHERE sequencename = $1 AND schemaname = $2`

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var lastValue, startValue, incrementBy, cacheSize int64
			var minValue, maxValue sql.NullInt64
			var cycle bool

			err = sessionState.Conn.QueryRowContext(ctx, query, input.SequenceName, schema).Scan(
				&lastValue,
				&startValue,
				&incrementBy,
				&minValue,
				&maxValue,
				&cacheSize,
				&cycle,
			)

			if err == sql.ErrNoRows {
				return nil, GetSequenceInfoOutput{}, fmt.Errorf("sequence '%s' not found in schema '%s'", input.SequenceName, schema)
			}
			if err != nil {
				logger.LogDatabaseOperation("GET_SEQUENCE_INFO", query, 0, err)
				return nil, GetSequenceInfoOutput{}, fmt.Errorf("query error: %v", err)
			}

			output := GetSequenceInfoOutput{
				SequenceName: input.SequenceName,
				Schema:       schema,
				LastValue:    lastValue,
				StartValue:   startValue,
				IncrementBy:  incrementBy,
				CacheSize:    cacheSize,
				Cycle:        cycle,
			}

			if minValue.Valid {
				val := minValue.Int64
				output.MinValue = &val
			}
			if maxValue.Valid {
				val := maxValue.Int64
				output.MaxValue = &val
			}

			logger.LogDatabaseOperation("GET_SEQUENCE_INFO", query, 1, nil)

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, GetSequenceInfoOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}
