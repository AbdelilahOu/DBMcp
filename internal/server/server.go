package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/logger"
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
	// Initialize logger first
	logCfg := logger.ConfigFromLoggingConfig(cfg.Config.Logging)
	if err := logger.Initialize(logCfg); err != nil {
		fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
	} else {
		logger.Info("Logger initialized successfully", map[string]interface{}{
			"level":       logger.LogLevelString(logCfg.Level),
			"output_file": logCfg.OutputFile,
			"console":     logCfg.Console,
		})
	}

	impl := &mcp.Implementation{Name: "db-mcp-server", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	logger.Info("MCP Server starting", map[string]interface{}{
		"version": cfg.Version,
	})

	// Initialize connection if specified
	if cfg.InitialConnection != "" {
		conn, exists := cfg.Config.GetConnection(cfg.InitialConnection)
		if !exists {
			err := fmt.Errorf("connection '%s' not found in config", cfg.InitialConnection)
			logger.Error("Initial connection not found", err, map[string]interface{}{
				"connection": cfg.InitialConnection,
			})
			return nil, err
		}
		err := initializeConnection(conn, cfg.InitialConnection)
		if err != nil {
			logger.Error("Failed to initialize connection", err, map[string]interface{}{
				"connection": cfg.InitialConnection,
			})
			return nil, fmt.Errorf("failed to initialize connection '%s': %w", cfg.InitialConnection, err)
		}
		logger.Info("Initial connection established", map[string]interface{}{
			"connection": cfg.InitialConnection,
			"type":       conn.Type,
		})
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

func initializeConnection(conn config.Connection, connectionName string) error {
	dbClient, err := client.NewDBClient(conn.URL, conn.Type)
	if err != nil {
		logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create or update the default session with this connection
	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil {
		err := fmt.Errorf("failed to create session")
		logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, err)
		return err
	}

	// Ensure the connection is properly set in the session
	sessionState.Conn = dbClient.DB

	logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, nil)
	return nil
}

func RunStdioServer(cfg StdioServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ensure logger cleanup on shutdown
	defer func() {
		if err := logger.Shutdown(); err != nil {
			fmt.Printf("Error shutting down logger: %v\n", err)
		}
	}()

	server, err := NewMCPServer(MCPServerConfig{
		Version:           cfg.Version,
		InitialConnection: cfg.InitialConnection,
		Config:            cfg.Config,
	})

	if err != nil {
		logger.Error("Failed to create MCP server", err)
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	logger.Info("DB MCP Server started and running", map[string]interface{}{
		"version": cfg.Version,
	})
	fmt.Printf("DB MCP Server running ...\n")

	err = server.Run(ctx, &mcp.StdioTransport{})
	if err != nil {
		logger.Error("Server stopped with error", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	return err
}
