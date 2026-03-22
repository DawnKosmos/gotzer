package provision

import (
	"context"
	"fmt"
	"strings"

	"github.com/DawnKosmos/gotzer/internal/caddy"
	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/docker"
	"github.com/DawnKosmos/gotzer/internal/ssh"
)

// PlatformProvisioner handles multi-app server setup
type PlatformProvisioner struct {
	Config    *config.PlatformConfig
	SSHClient *ssh.Client
}

// NewPlatformProvisioner creates a new platform provisioner
func NewPlatformProvisioner(cfg *config.PlatformConfig, sshClient *ssh.Client) *PlatformProvisioner {
	return &PlatformProvisioner{
		Config:    cfg,
		SSHClient: sshClient,
	}
}

// Setup configures the server for hosting multiple applications
func (p *PlatformProvisioner) Setup(ctx context.Context) error {
	fmt.Println("\n🔧 Setting up platform server...")

	// Step 1: Update system
	fmt.Println("\n📦 Updating system packages...")
	if _, err := p.SSHClient.Run(ctx, "sudo apt-get update && sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && sudo apt-get install -y libcap2-bin curl"); err != nil {
		return fmt.Errorf("failed to update system: %w", err)
	}

	// Step 2: Install Docker
	fmt.Println("\n🐳 Installing Docker...")
	dockerScript := `
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh && rm get-docker.sh
    sudo systemctl enable docker
    sudo systemctl start docker
fi
`
	if _, err := p.SSHClient.Run(ctx, dockerScript); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// Step 3: Install Caddy
	fmt.Println("\n🌐 Installing Caddy reverse proxy...")
	caddyScript := `
if ! command -v caddy &> /dev/null; then
    sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
    sudo apt-get update
    sudo apt-get install -y caddy
fi
`
	if _, err := p.SSHClient.Run(ctx, caddyScript); err != nil {
		return fmt.Errorf("failed to install Caddy: %w", err)
	}

	// Step 4: Create app user
	fmt.Println("\n👤 Creating app user...")
	userScript := `
sudo useradd -m -s /bin/bash app 2>/dev/null || true
sudo usermod -aG docker app
`
	if _, err := p.SSHClient.Run(ctx, userScript); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Step 5: Create per-app directories
	fmt.Println("\n📁 Creating application directories...")
	for _, name := range p.Config.SortedAppNames() {
		remotePath := p.Config.AppRemotePath(name)
		dirScript := fmt.Sprintf("sudo mkdir -p %s && sudo chown -R app:app %s", remotePath, remotePath)
		if _, err := p.SSHClient.Run(ctx, dirScript); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", name, err)
		}
		fmt.Printf("  → %s\n", remotePath)
	}

	// Step 6: Create per-app systemd services
	fmt.Println("\n⚙️  Creating systemd services...")
	for _, name := range p.Config.SortedAppNames() {
		app := p.Config.Apps[name]
		if err := p.createAppService(ctx, name, app); err != nil {
			return fmt.Errorf("failed to create service for %s: %w", name, err)
		}
		fmt.Printf("  → %s.service\n", name)
	}

	// Step 7: Setup shared Docker services
	if p.hasDockerServices() {
		fmt.Println("\n🐳 Setting up shared Docker services...")
		if err := p.setupDockerServices(ctx); err != nil {
			return fmt.Errorf("failed to setup Docker services: %w", err)
		}
	}

	// Step 8: Generate and install Caddyfile
	fmt.Println("\n🌐 Configuring Caddy reverse proxy...")
	if err := p.configureCaddy(ctx); err != nil {
		return fmt.Errorf("failed to configure Caddy: %w", err)
	}

	// Step 9: Configure firewall
	fmt.Println("\n🔒 Configuring firewall...")
	firewallScript := `
sudo apt-get install -y ufw
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow http
sudo ufw allow https
echo "y" | sudo ufw enable
`
	if _, err := p.SSHClient.Run(ctx, firewallScript); err != nil {
		fmt.Printf("  ⚠ Firewall setup warning: %v\n", err)
	}

	fmt.Println("\n✅ Platform setup complete!")
	fmt.Printf("   Domain: %s\n", p.Config.Domain)
	for _, name := range p.Config.SortedAppNames() {
		app := p.Config.Apps[name]
		fmt.Printf("   → %s.%s (port %d)\n", app.Subdomain, p.Config.Domain, app.Deploy.Port)
	}
	fmt.Println("\n   Run 'gotzer platform deploy' to deploy your apps.")

	return nil
}

