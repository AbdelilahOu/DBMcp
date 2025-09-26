package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/AbdelilahOu/DBMcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerConfig struct {
	Version           string
	InitialConnection string
	Config            *config.Config
}

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	impl := &mcp.Implementation{Name: "db-mcp-server", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	// Initialize connection if specified
	if cfg.InitialConnection != "" {
		conn, exists := cfg.Config.GetConnection(cfg.InitialConnection)
		if !exists {
			return nil, fmt.Errorf("connection '%s' not found in config", cfg.InitialConnection)
		}
		err := initializeConnection(conn)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize connection '%s': %w", cfg.InitialConnection, err)
		}
		fmt.Printf("Successfully initialized connection: %s\n", cfg.InitialConnection)
	}

	tools.RegisterTools(server, cfg.Config)

	return server, nil
}

type StdioServerConfig struct {
	Version           string
	InitialConnection string
	Config            *config.Config
}

func initializeConnection(conn config.Connection) error {
	dbClient, err := client.NewDBClient(conn.URL, conn.Type)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create or update the default session with this connection
	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil {
		return fmt.Errorf("failed to create session")
	}

	// Ensure the connection is properly set in the session
	sessionState.Conn = dbClient.DB

	return nil
}

func RunStdioServer(cfg StdioServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := NewMCPServer(MCPServerConfig{
		Version:           cfg.Version,
		InitialConnection: cfg.InitialConnection,
		Config:            cfg.Config,
	})

	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	fmt.Printf("DB MCP Server running ...\n")
	return server.Run(ctx, &mcp.StdioTransport{})
}
