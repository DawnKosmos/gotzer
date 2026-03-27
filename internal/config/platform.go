package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// PlatformConfig represents the .gotzer-platform.yaml configuration
// for hosting multiple applications on a single server with reverse proxy.
type PlatformConfig struct {
	Domain        string                `yaml:"domain"`
	Server        ServerConfig          `yaml:"server"`
	Services      ServicesConfig        `yaml:"services,omitempty"`
	ServiceRoutes []ServiceRoute        `yaml:"service_routes,omitempty"`
	Apps          map[string]*AppConfig `yaml:"apps"`
}

// ServiceRoute exposes a Docker service (by its internal port) behind a Caddy subdomain.
// Use this for Typesense, Centrifugo, etc. that run as Docker services but need a public URL.
type ServiceRoute struct {
	Subdomain string `yaml:"subdomain"` // e.g. "search" → search.yeabuddy.de
	Port      int    `yaml:"port"`      // internal port the Docker service listens on
}

// AppConfig defines a single application within the platform.
type AppConfig struct {
	Subdomain string          `yaml:"subdomain"`
	Path      string          `yaml:"path,omitempty"` // source directory, defaults to "."
	Build     AppBuildConfig  `yaml:"build"`
	Deploy    AppDeployConfig `yaml:"deploy"`
}

// AppBuildConfig defines how to build a single application.
type AppBuildConfig struct {
	Type    string            `yaml:"type"`              // "go" or "static"
	Main    string            `yaml:"main,omitempty"`    // Go main package path
	Output  string            `yaml:"output"`            // binary name or output dir
	Command string            `yaml:"command,omitempty"` // static build command
	Dir     string            `yaml:"dir,omitempty"`     // static build output dir
	LDFlags string            `yaml:"ldflags,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// AppDeployConfig defines how to deploy a single application.
type AppDeployConfig struct {
	Type       string            `yaml:"type,omitempty"`        // "service" (default) or "static"
	Port       int               `yaml:"port,omitempty"`        // internal port (service apps only)
	RemotePath string            `yaml:"remote_path,omitempty"` // override remote path (static apps)
	Command    []string          `yaml:"command,omitempty"`     // args appended to binary
	Env        map[string]string `yaml:"env,omitempty"`
}

// LoadPlatform reads the platform configuration from .gotzer-platform.yaml
func LoadPlatform(path string) (*PlatformConfig, error) {
	if path == "" {
		path = ".gotzer-platform.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("platform config not found. Run 'gotzer platform init' first")
		}
		return nil, fmt.Errorf("failed to read platform config: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg PlatformConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse platform config: %w", err)
	}

	// Set defaults
	if cfg.Server.Architecture == "" {
		cfg.Server.Architecture = "x64"
	}

	for name, app := range cfg.Apps {
		if app.Build.Type == "" {
			app.Build.Type = "go"
		}
		if app.Build.LDFlags == "" && app.Build.Type == "go" {
			app.Build.LDFlags = "-s -w"
		}
		if app.Path == "" {
			app.Path = "."
		}
		if app.Subdomain == "" {
			app.Subdomain = name
		}
		if app.Deploy.Type == "" {
			if app.Build.Type == "static" {
				app.Deploy.Type = "static"
			} else {
				app.Deploy.Type = "service"
			}
		}
		if app.Deploy.Type == "static" && app.Deploy.RemotePath == "" {
			app.Deploy.RemotePath = fmt.Sprintf("/var/www/%s", name)
		}
	}

	return &cfg, nil
}

// AppRemotePath returns the remote deployment path for an app.
func (p *PlatformConfig) AppRemotePath(appName string) string {
	return fmt.Sprintf("/opt/apps/%s", appName)
}

// AppServiceName returns the systemd service name for an app.
func (p *PlatformConfig) AppServiceName(appName string) string {
	return appName
}

// SortedAppNames returns app names in sorted order for deterministic output.
func (p *PlatformConfig) SortedAppNames() []string {
	names := make([]string, 0, len(p.Apps))
	for name := range p.Apps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
