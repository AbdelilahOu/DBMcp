package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "db-mcp-server",
	Short: "DB MCP Server for querying Postgres/MySQL",
	Long:  `A Model Context Protocol (MCP) server exposing DB tools for AI clients.`,
	// RunE: runServer,  // Handled in main.go for global setup
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags (persistent across subcmds, like GitHub MCP)
	rootCmd.PersistentFlags().StringP("conn-string", "c", os.Getenv("DB_CONN_STRING"), "DB connection string (e.g., postgres://user:pass@host/db)")
	rootCmd.PersistentFlags().BoolP("read-only", "r", false, "Enable read-only mode (SELECT only)")
	rootCmd.PersistentFlags().StringSliceP("toolsets", "t", []string{"db"}, "Toolsets to enable (e.g., db,schema)")

	// Subcommand: stdio (local transport, like IDE integration)
	stdioCmd := &cobra.Command{
		Use:   "stdio",
		Short: "Run over stdio transport (for local MCP clients)",
		RunE:  runStdioServer,
	}
	rootCmd.AddCommand(stdioCmd)

	// Subcommand: http (remote, optional for future)
	httpCmd := &cobra.Command{
		Use:   "http",
		Short: "Run over HTTP transport (for remote clients)",
		RunE:  runHTTPServer, // Stub for now
	}
	rootCmd.AddCommand(httpCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	connStr, _ := cmd.Flags().GetString("conn-string")
	readOnly, _ := cmd.Flags().GetBool("read-only")
	toolsets, _ := cmd.Flags().GetStringSlice("toolsets")

	if connStr == "" {
		return fmt.Errorf("conn-string required")
	}

	// Init DB client (global, pooled)
	dbClient, err := client.NewDBClient(connStr)
	if err != nil {
		return fmt.Errorf("init DB client: %w", err)
	}
	defer dbClient.Close()

	// MCP server setup (like GitHub MCP)
	impl := &mcp.Implementation{Name: "db-mcp-server", Version: "v0.1.0"}
	server := mcp.NewServer(impl, nil) // Options can add session hooks later

	// Register tools (inject dbClient via closure, like GH client)
	tools.AddDBTools(server, dbClient, readOnly, toolsets)

	fmt.Printf("DB MCP Server running (read-only: %t, tools: %v)...\n", readOnly, toolsets)
	// Run over stdio
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func runHTTPServer(cmd *cobra.Command, args []string) error {
	// TODO: Implement HTTP transport (mcp.HTTPTransport)
	return fmt.Errorf("HTTP not implemented yet")
}
