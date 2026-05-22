package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/driver"
	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListConnectionsInput struct{}

type ConnectionInfo struct {
	Name        string `json:"name" jsonschema_description:"Connection name"`
	DisplayName string `json:"display_name" jsonschema_description:"Human-readable connection name"`
	Type        string `json:"type" jsonschema_description:"Database type (postgres, mysql)"`
	Description string `json:"description,omitempty" jsonschema_description:"Connection description"`
}

type ListConnectionsOutput struct {
	Connections       []ConnectionInfo `json:"connections" jsonschema_description:"Available connections"`
	DefaultConnection string           `json:"default_connection" jsonschema_description:"Default connection name"`
}

func GetListConnectionsTool(cfg *config.Config) *ToolDefinition[ListConnectionsInput, ListConnectionsOutput] {
	return NewToolDefinition[ListConnectionsInput, ListConnectionsOutput](
		"list_connections",
		"List all available named connections from config.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListConnectionsInput) (*mcp.CallToolResult, ListConnectionsOutput, error) {
			if cfg == nil {
				return nil, ListConnectionsOutput{}, fmt.Errorf("config not loaded - server must be started with a valid config file")
			}

			connections := make([]ConnectionInfo, 0, len(cfg.Connections))

			for name, conn := range cfg.Connections {
				connections = append(connections, ConnectionInfo{
					Name:        name,
					DisplayName: conn.Name,
					Type:        conn.Type,
					Description: conn.Description,
				})
			}

			output := ListConnectionsOutput{
				Connections:       connections,
				DefaultConnection: cfg.DefaultConnection,
			}

			sort.Slice(connections, func(i, j int) bool {
				return connections[i].Name < connections[j].Name
			})

			lines := []string{fmt.Sprintf("Available connections: %d", len(connections))}
			for _, c := range connections {
				description := strings.TrimSpace(c.Description)
				if description == "" {
					description = "no description"
				}

				line := fmt.Sprintf("- %s: %s (driver: %s)", c.Name, description, c.Type)
				if c.Name == output.DefaultConnection {
					line += " [default]"
				}
				lines = append(lines, line)
			}
			msg := strings.Join(lines, "\n")

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: msg},
				},
			}, output, nil
		},
	)
}

type SwitchConnectionInput struct {
	Connection string `json:"connection" jsonschema:"required" jsonschema_description:"Name of the connection to switch to"`
}

type SwitchConnectionOutput struct {
	Status string `json:"status" jsonschema_description:"Success message"`
}

func GetSwitchConnectionTool(cfg *config.Config) *ToolDefinition[SwitchConnectionInput, SwitchConnectionOutput] {
	return NewToolDefinition[SwitchConnectionInput, SwitchConnectionOutput](
		"switch_connection",
		"Switch to different DB connection in session.",
		func(ctx context.Context, req *mcp.CallToolRequest, input SwitchConnectionInput) (*mcp.CallToolResult, SwitchConnectionOutput, error) {
			if cfg == nil {
				return nil, SwitchConnectionOutput{}, fmt.Errorf("config not loaded - server must be started with a valid config file")
			}

			conn, exists := cfg.GetConnection(input.Connection)
			if !exists {
				return nil, SwitchConnectionOutput{}, fmt.Errorf("connection '%s' not found", input.Connection)
			}

			dbClient, err := client.NewDBClient(conn.URL, conn.Type)
			if err != nil {
				logger.LogConnectionEvent("switch_connection", input.Connection, conn.Type, err)
				return nil, SwitchConnectionOutput{}, fmt.Errorf("failed to connect to '%s': %v", input.Connection, err)
			}

			var dbDriver driver.Driver
			switch conn.Type {
			case "postgres":
				dbDriver = &driver.PostgresDriver{}
			case "mysql":
				dbDriver = &driver.MysqlDriver{}
			case "sqlite":
				dbDriver = &driver.SqliteDriver{}
			}

			sessionState := state.SetSession("default", &state.DBSessionState{
				Conn:   dbClient.DB,
				Driver: dbDriver,
			})

			if sessionState == nil {
				_ = dbClient.Close()
				logger.LogConnectionEvent("switch_connection", input.Connection, conn.Type, fmt.Errorf("failed to create session"))
				return nil, SwitchConnectionOutput{}, fmt.Errorf("failed to create session")
			}

			logger.LogConnectionEvent("switch_connection", input.Connection, conn.Type, nil)

			output := SwitchConnectionOutput{
				Status: fmt.Sprintf("Successfully switched to connection '%s'", input.Connection),
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: output.Status},
				},
			}, output, nil
		},
	)
}

type TestConnectionInput struct {
	Connection string `json:"connection,omitempty" jsonschema_description:"Optional connection name to test (uses current if not specified)"`
}

type TestConnectionOutput struct {
	Success    bool   `json:"success" jsonschema_description:"Whether the connection test succeeded"`
	Message    string `json:"msg" jsonschema_description:"Test result message"`
	Connection string `json:"connection" jsonschema_description:"Connection that was tested"`
}

func GetTestConnectionTool(cfg *config.Config) *ToolDefinition[TestConnectionInput, TestConnectionOutput] {
	return NewToolDefinition[TestConnectionInput, TestConnectionOutput](
		"test_connection",
		"Test DB connectivity. Tests specific connection if name provided, else current active connection.",
		func(ctx context.Context, req *mcp.CallToolRequest, input TestConnectionInput) (*mcp.CallToolResult, TestConnectionOutput, error) {
			var connectionName string
			var testClient *client.DBClient
			var err error

			if input.Connection != "" {
				if cfg == nil {
					return nil, TestConnectionOutput{}, fmt.Errorf("config not loaded - server must be started with a valid config file")
				}

				conn, exists := cfg.GetConnection(input.Connection)
				if !exists {
					return nil, TestConnectionOutput{}, fmt.Errorf("connection '%s' not found", input.Connection)
				}

				testClient, err = client.NewDBClient(conn.URL, conn.Type)
				if err != nil {
					logger.LogConnectionEvent("test_connection", input.Connection, conn.Type, err)
					output := TestConnectionOutput{
						Success:    false,
						Message:    fmt.Sprintf("Connection test failed: %v", err),
						Connection: input.Connection,
					}

					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: output.Message},
						},
					}, output, nil
				}
				defer testClient.Close()

				connectionName = input.Connection
			} else {
				sessionState := state.GetSession("default")
				if sessionState == nil || sessionState.Conn == nil {
					output := TestConnectionOutput{
						Success:    false,
						Message:    "No active connection to test",
						Connection: "current",
					}

					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: output.Message},
						},
					}, output, nil
				}

				if err := sessionState.Conn.Ping(); err != nil {
					logger.LogConnectionEvent("test_connection", "current", "unknown", err)
					output := TestConnectionOutput{
						Success:    false,
						Message:    fmt.Sprintf("Connection test failed: %v", err),
						Connection: "current",
					}

					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: output.Message},
						},
					}, output, nil
				}

				connectionName = "current"
			}

			logger.LogConnectionEvent("test_connection", connectionName, "unknown", nil)

			output := TestConnectionOutput{
				Success:    true,
				Message:    "Connection test successful",
				Connection: connectionName,
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("%s (%s)", output.Message, output.Connection)},
				},
			}, output, nil
		},
	)
}
