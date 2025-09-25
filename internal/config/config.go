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

func LoadConfigFromFile(path string) (*Config, error) {
	return loadConfigFromFile(path)
}

func (c *Config) SaveConfig() error {
	configPaths := getConfigPaths()
	configPath := configPaths[0]

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	return c.saveConfigToFile(configPath)
}

func (c *Config) SaveConfigToFile(path string) error {
	return c.saveConfigToFile(path)
}

func (c *Config) GetConnection(name string) (Connection, bool) {
	conn, exists := c.Connections[name]
	return conn, exists
}

func (c *Config) AddConnection(name string, conn Connection) {
	if c.Connections == nil {
		c.Connections = make(map[string]Connection)
	}
	c.Connections[name] = conn
}

func (c *Config) RemoveConnection(name string) {
	delete(c.Connections, name)
}

func (c *Config) ListConnections() map[string]Connection {
	return c.Connections
}

func (c *Config) GetDefaultConnection() string {
	return c.DefaultConnection
}

func (c *Config) SetDefaultConnection(name string) {
	c.DefaultConnection = name
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

func (c *Config) saveConfigToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}
