package cli

import (
	"context"
	"fmt"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server and application status",
	Long:  `Displays the current status of your Hetzner server and running services.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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
		fmt.Println("❌ Server not found")
		fmt.Printf("   Run 'gotzer provision' to create %s\n", cfg.Server.Name)
		return nil
	}

	serverIP := server.PublicNet.IPv4.IP.String()

	fmt.Println("\n📊 Server Status")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("  Name:           %s\n", server.Name)
	fmt.Printf("  Status:         %s\n", server.Status)
	fmt.Printf("  IP:             %s\n", serverIP)
	fmt.Printf("  Type:           %s\n", server.ServerType.Name)
	fmt.Printf("  Location:       %s\n", server.Datacenter.Location.Name)
	fmt.Printf("  Image:          %s\n", server.Image.Name)

	// Try to get service status via SSH
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err == nil {
		defer sshClient.Close()

		fmt.Println("\n📦 Application Status")
		fmt.Println("────────────────────────────────────")

		// Service status
		output, err := sshClient.Run(ctx, fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'inactive'", cfg.Deploy.ServiceName))
		if err == nil {
			fmt.Printf("  %s:  %s", cfg.Deploy.ServiceName, output)
		}

		// Docker services
		output, err = sshClient.Run(ctx, "docker ps --format '{{.Names}}: {{.Status}}' 2>/dev/null || echo 'Docker not running'")
		if err == nil && output != "" {
			fmt.Println("\n🐳 Docker Services")
			fmt.Println("────────────────────────────────────")
			fmt.Printf("  %s", output)
		}

		// Disk usage
		output, err = sshClient.Run(ctx, "df -h / | tail -1 | awk '{print $5}'")
		if err == nil {
			fmt.Println("\n💾 Resources")
			fmt.Println("────────────────────────────────────")
			fmt.Printf("  Disk Usage:     %s", output)
		}

		// Memory
		output, err = sshClient.Run(ctx, "free -h | grep Mem | awk '{print $3 \"/\" $2}'")
		if err == nil {
			fmt.Printf("  Memory Usage:   %s", output)
		}
	}

	fmt.Println()
	return nil
}
