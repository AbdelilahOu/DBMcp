package tools

import (
	"context"
	"fmt"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolDefinition[TInput, TOutput any] struct {
	Tool    *mcp.Tool
	Handler func(ctx context.Context, req *mcp.CallToolRequest, input TInput) (*mcp.CallToolResult, TOutput, error)
}

func NewToolDefinition[TInput, TOutput any](
	name, description string,
	handler func(ctx context.Context, req *mcp.CallToolRequest, input TInput) (*mcp.CallToolResult, TOutput, error),
) *ToolDefinition[TInput, TOutput] {
	return &ToolDefinition[TInput, TOutput]{
		Tool: &mcp.Tool{
			Name:        name,
			Description: description,
		},
		Handler: handler,
	}
}

func (td *ToolDefinition[TInput, TOutput]) Register(s *mcp.Server) {
	wrappedHandler := func(ctx context.Context, req *mcp.CallToolRequest, input TInput) (*mcp.CallToolResult, TOutput, error) {
		result, output, err := td.Handler(ctx, req, input)

		logger.LogToolCall(td.Tool.Name, err)

		return result, output, err
	}

	mcp.AddTool(s, td.Tool, wrappedHandler)
}

func textResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
	}
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func qualifiedName(schema, name string) string {
	if schema == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", schema, name)
}
