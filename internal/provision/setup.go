package provision

import (
	"context"
	"fmt"
	"regexp"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/docker"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/DawnKosmos/gotzer/internal/systemd"
)

// Provisioner handles server setup
type Provisioner struct {
	Config    *config.Config
	SSHClient *ssh.Client
}

// NewProvisioner creates a new provisioner
func NewProvisioner(cfg *config.Config, sshClient *ssh.Client) *Provisioner {
	return &Provisioner{
		Config:    cfg,
		SSHClient: sshClient,
	}
}

// Setup configures a new server
func (p *Provisioner) Setup(ctx context.Context) error {
	cfg := p.Config

	fmt.Println("\n🔧 Setting up server...")

	// Step 0: Check free ports
	if len(cfg.Server.FreePorts) > 0 {
		p.checkFreePorts(ctx)
	}

	// Step 1: Update system
	fmt.Println("\n📦 Updating system packages...")
	if _, err := p.SSHClient.Run(ctx, "sudo apt-get update && sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && sudo apt-get install -y libcap2-bin"); err != nil {
		return fmt.Errorf("failed to update system: %w", err)
	}

	// Step 2: Install Docker
	fmt.Println("\n🐳 Installing Docker...")
	dockerScript := `
curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh
sudo systemctl enable docker
sudo systemctl start docker
`
	if _, err := p.SSHClient.Run(ctx, dockerScript); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// Step 3: Create app user
	fmt.Println("\n👤 Creating app user...")
	userScript := fmt.Sprintf(`
sudo useradd -m -s /bin/bash %s 2>/dev/null || true
sudo usermod -aG docker %s
`, cfg.Deploy.User, cfg.Deploy.User)
	if _, err := p.SSHClient.Run(ctx, userScript); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Step 4: Create app directory
	fmt.Println("\n📁 Creating application directory...")
	dirScript := fmt.Sprintf(`
sudo mkdir -p %s
sudo chown -R %s:%s %s
`, cfg.Deploy.RemotePath, cfg.Deploy.User, cfg.Deploy.User, cfg.Deploy.RemotePath)
	if _, err := p.SSHClient.Run(ctx, dirScript); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Step 5: Create systemd service
	fmt.Println("\n⚙️ Creating systemd service...")
	if err := p.createSystemdService(ctx); err != nil {
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	// Step 6: Setup Docker services
	if p.hasDockerServices() {
		fmt.Println("\n🐳 Setting up Docker services...")
		if err := p.setupDockerServices(ctx); err != nil {
			return fmt.Errorf("failed to setup Docker services: %w", err)
		}
	}

	// Step 7: Configure firewall
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

	fmt.Println("\n✅ Server setup complete!")
	return nil
}

// createSystemdService creates the systemd unit file
func (p *Provisioner) createSystemdService(ctx context.Context) error {
	return systemd.Configure(ctx, p.SSHClient, p.Config)
}

// hasDockerServices checks if any Docker services are enabled
func (p *Provisioner) hasDockerServices() bool {
	services := p.Config.Services
	if services.Postgres != nil && services.Postgres.Enabled {
		return true
	}
	if services.Typesense != nil && services.Typesense.Enabled {
		return true
	}
	if services.Redis != nil && services.Redis.Enabled {
		return true
	}
	return len(services.Custom) > 0
}

// setupDockerServices creates and starts Docker Compose services
func (p *Provisioner) setupDockerServices(ctx context.Context) error {
	cfg := p.Config

	composeContent := docker.GenerateCompose(cfg)
	if composeContent == "" {
		return nil
	}

	// Create services directory
	servicesDir := fmt.Sprintf("%s/services", cfg.Deploy.RemotePath)
	if _, err := p.SSHClient.Run(ctx, fmt.Sprintf("sudo mkdir -p %s", servicesDir)); err != nil {
		return fmt.Errorf("failed to create services directory: %w", err)
	}

	// Write docker-compose.yml
	composePath := fmt.Sprintf("%s/docker-compose.yml", servicesDir)
	cmd := fmt.Sprintf(`echo '%s' | sudo tee %s > /dev/null`, composeContent, composePath)
	if _, err := p.SSHClient.Run(ctx, cmd); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	// Start services
	if _, err := p.SSHClient.Run(ctx, fmt.Sprintf("cd %s && sudo docker compose up -d", servicesDir)); err != nil {
		return fmt.Errorf("failed to start Docker services: %w", err)
	}

	return nil
}

func (p *Provisioner) checkFreePorts(ctx context.Context) {
	fmt.Println("\n🔍 Checking free ports...")
	out, err := p.SSHClient.Run(ctx, "ss -tuln")
	if err != nil {
		fmt.Printf("  ⚠ Could not check free ports: %v\n", err)
		return
	}

	for _, port := range p.Config.Server.FreePorts {
		re := regexp.MustCompile(fmt.Sprintf(":%d(\\s|$)", port))
		if re.MatchString(out) {
			fmt.Printf("  ⚠ Warning: Port %d is already in use!\n", port)
		}
	}
}
