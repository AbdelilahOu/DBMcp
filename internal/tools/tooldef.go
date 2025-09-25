package tools

import (
	"context"

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
	mcp.AddTool(s, td.Tool, td.Handler)
}
