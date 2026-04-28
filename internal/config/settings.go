package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RoutingStrategy defines how requests are routed to backends
type RoutingStrategy string

const (
	RoundRobin RoutingStrategy = "round-robin"
	QueryParam RoutingStrategy = "query-param"
	Header     RoutingStrategy = "header"
	Cookie     RoutingStrategy = "cookie"
	Static     RoutingStrategy = "static"
)

// Backend represents a single backend service
type Backend struct {
	Name   string   `yaml:"name"`
	Host   string   `yaml:"host"`
	Port   int      `yaml:"port"`
	Groups []string `yaml:"groups"`
}

// RoutingRule defines how to route requests for one step in the ordered routing chain.
// For round-robin, Targets lists backend names eligible for that hop (order defines
// rotation). If Targets is empty, all backends participate.
type RoutingRule struct {
	Strategy     RoutingStrategy `yaml:"strategy"`
	ParamName    string          `yaml:"param_name"`
	DefaultGroup string          `yaml:"default_group"`
	Targets      []string        `yaml:"targets"` // backend names for round-robin only
}

// App contains application-level settings
type App struct {
	Port string `yaml:"port"`
}

// Config is the root configuration structure
type Config struct {
	App      App         `yaml:"app"`
	Backends []Backend   `yaml:"backends"`
	Routing  RoutingList `yaml:"routing"`
	State    State       `yaml:"state"`
}

// Parse unmarshals YAML into Config, applies defaults, and validates.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	applyDefaults(&cfg)
	if err := Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load reads configuration from ABBIE_CONFIG (default config.yaml), applies env overrides.
func Load() (*Config, error) {
	path := os.Getenv("ABBIE_CONFIG")
	if path == "" {
		path = "config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if port := os.Getenv("ABBIE_PORT"); port != "" {
		cfg.App.Port = port
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.App.Port == "" {
		cfg.App.Port = "8080"
	}
	if cfg.State.Redis.KeyPrefix == "" {
		cfg.State.Redis.KeyPrefix = "abbie:rr:"
	}
}

// Validate returns an error if the configuration is unusable.
func Validate(cfg *Config) error {
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured")
	}
	return validateRouting(cfg)
}
