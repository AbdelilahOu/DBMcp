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
	Short: "DB MCP Server for querying Postgres/MySQL",
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

	var initialConnection string

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		fmt.Fprintln(os.Stderr, "Server will start without connections. Use list_connections and switch_connection tools.")
	} else {
		if connection != "" {
			if _, exists := cfg.GetConnection(connection); exists {
				fmt.Fprintf(os.Stderr, "Config loaded. Will initialize connection: %s\n", connection)
				initialConnection = connection
			} else {
				return fmt.Errorf("connection '%s' not found in config", connection)
			}
		} else if cfg.DefaultConnection != "" {
			if _, exists := cfg.GetConnection(cfg.DefaultConnection); exists {
				fmt.Fprintf(os.Stderr, "Config loaded. Will initialize default connection: %s\n", cfg.DefaultConnection)
				initialConnection = cfg.DefaultConnection
			} else {
				fmt.Fprintf(os.Stderr, "Config loaded. Default connection '%s' not found, starting without initial connection.\n", cfg.DefaultConnection)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Config loaded. Use list_connections and switch_connection tools to connect to a database.")
		}
	}

	return server.RunStdioServer(server.StdioServerConfig{
		Version:           "v0.1.0",
		InitialConnection: initialConnection,
		Config:            cfg,
	})
}
