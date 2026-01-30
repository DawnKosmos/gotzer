package cli

import (
	"context"
	"fmt"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH into the server",
	Long:  `Opens an interactive SSH session to your Hetzner server.`,
	RunE:  runSSH,
}

func runSSH(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("server %s not found", cfg.Server.Name)
	}

	serverIP := server.PublicNet.IPv4.IP.String()
	printInfo(fmt.Sprintf("Connecting to %s (%s)...", cfg.Server.Name, serverIP))

	// Connect via SSH
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	return sshClient.Shell()
}
