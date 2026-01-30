package cli

import (
	"context"
	"fmt"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/deploy"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy the Go application",
	Long: `Builds the Go application for the target architecture and deploys it:
  1. Cross-compiles for Linux (ARM64 or AMD64)
  2. Stops the systemd service
  3. Uploads the binary via SCP
  4. Starts the systemd service

This is the default command and only updates the Go app, not Docker services.`,
	RunE: runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configs
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}

	// Get server info
	hc := hetzner.NewClient(globalCfg.Token)
	server, err := hc.GetServer(ctx, cfg.Server.Name)
	if err != nil {
		return err
	}
	if server == nil {
		return fmt.Errorf("server %s not found. Run 'gotzer provision' first", cfg.Server.Name)
	}

	serverIP := server.PublicNet.IPv4.IP.String()
	printInfo(fmt.Sprintf("Deploying to %s (%s)", cfg.Server.Name, serverIP))

	// Connect via SSH
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	// Deploy
	deployer := deploy.NewDeployer(cfg, sshClient)
	return deployer.Deploy(ctx)
}
