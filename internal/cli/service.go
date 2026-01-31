package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the application service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceCmd(cmd.Context(), "stop")
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the application service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceCmd(cmd.Context(), "start")
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the application service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceCmd(cmd.Context(), "restart")
	},
}

func runServiceCmd(ctx context.Context, action string) error {
	cfg, err := config.Load(cfgFile)
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
		return fmt.Errorf("server %s not found", cfg.Server.Name)
	}

	serverIP := server.PublicNet.IPv4.IP.String()
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return err
	}
	defer sshClient.Close()

	printInfo(fmt.Sprintf("%s service %s...", strings.Title(action), cfg.Deploy.ServiceName))
	_, err = sshClient.Run(ctx, fmt.Sprintf("sudo systemctl %s %s", action, cfg.Deploy.ServiceName))
	if err != nil {
		return fmt.Errorf("failed to %s service: %w", action, err)
	}

	printSuccess(fmt.Sprintf("Service %s complete", action))
	return nil
}
