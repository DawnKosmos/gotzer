package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Gotzer project",
	Long:  `Creates a .gotzer.yaml configuration file in the current directory.`,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := ".gotzer.yaml"

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file %s already exists", configPath)
	}

	// Get project name from directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectName := filepath.Base(cwd)

	config := fmt.Sprintf(`# Gotzer Configuration
# See: https://github.com/DawnKosmos/gotzer

name: %s
description: "My Go application"

# Hetzner Server Configuration
server:
  name: %s-server
  location: fsn1                    # fsn1, nbg1, hel1, ash, hil
  type: cpx11                       # Shared vCPU: cpx11, cpx21, cpx31 | ARM: cax11, cax21
  image: ubuntu-24.04
  architecture: x64                 # x64 or arm64

# Go Build Configuration
build:
  main: ./cmd/server                # Path to main package
  output: app                       # Binary name
  ldflags: "-s -w"                  # Strip debug info for smaller binary

# Deployment Configuration
deploy:
  remote_path: /opt/apps/%s
  service_name: %s
  user: app
  env:
    APP_ENV: production
    # DATABASE_URL: "postgres://user:pass@localhost:5432/myapp?sslmode=disable"

# Docker Services (optional)
services:
  postgres:
    enabled: false
    image: postgres:16
    port: 5432
    volumes:
      - pgdata:/var/lib/postgresql/data
    env:
      POSTGRES_DB: myapp
      POSTGRES_USER: myapp
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"

  typesense:
    enabled: false
    image: typesense/typesense:27.1
    port: 8108
    volumes:
      - typesense-data:/data
    env:
      TYPESENSE_API_KEY: "${TYPESENSE_API_KEY}"
      TYPESENSE_DATA_DIR: /data
`, projectName, projectName, projectName, projectName)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	printSuccess(fmt.Sprintf("Created %s", configPath))
	printInfo("Next steps:")
	fmt.Println("  1. Edit .gotzer.yaml to configure your project")
	fmt.Println("  2. Run 'gotzer auth' to set your Hetzner API token")
	fmt.Println("  3. Run 'gotzer provision' to create your server")
	fmt.Println("  4. Run 'gotzer deploy' to deploy your app")

	return nil
}
