package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/deploy"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/provision"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var platformCfgFile string

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Manage multi-app platform deployments",
	Long: `Platform commands for hosting multiple applications on a single server
with automatic reverse proxy (Caddy), shared Docker services, and
per-app systemd management.

Quick start:
  gotzer platform init               # Create .gotzer-platform.yaml
  gotzer auth                        # Configure Hetzner API token
  gotzer platform provision          # Create server + setup platform
  gotzer platform deploy             # Deploy all apps
  gotzer platform deploy --app blog  # Deploy a single app`,
}

var platformInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a platform configuration file",
	Long:  `Creates a .gotzer-platform.yaml for hosting multiple apps on a single server.`,
	RunE:  runPlatformInit,
}

var platformProvisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision server with reverse proxy and shared services",
	Long: `Creates a Hetzner server and sets up:
  - Docker and Docker Compose
  - Caddy reverse proxy with automatic HTTPS
  - Application user and per-app directories
  - Per-app systemd services
  - Shared Docker services (PostgreSQL, Redis, etc.)
  - UFW firewall rules`,
	RunE: runPlatformProvision,
}

var (
	platformProvisionUpdate bool
	platformSSHKeyName      string
)

var platformDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy applications",
	Long:  `Deploy all applications, or a specific one with --app.`,
	RunE:  runPlatformDeploy,
}

var platformDeployApp string

var platformStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show platform status",
	Long:  `Displays the status of your platform server, Caddy, apps, and Docker services.`,
	RunE:  runPlatformStatus,
}

func init() {
	platformCmd.PersistentFlags().StringVar(&platformCfgFile, "config", "", "platform config file (default .gotzer-platform.yaml)")

	platformProvisionCmd.Flags().BoolVar(&platformProvisionUpdate, "update", false, "Update existing server configuration")
	platformProvisionCmd.Flags().StringVar(&platformSSHKeyName, "ssh-key", "", "SSH key name in Hetzner")

	platformDeployCmd.Flags().StringVar(&platformDeployApp, "app", "", "Deploy a specific app (deploys all if not set)")

	platformCmd.AddCommand(platformInitCmd)
	platformCmd.AddCommand(platformProvisionCmd)
	platformCmd.AddCommand(platformDeployCmd)
	platformCmd.AddCommand(platformStatusCmd)
}

func runPlatformInit(cmd *cobra.Command, args []string) error {
	configPath := ".gotzer-platform.yaml"
	if platformCfgFile != "" {
		configPath = platformCfgFile
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file %s already exists", configPath)
	}

	template := `# Gotzer Platform Configuration
# Hosts multiple apps on a single server with reverse proxy (Caddy)
# See: https://github.com/DawnKosmos/gotzer

domain: example.com

server:
  name: my-platform
  location: nbg1                    # fsn1, nbg1, hel1, ash, hil
  type: cax21                       # ARM64 shared server
  image: ubuntu-24.04
  architecture: arm64

# Shared Docker services (available to all apps)
services:
  postgres:
    enabled: true
    image: postgres:16
    port: 5432
    bind_ip: "127.0.0.1"
    volumes:
      - pgdata:/var/lib/postgresql/data
    env:
      POSTGRES_DB: platform
      POSTGRES_USER: platform
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"

  typesense:
    enabled: false
    image: typesense/typesense:27.1
    port: 8108
    bind_ip: "127.0.0.1"
    volumes:
      - typesense-data:/data
    env:
      TYPESENSE_API_KEY: "${TYPESENSE_API_KEY}"
      TYPESENSE_DATA_DIR: /data

# Expose Docker services via Caddy at a subdomain (no Go app needed)
# service_routes:
#   - subdomain: search             # → search.example.com → localhost:8108
#     port: 8108

# Applications (each gets a subdomain + systemd service)
# The root domain (example.com) automatically returns HTTP 200 OK.
apps:
  api:
    subdomain: api                  # → api.example.com
    path: ./apps/api                # source directory
    build:
      type: go
      main: ./cmd/server
      output: api
    deploy:
      port: 8081                    # internal port (Caddy proxies to this)
      env:
        DATABASE_URL: "postgres://platform:${POSTGRES_PASSWORD}@localhost:5432/api?sslmode=disable"

  web:
    subdomain: www                  # → www.example.com
    path: ./apps/web                # source directory containing package.json
    build:
      type: static
      command: "npm install && npm run build"
      dir: dist                     # output directory after build
      env:
        VITE_API_URL: "https://api.example.com"
    deploy:
      type: static                  # Caddy serves files directly (no systemd service)
`

	if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	printSuccess(fmt.Sprintf("Created %s", configPath))
	printInfo("Next steps:")
	fmt.Println("  1. Edit the config to match your project structure")
	fmt.Println("  2. Run 'gotzer auth' to set your Hetzner API token")
	fmt.Println("  3. Run 'gotzer platform provision' to create the server")
	fmt.Println("  4. Run 'gotzer platform deploy' to deploy all apps")

	return nil
}

