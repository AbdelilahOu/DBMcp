package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "db-mcp-server",
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

	rootCmd.PersistentFlags().StringP("conn-string", "c", os.Getenv("DB_CONN_STRING"), "DB connection string (e.g., postgres://user:pass@host/db)")
	rootCmd.PersistentFlags().StringP("connection", "n", "", "Named connection from config file")
	rootCmd.PersistentFlags().BoolP("read-only", "r", false, "Enable read-only mode (SELECT only)")
	rootCmd.PersistentFlags().StringSliceP("toolsets", "t", []string{"db"}, "Toolsets to enable (e.g., db,schema)")

	stdioCmd := &cobra.Command{
		Use:   "stdio",
		Short: "Run over stdio transport (for local MCP clients)",
		RunE:  runStdioServer,
	}
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	connStr, _ := cmd.Flags().GetString("conn-string")
	connection, _ := cmd.Flags().GetString("connection")
	readOnly, _ := cmd.Flags().GetBool("read-only")
	toolsets, _ := cmd.Flags().GetStringSlice("toolsets")

	var finalConnStr string
	var err error

	if connection != "" {

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}

		conn, exists := cfg.GetConnection(connection)
		if !exists {
			return fmt.Errorf("connection '%s' not found in config", connection)
		}

		finalConnStr = conn.URL
		fmt.Printf("Using named connection: %s (%s)\n", connection, conn.Name)

		tools.GlobalConfig = cfg
	} else if connStr != "" {

		finalConnStr = connStr
		fmt.Printf("Using direct connection string\n")
	} else {

		cfg, err := config.LoadConfig()
		if err == nil && cfg.DefaultConnection != "" {
			conn, exists := cfg.GetConnection(cfg.DefaultConnection)
			if exists {
				finalConnStr = conn.URL
				fmt.Printf("Using default connection: %s (%s)\n", cfg.DefaultConnection, conn.Name)
				tools.GlobalConfig = cfg
			}
		}

		if finalConnStr == "" {
			return fmt.Errorf("no connection specified. Use --conn-string or --connection, or set a default connection in config")
		}
	}

	dbClient, err := client.NewDBClient(finalConnStr)
	if err != nil {
		return fmt.Errorf("init DB client: %w", err)
	}
	defer dbClient.Close()

	impl := &mcp.Implementation{Name: "db-mcp-server", Version: "v0.1.0"}
	server := mcp.NewServer(impl, nil)

	tools.AddDBTools(server, dbClient, readOnly, toolsets)

	fmt.Printf("DB MCP Server running (read-only: %t, tools: %v)...\n", readOnly, toolsets)

	return server.Run(context.Background(), &mcp.StdioTransport{})
}
