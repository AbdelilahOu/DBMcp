package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Connection struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type Config struct {
	Connections       map[string]Connection `json:"connections"`
	DefaultConnection string                `json:"default_connection"`
}

func LoadConfig() (*Config, error) {
	configPaths := getConfigPaths()

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			config, err := loadConfigFromFile(path)
			if err != nil {
				continue
			}
			return config, nil
		}
	}

	return &Config{
		Connections: make(map[string]Connection),
	}, nil
}

func (c *Config) GetConnection(name string) (Connection, bool) {
	conn, exists := c.Connections[name]
	return conn, exists
}

func (c *Config) ListConnections() map[string]Connection {
	return c.Connections
}

func (c *Config) ValidateConnection(conn Connection) error {
	if conn.Name == "" {
		return fmt.Errorf("connection name is required")
	}
	if conn.Type == "" {
		return fmt.Errorf("connection type is required")
	}
	if conn.Type != "postgres" && conn.Type != "mysql" {
		return fmt.Errorf("connection type must be 'postgres' or 'mysql'")
	}
	if conn.URL == "" {
		return fmt.Errorf("connection URL is required")
	}
	return nil
}

func getConfigPaths() []string {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "db-mcp", "connections.json"))
		}
	default:

		homeDir := os.Getenv("HOME")
		if homeDir != "" {
			paths = append(paths, filepath.Join(homeDir, ".config", "db-mcp", "connections.json"))
		}
	}

	if pwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(pwd, "connections.json"))
	}

	return paths
}

func loadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	for name, conn := range config.Connections {
		conn.Name = name
		if err := config.ValidateConnection(conn); err != nil {
			return nil, fmt.Errorf("invalid connection %s: %v", name, err)
		}
	}

	return &config, nil
}
