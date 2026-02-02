package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DawnKosmos/gotzer/internal/build"
	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/DawnKosmos/gotzer/internal/systemd"
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
		cfg.Build.Type,
		cfg.Build.Main,
		cfg.Build.Output,
		cfg.Server.GOARCH(),
	)
	builder.Command = cfg.Build.Command
	builder.Dir = cfg.Build.Dir
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

	// Step 3: Upload the application
	fmt.Println("\n📤 Uploading application...")
	remotePath := cfg.Deploy.RemotePath

	if cfg.Deploy.Type == "static" {
		if err := d.SSHClient.UploadDir(ctx, binaryPath, remotePath); err != nil {
			return fmt.Errorf("static upload failed: %w", err)
		}
		fmt.Printf("  → Uploaded directory to %s\n", remotePath)

		// Set permissions
		_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo chown -R %s:%s %s && sudo chmod -R 755 %s",
			cfg.Deploy.User, cfg.Deploy.User, remotePath, remotePath))
		if err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		fmt.Println("\n🎉 Static deployment complete!")
		return nil
	}

	// Legacy Go upload logic
	remoteBinaryPath := filepath.Join(remotePath, cfg.Build.Output)

	// Upload to temp location first
	tempPath := fmt.Sprintf("/tmp/%s", cfg.Build.Output)
	if err := d.SSHClient.Upload(ctx, binaryPath, tempPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Move to final location with sudo and set permissions
	_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo mv %s %s && sudo chmod +x %s && sudo setcap 'cap_net_bind_service=+ep' %s",
		tempPath, remoteBinaryPath, remoteBinaryPath, remoteBinaryPath))
	if err != nil {
		return fmt.Errorf("failed to move or configure binary: %w", err)
	}

	fmt.Printf("  → Uploaded to %s\n", remoteBinaryPath)

	// Step 4: Update service configuration
	fmt.Println("\n⚙️ Updating service configuration...")
	if err := systemd.Configure(ctx, d.SSHClient, d.Config); err != nil {
		return fmt.Errorf("failed to update service config: %w", err)
	}

	// Step 5: Start the service
	fmt.Println("\n🚀 Starting service...")
	_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl start %s", cfg.Deploy.ServiceName))
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Step 6: Check service status
	fmt.Println("\n✅ Checking status...")
	output, err := d.SSHClient.Run(ctx, fmt.Sprintf("systemctl is-active %s", cfg.Deploy.ServiceName))
	if err != nil {
		// If it failed, try to get logs to show why
		logs, logErr := d.SSHClient.Run(ctx, fmt.Sprintf("sudo journalctl -u %s -n 10 --no-pager", cfg.Deploy.ServiceName))
		if logErr == nil {
			fmt.Printf("\n❌ Service failed to start. Last 10 lines of logs:\n%s\n", logs)
		}
		return fmt.Errorf("service failed to start (status %v): %w", err, err)
	}
	fmt.Printf("  → Service status: %s", output)

	fmt.Println("\n🎉 Deployment complete!")
	return nil
}