func (p *PlatformProvisioner) createAppService(ctx context.Context, name string, app *config.AppConfig) error {
	remotePath := p.Config.AppRemotePath(name)
	binaryPath := fmt.Sprintf("%s/%s", remotePath, app.Build.Output)

	// Build exec command
	execCmd := binaryPath
	if len(app.Deploy.Command) > 0 {
		execCmd = fmt.Sprintf("%s %s", execCmd, strings.Join(app.Deploy.Command, " "))
	}

	// Build environment lines — always inject PORT
	var envLines []string
	envLines = append(envLines, fmt.Sprintf("Environment=PORT=%d", app.Deploy.Port))
	for k, v := range app.Deploy.Env {
		envLines = append(envLines, fmt.Sprintf("Environment=%s=%s", k, v))
	}
	envSection := strings.Join(envLines, "\n")

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s (%s.%s)
After=network.target docker.service

[Service]
Type=simple
User=app
Group=app
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5
%s

[Install]
WantedBy=multi-user.target`, name, app.Subdomain, p.Config.Domain, remotePath, execCmd, envSection)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	cmd := fmt.Sprintf(`echo '%s' | sudo tee %s > /dev/null`, serviceContent, servicePath)
	if _, err := p.SSHClient.Run(ctx, cmd); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	if _, err := p.SSHClient.Run(ctx, "sudo systemctl daemon-reload"); err != nil {
		return err
	}
	if _, err := p.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl enable %s", name)); err != nil {
		return err
	}

	return nil
}

func (p *PlatformProvisioner) hasDockerServices() bool {
	s := p.Config.Services
	if s.Postgres != nil && s.Postgres.Enabled {
		return true
	}
	if s.Typesense != nil && s.Typesense.Enabled {
		return true
	}
	if s.Redis != nil && s.Redis.Enabled {
		return true
	}
	if s.Centrifugo != nil && s.Centrifugo.Enabled {
		return true
	}
	return len(s.Custom) > 0
}

func (p *PlatformProvisioner) setupDockerServices(ctx context.Context) error {
	composeContent := docker.GenerateComposeFromServices(&p.Config.Services)
	if composeContent == "" {
		return nil
	}

	// Shared services directory (server-level, not per-app)
	servicesDir := "/opt/services"
	if _, err := p.SSHClient.Run(ctx, fmt.Sprintf("sudo mkdir -p %s", servicesDir)); err != nil {
		return fmt.Errorf("failed to create services directory: %w", err)
	}

	composePath := fmt.Sprintf("%s/docker-compose.yml", servicesDir)
	cmd := fmt.Sprintf(`echo '%s' | sudo tee %s > /dev/null`, composeContent, composePath)
	if _, err := p.SSHClient.Run(ctx, cmd); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	if _, err := p.SSHClient.Run(ctx, fmt.Sprintf("cd %s && sudo docker compose up -d", servicesDir)); err != nil {
		return fmt.Errorf("failed to start Docker services: %w", err)
	}

	return nil
}

func (p *PlatformProvisioner) configureCaddy(ctx context.Context) error {
	caddyfile := caddy.GenerateCaddyfile(p.Config)

	cmd := fmt.Sprintf(`echo '%s' | sudo tee /etc/caddy/Caddyfile > /dev/null`, caddyfile)
	if _, err := p.SSHClient.Run(ctx, cmd); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	// Reload Caddy, falling back to restart
	if _, err := p.SSHClient.Run(ctx, "sudo systemctl reload caddy"); err != nil {
		if _, err := p.SSHClient.Run(ctx, "sudo systemctl restart caddy"); err != nil {
			return fmt.Errorf("failed to restart Caddy: %w", err)
		}
	}

	return nil
}
