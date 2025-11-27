package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RoutingStrategy defines how requests are routed to backends
type RoutingStrategy string

const (
	RoundRobin  RoutingStrategy = "round-robin"
	QueryParam  RoutingStrategy = "query-param"
	Header      RoutingStrategy = "header"
	Cookie      RoutingStrategy = "cookie"
	Static      RoutingStrategy = "static"
)

// Backend represents a single backend service
type Backend struct {
	Name     string   `yaml:"name"`
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	Groups   []string `yaml:"groups"`   // e.g., ["defense", "healthcare"]
}

// RoutingRule defines how to route requests
type RoutingRule struct {
	Strategy     RoutingStrategy `yaml:"strategy"`
	ParamName    string          `yaml:"param_name"`    // for query-param, header, or cookie strategies
	DefaultGroup string          `yaml:"default_group"` // fallback when no match
}

// App contains application-level settings
type App struct {
	Port string `yaml:"port"`
}

// Config is the root configuration structure
type Config struct {
	App      App           `yaml:"app"`
	Backends []Backend     `yaml:"backends"`
	Routing  RoutingRule   `yaml:"routing"`
}

// NewConfigFactory loads configuration from a YAML file or environment variables
func NewConfigFactory() (*Config, error) {
	configPath := os.Getenv("ABBIE_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Load from file system
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables if set
	if port := os.Getenv("ABBIE_PORT"); port != "" {
		config.App.Port = port
	}

	// Set defaults
	if config.App.Port == "" {
		config.App.Port = "8080"
	}

	// Validate configuration
	if len(config.Backends) == 0 {
		return nil, fmt.Errorf("no backends configured")
	}

	return &config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
