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
	DBUrl             string
	ReadOnly          bool
	InitialConnection string // Optional: name of connection to initialize at startup
}

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	impl := &mcp.Implementation{Name: "db-mcp-server", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	// Initialize connection if specified
	if cfg.InitialConnection != "" {
		err := initializeConnection(cfg.InitialConnection)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize connection '%s': %w", cfg.InitialConnection, err)
		}
		fmt.Printf("Successfully initialized connection: %s\n", cfg.InitialConnection)
	}

	// Register tools without requiring an active DB connection at startup
	tools.RegisterTools(server, cfg.ReadOnly)

	return server, nil
}

type StdioServerConfig struct {
	Version           string
	ReadOnly          bool
	InitialConnection string // Optional: name of connection to initialize at startup
}

// initializeConnection creates a database connection and sets it in the session state
func initializeConnection(connectionName string) error {
	if tools.GlobalConfig == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		tools.GlobalConfig = cfg
	}

	conn, exists := tools.GlobalConfig.GetConnection(connectionName)
	if !exists {
		return fmt.Errorf("connection '%s' not found in config", connectionName)
	}

	dbClient, err := client.NewDBClient(conn.URL)
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
		ReadOnly:          cfg.ReadOnly,
		InitialConnection: cfg.InitialConnection,
	})

	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	fmt.Printf("DB MCP Server running (read-only: %t)...\n", cfg.ReadOnly)

	return server.Run(ctx, &mcp.StdioTransport{})
}
