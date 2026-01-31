package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/DawnKosmos/gotzer/internal/provision"
	"github.com/DawnKosmos/gotzer/internal/ssh"
	"github.com/spf13/cobra"
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision a new Hetzner server with all services",
	Long: `Creates a new Hetzner Cloud server and sets up:
  - Docker and Docker Compose
  - Application user and directories
  - Systemd service for your app
  - PostgreSQL, Typesense, and other Docker services (if enabled)
  - UFW firewall rules`,
	RunE: runProvision,
}

var sshKeyName string
var provisionUpdate bool

func init() {
	provisionCmd.Flags().StringVar(&sshKeyName, "ssh-key", "", "SSH key name in Hetzner (uses first available if not set)")
	provisionCmd.Flags().BoolVar(&provisionUpdate, "update", false, "Update an existing server (sync configuration and services)")
}

func runProvision(cmd *cobra.Command, args []string) error {
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

	// Create Hetzner client
	hc := hetzner.NewClient(globalCfg.Token)

	// Check if server already exists
	existing, err := hc.GetServer(ctx, cfg.Server.Name)
	if err != nil {
		return err
	}

	var serverIP string
	if existing != nil {
		if !provisionUpdate {
			return fmt.Errorf("server %s already exists (IP: %s). Use 'gotzer provision --update' to sync services",
				cfg.Server.Name, existing.PublicNet.IPv4.IP.String())
		}
		serverIP = existing.PublicNet.IPv4.IP.String()
		printInfo(fmt.Sprintf("Using existing server %s (IP: %s)...", cfg.Server.Name, serverIP))
	} else {
		// New server creation logic...
		// Get SSH key
		var sshKeys []string
		if sshKeyName != "" {
			sshKeys = []string{sshKeyName}
		} else {
			keys, err := hc.ListSSHKeys(ctx)
			if err != nil {
				return fmt.Errorf("failed to list SSH keys: %w", err)
			}
			if len(keys) == 0 {
				return fmt.Errorf("no SSH keys found")
			}
			sshKeys = []string{keys[0].Name}
			printInfo(fmt.Sprintf("Using SSH key: %s", keys[0].Name))
		}

		printInfo(fmt.Sprintf("Creating server %s (%s in %s)...",
			cfg.Server.Name, cfg.Server.Type, cfg.Server.Location))

		server, err := hc.CreateServer(ctx, hetzner.ServerOpts{
			Name:        cfg.Server.Name,
			Location:    cfg.Server.Location,
			ServerType:  cfg.Server.Type,
			Image:       cfg.Server.Image,
			SSHKeyNames: sshKeys,
		})
		if err != nil {
			return err
		}
		serverIP = server.PublicNet.IPv4.IP.String()
		printSuccess(fmt.Sprintf("Server created! IP: %s", serverIP))
	}

	// Wait for SSH to be available
	printInfo("Waiting for SSH to be available...")
	if err := ssh.WaitForSSH(ctx, serverIP, 2*time.Minute); err != nil {
		return fmt.Errorf("SSH not available: %w", err)
	}
	printSuccess("SSH is ready")

	// Connect via SSH
	sshKeyPath := config.ExpandPath(globalCfg.DefaultSSHKey)
	sshClient := ssh.NewClient(serverIP, "root", sshKeyPath)
	if err := sshClient.Connect(ctx); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	// Run provisioning
	prov := provision.NewProvisioner(cfg, sshClient)
	if err := prov.Setup(ctx); err != nil {
		return err
	}

	printSuccess("\n✅ Server ready! Run 'gotzer deploy' to deploy your app.")
	printInfo(fmt.Sprintf("Server IP: %s", serverIP))

	return nil
}

// findSSHKey looks for an SSH key file
func findSSHKey() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
