package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdelilahOu/DBMcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerConfig struct {
	Version  string
	DBUrl    string
	ReadOnly bool
}

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	impl := &mcp.Implementation{Name: "db-mcp-server", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	// Register tools without requiring an active DB connection at startup
	tools.RegisterTools(server, cfg.ReadOnly)

	return server, nil
}

type StdioServerConfig struct {
	Version  string
	ReadOnly bool
}

func RunStdioServer(cfg StdioServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := NewMCPServer(MCPServerConfig{
		Version:  cfg.Version,
		ReadOnly: cfg.ReadOnly,
	})

	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	fmt.Printf("DB MCP Server running (read-only: %t, tools: %v)...\n", cfg.ReadOnly)

	return server.Run(ctx, &mcp.StdioTransport{})
}
