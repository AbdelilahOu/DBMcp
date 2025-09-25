package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/AbdelilahOu/DBMcp/internal/config"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/AbdelilahOu/DBMcp/pkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var GlobalConfig *config.Config

func listConnectionsHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.ListConnectionsInput) (*mcp.CallToolResult, mcpdb.ListConnectionsOutput, error) {
	if GlobalConfig == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			return nil, mcpdb.ListConnectionsOutput{}, fmt.Errorf("failed to load config: %v", err)
		}
		GlobalConfig = cfg
	}

	connections := make([]mcpdb.ConnectionInfo, 0, len(GlobalConfig.Connections))

	for name, conn := range GlobalConfig.Connections {
		connections = append(connections, mcpdb.ConnectionInfo{
			Name:        name,
			DisplayName: conn.Name,
			Type:        conn.Type,
			Description: conn.Description,
		})
	}

	output := mcpdb.ListConnectionsOutput{
		Connections:       connections,
		DefaultConnection: GlobalConfig.DefaultConnection,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, mcpdb.ListConnectionsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func switchConnectionHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.SwitchConnectionInput) (*mcp.CallToolResult, mcpdb.SwitchConnectionOutput, error) {
	if GlobalConfig == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			return nil, mcpdb.SwitchConnectionOutput{}, fmt.Errorf("failed to load config: %v", err)
		}
		GlobalConfig = cfg
	}

	conn, exists := GlobalConfig.GetConnection(input.Connection)
	if !exists {
		return nil, mcpdb.SwitchConnectionOutput{}, fmt.Errorf("connection '%s' not found", input.Connection)
	}

	dbClient, err := client.NewDBClient(conn.URL)
	if err != nil {
		return nil, mcpdb.SwitchConnectionOutput{}, fmt.Errorf("failed to connect to '%s': %v", input.Connection, err)
	}

	sessionID := "default"
	sessionState := state.GetOrCreateSession(sessionID, dbClient)
	if sessionState == nil {
		return nil, mcpdb.SwitchConnectionOutput{}, fmt.Errorf("failed to create session")
	}

	sessionState.Conn = dbClient.DB

	output := mcpdb.SwitchConnectionOutput{
		Message:    fmt.Sprintf("Successfully switched to connection '%s'", input.Connection),
		Connection: input.Connection,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, mcpdb.SwitchConnectionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}

func testConnectionHandler(ctx context.Context, req *mcp.CallToolRequest, input mcpdb.TestConnectionInput, dbClient *client.DBClient) (*mcp.CallToolResult, mcpdb.TestConnectionOutput, error) {
	var connectionName string
	var testClient *client.DBClient
	var err error

	if input.Connection != "" {
		if GlobalConfig == nil {
			cfg, err := config.LoadConfig()
			if err != nil {
				return nil, mcpdb.TestConnectionOutput{}, fmt.Errorf("failed to load config: %v", err)
			}
			GlobalConfig = cfg
		}

		conn, exists := GlobalConfig.GetConnection(input.Connection)
		if !exists {
			return nil, mcpdb.TestConnectionOutput{}, fmt.Errorf("connection '%s' not found", input.Connection)
		}

		testClient, err = client.NewDBClient(conn.URL)
		if err != nil {
			output := mcpdb.TestConnectionOutput{
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
		sessionID := "default"
		sessionState := state.GetOrCreateSession(sessionID, dbClient)
		if sessionState == nil || sessionState.Conn == nil {
			output := mcpdb.TestConnectionOutput{
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
			output := mcpdb.TestConnectionOutput{
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

	output := mcpdb.TestConnectionOutput{
		Success:    true,
		Message:    "Connection test successful",
		Connection: connectionName,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, mcpdb.TestConnectionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, output, nil
}
