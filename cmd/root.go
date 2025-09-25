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

	// Load config and set global config for tools to use
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: Failed to load config: %v\n", err)
		fmt.Println("Server will start without connections. Use list_connections and switch_connection tools.")
	} else {
		tools.GlobalConfig = cfg
		if connection != "" {
			if _, exists := cfg.GetConnection(connection); exists {
				fmt.Printf("Config loaded. Default connection available: %s\n", connection)
			} else {
				return fmt.Errorf("connection '%s' not found in config", connection)
			}
		} else if cfg.DefaultConnection != "" {
			fmt.Printf("Config loaded. Default connection available: %s\n", cfg.DefaultConnection)
		} else {
			fmt.Println("Config loaded. Use list_connections and switch_connection tools to connect to a database.")
		}
	}

	return server.RunStdioServer(server.StdioServerConfig{
		ReadOnly: readOnly,
		Version:  "v0.1.0",
	})
}
