package tools

import (
	"context"
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
		"List sequences. For auto-increment values. Returns names, schemas. Use get_sequence_info for details. Note: MySQL uses AUTO_INCREMENT.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListSequencesInput) (*mcp.CallToolResult, ListSequencesOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListSequencesOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListSequences(ctx, session.Conn, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_SEQUENCES", "list sequences", 0, err)
				return nil, ListSequencesOutput{}, err
			}

			seqs := make([]SequenceInfo, len(rows))
			for i, r := range rows {
				seqs[i] = SequenceInfo{Name: r.Name, Schema: r.Schema}
			}

			logger.LogDatabaseOperation("LIST_SEQUENCES", "list sequences", int64(len(seqs)), nil)

			output := ListSequencesOutput{Sequences: seqs}
			message := fmt.Sprintf("Found %d %s", len(seqs), pluralize(len(seqs), "sequence", "sequences"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}

func GetSequenceInfoTool() *ToolDefinition[GetSequenceInfoInput, GetSequenceInfoOutput] {
	return NewToolDefinition[GetSequenceInfoInput, GetSequenceInfoOutput](
		"get_sequence_info",
		"Get sequence details. Returns current/start value, increment, min/max, cache, cycle. For troubleshooting auto-increment.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetSequenceInfoInput) (*mcp.CallToolResult, GetSequenceInfoOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetSequenceInfoOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			det, err := session.Driver.GetSequenceInfo(ctx, session.Conn, input.SequenceName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_SEQUENCE_INFO", "get sequence info", 0, err)
				return nil, GetSequenceInfoOutput{}, err
			}

			logger.LogDatabaseOperation("GET_SEQUENCE_INFO", "get sequence info", 1, nil)

			output := GetSequenceInfoOutput{
				SequenceName: det.SequenceName, Schema: det.Schema,
				LastValue: det.LastValue, StartValue: det.StartValue, IncrementBy: det.IncrementBy,
				MinValue: det.MinValue, MaxValue: det.MaxValue,
				CacheSize: det.CacheSize, Cycle: det.Cycle,
			}
			message := fmt.Sprintf("Sequence %s: last=%d, start=%d, increment=%d",
				qualifiedName(output.Schema, output.SequenceName), output.LastValue, output.StartValue, output.IncrementBy)

			return textResult(message), output, nil
		},
	)
}
