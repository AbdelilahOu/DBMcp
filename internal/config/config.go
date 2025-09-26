package config

import (
	"encoding/json"
	"fmt"
	"os"
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

func LoadConfig(configPath string) (*Config, error) {
	config, err := loadConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}
	return config, nil

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
