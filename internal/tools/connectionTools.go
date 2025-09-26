package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListConnectionsInput struct{}

type ConnectionInfo struct {
	Name        string `json:"name" jsonschema_description:"Connection name"`
	DisplayName string `json:"display_name" jsonschema_description:"Human-readable connection name"`
	Type        string `json:"type" jsonschema_description:"Database type (postgres, mysql)"`
	Description string `json:"description" jsonschema_description:"Connection description"`
}

type ListConnectionsOutput struct {
	Connections       []ConnectionInfo `json:"connections" jsonschema_description:"Available connections"`
	DefaultConnection string           `json:"default_connection" jsonschema_description:"Default connection name"`
}

type SwitchConnectionInput struct {
	Connection string `json:"connection" jsonschema:"required" jsonschema_description:"Name of the connection to switch to"`
}

type SwitchConnectionOutput struct {
	Message    string `json:"message" jsonschema_description:"Success message"`
	Connection string `json:"connection" jsonschema_description:"Active connection name"`
}

type TestConnectionInput struct {
	Connection string `json:"connection,omitempty" jsonschema_description:"Optional connection name to test (uses current if not specified)"`
}

type TestConnectionOutput struct {
	Success    bool   `json:"success" jsonschema_description:"Whether the connection test succeeded"`
	Message    string `json:"message" jsonschema_description:"Test result message"`
	Connection string `json:"connection" jsonschema_description:"Connection that was tested"`
}

func GetListConnectionsTool(cfg *config.Config) *ToolDefinition[ListConnectionsInput, ListConnectionsOutput] {
	return NewToolDefinition[ListConnectionsInput, ListConnectionsOutput](
		"list_connections",
		"List all available named connections from config.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListConnectionsInput) (*mcp.CallToolResult, ListConnectionsOutput, error) {
			return listConnectionsHandler(ctx, req, input, cfg)
		},
	)
}

func listConnectionsHandler(ctx context.Context, req *mcp.CallToolRequest, input ListConnectionsInput, cfg *config.Config) (*mcp.CallToolResult, ListConnectionsOutput, error) {
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

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, ListConnectionsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func GetSwitchConnectionTool(cfg *config.Config) *ToolDefinition[SwitchConnectionInput, SwitchConnectionOutput] {
	return NewToolDefinition[SwitchConnectionInput, SwitchConnectionOutput](
		"switch_connection",
		"Switch to a different database connection during the session.",
		func(ctx context.Context, req *mcp.CallToolRequest, input SwitchConnectionInput) (*mcp.CallToolResult, SwitchConnectionOutput, error) {
			return switchConnectionHandler(ctx, req, input, cfg)
		},
	)
}

func switchConnectionHandler(ctx context.Context, req *mcp.CallToolRequest, input SwitchConnectionInput, cfg *config.Config) (*mcp.CallToolResult, SwitchConnectionOutput, error) {
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

	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil {
		logger.LogConnectionEvent("switch_connection", input.Connection, conn.Type, fmt.Errorf("failed to create session"))
		return nil, SwitchConnectionOutput{}, fmt.Errorf("failed to create session")
	}

	sessionState.Conn = dbClient.DB

	// Log successful connection switch
	logger.LogConnectionEvent("switch_connection", input.Connection, conn.Type, nil)

	output := SwitchConnectionOutput{
		Message:    fmt.Sprintf("Successfully switched to connection '%s'", input.Connection),
		Connection: input.Connection,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, SwitchConnectionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func GetTestConnectionTool(cfg *config.Config) *ToolDefinition[TestConnectionInput, TestConnectionOutput] {
	return NewToolDefinition[TestConnectionInput, TestConnectionOutput](
		"test_connection",
		"Test connectivity to a database before executing queries.",
		func(ctx context.Context, req *mcp.CallToolRequest, input TestConnectionInput) (*mcp.CallToolResult, TestConnectionOutput, error) {
			return testConnectionHandler(ctx, req, input, cfg)
		},
	)
}

func testConnectionHandler(ctx context.Context, req *mcp.CallToolRequest, input TestConnectionInput, cfg *config.Config) (*mcp.CallToolResult, TestConnectionOutput, error) {
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

			jsonBytes, _ := json.Marshal(output)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
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

			jsonBytes, _ := json.Marshal(output)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
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

			jsonBytes, _ := json.Marshal(output)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		}

		connectionName = "current"
	}

	// Log successful connection test
	logger.LogConnectionEvent("test_connection", connectionName, "unknown", nil)

	output := TestConnectionOutput{
		Success:    true,
		Message:    "Connection test successful",
		Connection: connectionName,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, TestConnectionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}
