package main

import (
	"fmt"
	"os"

	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/server"
	"github.com/AbdelilahOu/DBMcp/internal/tools"
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
	rootCmd.PersistentFlags().StringP("connection", "n", "", "Named connection from config file")
	rootCmd.PersistentFlags().BoolP("read-only", "r", false, "Enable read-only mode (SELECT only)")

	stdioCmd := &cobra.Command{
		Use:   "stdio",
		Short: "Run over stdio transport (for local MCP clients)",
		RunE:  runStdioServer,
	}
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	connection, _ := cmd.Flags().GetString("connection")
	readOnly, _ := cmd.Flags().GetBool("read-only")

	var finalConnStr string

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

	return server.RunStdioServer(server.StdioServerConfig{
		DBUrl:    finalConnStr,
		ReadOnly: readOnly,
		Version:  "v0.1.0",
	})
}
