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

// PlatformDeployer handles multi-app deployment
type PlatformDeployer struct {
	Config    *config.PlatformConfig
	SSHClient *ssh.Client
}

// NewPlatformDeployer creates a new platform deployer
func NewPlatformDeployer(cfg *config.PlatformConfig, sshClient *ssh.Client) *PlatformDeployer {
	return &PlatformDeployer{
		Config:    cfg,
		SSHClient: sshClient,
	}
}

// DeployAll builds and deploys all applications
func (d *PlatformDeployer) DeployAll(ctx context.Context) error {
	for _, name := range d.Config.SortedAppNames() {
		fmt.Printf("\n━━━ Deploying %s ━━━\n", name)
		if err := d.DeployApp(ctx, name); err != nil {
			return fmt.Errorf("failed to deploy %s: %w", name, err)
		}
	}
	fmt.Println("\n🎉 All apps deployed!")
	return nil
}

// DeployApp builds and deploys a single application
func (d *PlatformDeployer) DeployApp(ctx context.Context, appName string) error {
	app, ok := d.Config.Apps[appName]
	if !ok {
		return fmt.Errorf("app %q not found in platform config", appName)
	}

	// Resolve remote path: static apps go to /var/www/<name>, services to /opt/apps/<name>
	var remotePath string
	if app.Deploy.Type == "static" {
		remotePath = app.Deploy.RemotePath
	} else {
		remotePath = d.Config.AppRemotePath(appName)
	}

	// Step 1: Build
	fmt.Println("\n📦 Building application...")
	builder := build.NewBuilder(
		app.Build.Type,
		app.Build.Main,
		app.Build.Output,
		d.Config.Server.GOARCH(),
	)
	builder.LDFlags = app.Build.LDFlags
	builder.Command = app.Build.Command
	builder.Env = app.Build.Env

	// Set working directory to app's source path
	if app.Build.Type == "static" {
		// For static builds, Dir is the output directory
		if app.Path != "." && app.Build.Dir != "" {
			builder.Dir = filepath.Join(app.Path, app.Build.Dir)
		} else if app.Build.Dir != "" {
			builder.Dir = app.Build.Dir
		}
	} else {
		// For Go builds, Dir is the working directory
		if app.Path != "." {
			builder.Dir = app.Path
		}
	}

	binaryPath, err := builder.Build(ctx)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	if app.Build.Type != "static" {
		defer os.RemoveAll(filepath.Dir(binaryPath))
	}

	if app.Deploy.Type == "static" {
		// Step 2 (static): upload files directly — Caddy serves them, no systemd needed
		fmt.Println("\n📤 Uploading static files...")
		if err := d.SSHClient.UploadDir(ctx, binaryPath, remotePath); err != nil {
			return fmt.Errorf("static upload failed: %w", err)
		}
		_, _ = d.SSHClient.Run(ctx, fmt.Sprintf("sudo chown -R app:app %s && sudo chmod -R 755 %s", remotePath, remotePath))
		fmt.Printf("  → Uploaded to %s\n", remotePath)
		fmt.Printf("✅ %s deployed successfully! (served by Caddy at %s.%s)\n", appName, app.Subdomain, d.Config.Domain)
		return nil
	}

	// Step 2: Stop service
	fmt.Println("\n🛑 Stopping service...")
	_, _ = d.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl stop %s 2>/dev/null || true", appName))

	// Step 3: Upload binary
	fmt.Println("\n📤 Uploading application...")
	remoteBinaryPath := filepath.Join(remotePath, app.Build.Output)
	tempPath := fmt.Sprintf("/tmp/%s", app.Build.Output)

	if err := d.SSHClient.Upload(ctx, binaryPath, tempPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	_, err = d.SSHClient.Run(ctx, fmt.Sprintf(
		"sudo mv %s %s && sudo chmod +x %s && sudo chown app:app %s && sudo setcap 'cap_net_bind_service=+ep' %s",
		tempPath, remoteBinaryPath, remoteBinaryPath, remoteBinaryPath, remoteBinaryPath))
	if err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}
	fmt.Printf("  → Uploaded to %s\n", remoteBinaryPath)

	// Step 4: Start service
	fmt.Println("\n🚀 Starting service...")
	_, err = d.SSHClient.Run(ctx, fmt.Sprintf("sudo systemctl start %s", appName))
	if err != nil {
		logs, logErr := d.SSHClient.Run(ctx, fmt.Sprintf("sudo journalctl -u %s -n 10 --no-pager", appName))
		if logErr == nil {
			fmt.Printf("\n❌ Service failed to start. Last 10 lines of logs:\n%s\n", logs)
		}
		return fmt.Errorf("service failed to start: %w", err)
	}

	// Step 5: Verify
	output, err := d.SSHClient.Run(ctx, fmt.Sprintf("systemctl is-active %s", appName))
	if err == nil {
		fmt.Printf("  → Service status: %s", output)
	}

	fmt.Printf("✅ %s deployed successfully!\n", appName)
	return nil
}
