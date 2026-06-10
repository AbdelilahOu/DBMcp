package main

import (
	"fmt"
	"os"

	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "DBMcp",
	Short: "DB MCP Server for querying Postgres, MySQL & Sqlite",
	Long:  `A Model Context Protocol (MCP) server exposing DB tools for AI clients.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("connection", "n", "", "Named connection from config file")
	rootCmd.PersistentFlags().StringP("config", "c", "", "connections config file path")

	stdioCmd := &cobra.Command{
		Use:   "stdio",
		Short: "Run over stdio transport (for local MCP clients)",
		RunE:  runStdioServer,
	}
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	connection, _ := cmd.Flags().GetString("connection")
	configPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Connections) == 0 {
		return fmt.Errorf("config loaded but contains no connections")
	}

	initialConnection, message, err := resolveInitialConnection(cfg, connection)
	if err != nil {
		return err
	}
	if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}

	return server.RunStdioServer(server.StdioServerConfig{
		Version:           "v0.1.0",
		InitialConnection: initialConnection,
		Config:            cfg,
	})
}

func resolveInitialConnection(cfg *config.Config, requested string) (string, string, error) {
	if requested != "" {
		if _, exists := cfg.GetConnection(requested); !exists {
			return "", "", fmt.Errorf("connection '%s' not found in config", requested)
		}
		return requested, fmt.Sprintf("Config loaded. Will initialize connection: %s", requested), nil
	}

	if cfg.DefaultConnection != "" {
		if _, exists := cfg.GetConnection(cfg.DefaultConnection); exists {
			return cfg.DefaultConnection, fmt.Sprintf("Config loaded. Will initialize default connection: %s", cfg.DefaultConnection), nil
		}

		if name, ok := onlyConnectionName(cfg); ok {
			message := fmt.Sprintf("Config loaded. Default connection '%s' not found. Will initialize only connection: %s", cfg.DefaultConnection, name)
			return name, message, nil
		}

		message := fmt.Sprintf("Config loaded. Default connection '%s' not found, starting without initial connection.", cfg.DefaultConnection)
		return "", message, nil
	}

	if name, ok := onlyConnectionName(cfg); ok {
		return name, fmt.Sprintf("Config loaded. Will initialize only connection: %s", name), nil
	}

	return "", "Config loaded. Use list_connections and switch_connection tools to connect to a database.", nil
}

func onlyConnectionName(cfg *config.Config) (string, bool) {
	if len(cfg.Connections) != 1 {
		return "", false
	}

	for name := range cfg.Connections {
		return name, true
	}

	return "", false
}