func runPlatformProvision(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg, err := config.LoadPlatform(platformCfgFile)
	if err != nil {
		return err
	}

	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}

	hc := hetzner.NewClient(globalCfg.Token)

	existing, err := hc.GetServer(ctx, cfg.Server.Name)
	if err != nil {
		return err
	}

	var serverIP string
	if existing != nil {
		if !platformProvisionUpdate {
			return fmt.Errorf("server %s already exists (IP: %s). Use --update to sync configuration",
				cfg.Server.Name, existing.PublicNet.IPv4.IP.String())
		}
		serverIP = existing.PublicNet.IPv4.IP.String()
		printInfo(fmt.Sprintf("Using existing server %s (%s)", cfg.Server.Name, serverIP))
	} else {
		var sshKeys []string
		if platformSSHKeyName != "" {
			sshKeys = []string{platformSSHKeyName}
		} else {
			keys, err := hc.ListSSHKeys(ctx)
			if err != nil {
				return fmt.Errorf("failed to list SSH keys: %w", err)
			}
			if len(keys) == 0 {
				return fmt.Errorf("no SSH keys found in Hetzner. Add one in the Cloud Console")
			}
			sshKeys = []string{keys[0].Name}
			printInfo(fmt.Sprintf("Using SSH key: %s", keys[0].Name))
		}

		printInfo(fmt.Sprintf("Creating server %s (%s in %s)...",
			cfg.Server.Name, cfg.Server.Type, cfg.Server.Location))

		server, err := hc.CreateServer(ctx, hetzner.ServerOpts{
			Name:        cfg.Server.Name,
			Location:    cfg.Server.Location,
			ServerType:  cfg.Server.Type,
			Image:       cfg.Server.Image,
			SSHKeyNames: sshKeys,
		})
		if err != nil {
			return err
		}
		serverIP = server.PublicNet.IPv4.IP.String()
		printSuccess(fmt.Sprintf("Server created! IP: %s", serverIP))
	}

	// Wait for SSH
	printInfo("Waiting for SSH...")
	if err := ssh.WaitForSSH(ctx, serverIP, 2*time.Minute); err != nil {
		return fmt.Errorf("SSH not available: %w", err)
	}
	printSuccess("SSH is ready")

	// Connect
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	// Run platform setup
	prov := provision.NewPlatformProvisioner(cfg, sshClient)
	if err := prov.Setup(ctx); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Platform ready at %s!", cfg.Domain))
	printInfo(fmt.Sprintf("Server IP: %s", serverIP))
	printInfo("⚠️  Remember to set DNS records:")
	fmt.Printf("   %s → %s  (root domain)\n", cfg.Domain, serverIP)
	for _, name := range cfg.SortedAppNames() {
		app := cfg.Apps[name]
		fmt.Printf("   %s.%s → %s\n", app.Subdomain, cfg.Domain, serverIP)
	}
	for _, route := range cfg.ServiceRoutes {
		fmt.Printf("   %s.%s → %s  (service route)\n", route.Subdomain, cfg.Domain, serverIP)
	}

	return nil
}

