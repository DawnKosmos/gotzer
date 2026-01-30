package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DawnKosmos/gotzer/internal/build"
	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/ssh"
)

// Deployer handles application deployment
type Deployer struct {
	Config    *config.Config
	SSHClient *ssh.Client
}

// NewDeployer creates a new deployer
func NewDeployer(cfg *config.Config, sshClient *ssh.Client) *Deployer {
	return &Deployer{
		Config:    cfg,
		SSHClient: sshClient,
	}
}

// Deploy builds and deploys the application
func (d *Deployer) Deploy(ctx context.Context) error {
	cfg := d.Config

	// Step 1: Build the binary
	fmt.Println("\n📦 Building application...")
	builder := build.NewBuilder(
		cfg.Build.Main,
		cfg.Build.Output,
		cfg.Server.GOARCH(),
	)
	builder.LDFlags = cfg.Build.LDFlags
	builder.Env = cfg.Build.Env

	binaryPath, err := builder.Build(ctx)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(binaryPath)) // Cleanup temp dir

	// Step 2: Stop the service
	fmt.Println("\n🛑 Stopping service...")
	_, stopErr := d.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl stop %s 2>/dev/null || true", cfg.Deploy.ServiceName))
	if stopErr != nil {
		fmt.Printf("  ⚠ Note: %v\n", stopErr)
	}

	// Step 3: Upload the binary
	fmt.Println("\n📤 Uploading binary...")
	remoteBinaryPath := filepath.Join(cfg.Deploy.RemotePath, cfg.Build.Output)

	// Upload to temp location first
	tempPath := fmt.Sprintf("/tmp/%s", cfg.Build.Output)
	if err := d.SSHClient.Upload(ctx, binaryPath, tempPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Move to final location with sudo
	_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo mv %s %s && sudo chmod +x %s",
		tempPath, remoteBinaryPath, remoteBinaryPath))
	if err != nil {
		return fmt.Errorf("failed to move binary: %w", err)
	}

	fmt.Printf("  → Uploaded to %s\n", remoteBinaryPath)

	// Step 4: Start the service
	fmt.Println("\n🚀 Starting service...")
	_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl start %s", cfg.Deploy.ServiceName))
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Step 5: Check service status
	fmt.Println("\n✅ Checking status...")
	output, err := d.SSHClient.Run(ctx, fmt.Sprintf("systemctl is-active %s", cfg.Deploy.ServiceName))
	if err != nil {
		return fmt.Errorf("service failed to start: %w", err)
	}
	fmt.Printf("  → Service status: %s", output)

	fmt.Println("\n🎉 Deployment complete!")
	return nil
}
