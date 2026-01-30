package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var logsFollow bool
var logsLines int

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View application logs",
	Long:  `Streams logs from your application's systemd service using journalctl.`,
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "Number of lines to show")
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

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

	// Connect via SSH
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	// Build journalctl command
	journalCmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager", cfg.Deploy.ServiceName, logsLines)
	if logsFollow {
		journalCmd += " -f"
	}

	printInfo(fmt.Sprintf("Streaming logs from %s...", cfg.Deploy.ServiceName))

	return sshClient.RunInteractive(ctx, journalCmd)
}