func runPlatformDeploy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg, err := config.LoadPlatform(platformCfgFile)
	if err != nil {
		return err
	}

	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}

	hc := hetzner.NewClient(globalCfg.Token)
	server, err := hc.GetServer(ctx, cfg.Server.Name)
	if err != nil {
		return err
	}
	if server == nil {
		return fmt.Errorf("server %s not found. Run 'gotzer platform provision' first", cfg.Server.Name)
	}

	serverIP := server.PublicNet.IPv4.IP.String()
	printInfo(fmt.Sprintf("Deploying to %s (%s)", cfg.Server.Name, serverIP))

	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	deployer := deploy.NewPlatformDeployer(cfg, sshClient)

	if platformDeployApp != "" {
		return deployer.DeployApp(ctx, platformDeployApp)
	}
	return deployer.DeployAll(ctx)
}

func runPlatformStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg, err := config.LoadPlatform(platformCfgFile)
	if err != nil {
		return err
	}

	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}

	hc := hetzner.NewClient(globalCfg.Token)
	server, err := hc.GetServer(ctx, cfg.Server.Name)
	if err != nil {
		return err
	}
	if server == nil {
		fmt.Println("❌ Server not found")
		fmt.Printf("   Run 'gotzer platform provision' to create %s\n", cfg.Server.Name)
		return nil
	}

	serverIP := server.PublicNet.IPv4.IP.String()

	fmt.Println("\n📊 Platform Status")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("  Domain:         %s\n", cfg.Domain)
	fmt.Printf("  Server:         %s\n", server.Name)
	fmt.Printf("  Status:         %s\n", server.Status)
	fmt.Printf("  IP:             %s\n", serverIP)
	fmt.Printf("  Type:           %s\n", server.ServerType.Name)
	fmt.Printf("  Location:       %s\n", server.Datacenter.Location.Name)

	// Try SSH for detailed status
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err == nil {
		defer sshClient.Close()

		fmt.Println("\n🌐 Reverse Proxy (Caddy)")
		fmt.Println("────────────────────────────────────")
		output, err := sshClient.Run(ctx, "systemctl is-active caddy 2>/dev/null || echo 'inactive'")
		if err == nil {
			fmt.Printf("  Caddy:          %s", output)
		}

		fmt.Println("\n📦 Applications")
		fmt.Println("────────────────────────────────────")
		for _, name := range cfg.SortedAppNames() {
			app := cfg.Apps[name]
			if app.Deploy.Type == "static" {
				fmt.Printf("  %-15s %s.%s  [static/caddy]\n", name, app.Subdomain, cfg.Domain)
				continue
			}
			output, err := sshClient.Run(ctx, fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'inactive'", name))
			status := "unknown"
			if err == nil {
				status = strings.TrimSpace(output)
			}
			fmt.Printf("  %-15s %s.%s → :%d  [%s]\n", name, app.Subdomain, cfg.Domain, app.Deploy.Port, status)
		}

		if len(cfg.ServiceRoutes) > 0 {
			fmt.Println("\n🔀 Service Routes")
			fmt.Println("────────────────────────────────────")
			for _, route := range cfg.ServiceRoutes {
				fmt.Printf("  %s.%s → localhost:%d\n", route.Subdomain, cfg.Domain, route.Port)
			}
		}

		// Docker services
		output, err = sshClient.Run(ctx, "docker ps --format '{{.Names}}: {{.Status}}' 2>/dev/null || echo 'Docker not running'")
		if err == nil && output != "" {
			fmt.Println("\n🐳 Docker Services")
			fmt.Println("────────────────────────────────────")
			fmt.Printf("  %s", output)
		}

		// Resources
		output, err = sshClient.Run(ctx, "df -h / | tail -1 | awk '{print $5}'")
		if err == nil {
			fmt.Println("\n💾 Resources")
			fmt.Println("────────────────────────────────────")
			fmt.Printf("  Disk Usage:     %s", output)
		}
		output, err = sshClient.Run(ctx, "free -h | grep Mem | awk '{print $3 \"/\" $2}'")
		if err == nil {
			fmt.Printf("  Memory Usage:   %s", output)
		}
	}

	fmt.Println()
	return nil
}
