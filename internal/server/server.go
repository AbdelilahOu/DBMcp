package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerConfig struct {
	Version  string
	DBUrl    string
	ReadOnly bool
}

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	dbClient, err := client.NewDBClient(cfg.DBUrl)
	if err != nil {
		return nil, fmt.Errorf("init DB client: %w", err)
	}
	defer dbClient.Close()

	impl := &mcp.Implementation{Name: "db-mcp-server", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	tools.RegisterTools(server, dbClient, cfg.ReadOnly)

	return server, nil
}

type StdioServerConfig struct {
	Version  string
	DBUrl    string
	ReadOnly bool
}

func RunStdioServer(cfg StdioServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := NewMCPServer(MCPServerConfig{
		Version:  cfg.Version,
		DBUrl:    cfg.DBUrl,
		ReadOnly: cfg.ReadOnly,
	})

	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	fmt.Printf("DB MCP Server running (read-only: %t, tools: %v)...\n", cfg.ReadOnly)

	return server.Run(ctx, &mcp.StdioTransport{})
}
