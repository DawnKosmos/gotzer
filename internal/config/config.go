package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the .gotzer.yaml project configuration
type Config struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Server      ServerConfig   `yaml:"server"`
	Build       BuildConfig    `yaml:"build"`
	Deploy      DeployConfig   `yaml:"deploy"`
	Services    ServicesConfig `yaml:"services,omitempty"`
}

type ServerConfig struct {
	Name         string `yaml:"name"`
	Location     string `yaml:"location"`
	Type         string `yaml:"type"`
	Image        string `yaml:"image"`
	Architecture string `yaml:"architecture"` // x64 or arm64
}

type BuildConfig struct {
	Main    string            `yaml:"main"`
	Output  string            `yaml:"output"`
	LDFlags string            `yaml:"ldflags,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type DeployConfig struct {
	RemotePath  string            `yaml:"remote_path"`
	ServiceName string            `yaml:"service_name"`
	User        string            `yaml:"user"`
	Command     []string          `yaml:"command,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
}

type ServicesConfig struct {
	Postgres  *ServiceConfig  `yaml:"postgres,omitempty"`
	Typesense *ServiceConfig  `yaml:"typesense,omitempty"`
	Redis     *ServiceConfig  `yaml:"redis,omitempty"`
	Custom    []ServiceConfig `yaml:"custom,omitempty"`
}

type ServiceConfig struct {
	Enabled bool              `yaml:"enabled"`
	Image   string            `yaml:"image"`
	Port    int               `yaml:"port"`
	Volumes []string          `yaml:"volumes,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// Load reads the project configuration from .gotzer.yaml
func Load(path string) (*Config, error) {
	if path == "" {
		path = ".gotzer.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found. Run 'gotzer init' first")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Server.Architecture == "" {
		config.Server.Architecture = "x64"
	}
	if config.Deploy.User == "" {
		config.Deploy.User = "app"
	}

	return &config, nil
}

// GOARCH returns the Go architecture string
func (s *ServerConfig) GOARCH() string {
	switch strings.ToLower(s.Architecture) {
	case "arm64", "arm":
		return "arm64"
	default:
		return "amd64"
	}
}

// ExpandPath expands ~ to home directory
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
