package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/driver"
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
	if cfg.Config == nil {
		cfg.Config = &config.Config{
			Connections: make(map[string]config.Connection),
		}
	}

	logCfg := logger.ConfigFromLoggingConfig(cfg.Config.Logging)
	if err := logger.Initialize(logCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logger: %v\n", err)
	} else {
		logger.Info("Logger initialized successfully", map[string]interface{}{
			"level":       logCfg.Level.String(),
			"output_file": logCfg.OutputFile,
			"console":     logCfg.Console,
		})
	}

	impl := &mcp.Implementation{Name: "DBMcp", Version: cfg.Version}
	server := mcp.NewServer(impl, nil)

	logger.Info("MCP Server starting", map[string]interface{}{
		"version": cfg.Version,
	})

	var initialDriver driver.Driver
	if cfg.InitialConnection != "" {
		conn, exists := cfg.Config.GetConnection(cfg.InitialConnection)
		if !exists {
			err := fmt.Errorf("connection '%s' not found in config", cfg.InitialConnection)
			logger.Error("Initial connection not found", err, map[string]interface{}{
				"connection": cfg.InitialConnection,
			})
			return nil, err
		}
		drv, err := initializeConnection(conn, cfg.InitialConnection)
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
		fmt.Fprintf(os.Stderr, "Successfully initialized connection: %s\n", cfg.InitialConnection)
		initialDriver = drv
	}

	tools.RegisterTools(server, cfg.Config, initialDriver)

	return server, nil
}

type StdioServerConfig struct {
	Version           string
	InitialConnection string
	Config            *config.Config
}

func initializeConnection(conn config.Connection, connectionName string) (driver.Driver, error) {
	dbClient, err := client.NewDBClient(conn.URL, conn.Type)
	if err != nil {
		logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	var drv driver.Driver
	switch conn.Type {
	case "postgres":
		drv = &driver.PostgresDriver{}
	case "mysql":
		drv = &driver.MysqlDriver{}
	case "sqlite":
		drv = &driver.SqliteDriver{}
	}

	sessionState := state.SetSession(&state.DBSessionState{
		Conn:   dbClient.DB,
		Driver: drv,
	})
	if sessionState == nil {
		_ = dbClient.Close()
		err := fmt.Errorf("failed to create session")
		logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, err)
		return nil, err
	}

	logger.LogConnectionEvent("initialize_connection", connectionName, conn.Type, nil)
	return drv, nil
}

func RunStdioServer(cfg StdioServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer state.CloseAllSessions()

	defer func() {
		if err := logger.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down logger: %v\n", err)
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
	fmt.Fprintln(os.Stderr, "DB MCP Server running ...")

	err = server.Run(ctx, &mcp.StdioTransport{})
	if err != nil {
		logger.Error("Server stopped with error", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	return err
}
