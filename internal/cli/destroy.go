package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/hetzner"
	"github.com/spf13/cobra"
)

var destroyForce bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Hetzner server",
	Long: `Permanently deletes the Hetzner server and all associated data.

WARNING: This action cannot be undone. All data on the server will be lost.`,
	RunE: runDestroy,
}

func init() {
	destroyCmd.Flags().BoolVarP(&destroyForce, "force", "f", false, "Skip confirmation prompt")
}

func runDestroy(cmd *cobra.Command, args []string) error {
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
		printInfo(fmt.Sprintf("Server %s does not exist", cfg.Server.Name))
		return nil
	}

	serverIP := server.PublicNet.IPv4.IP.String()

	// Confirmation
	if !destroyForce {
		fmt.Printf("⚠️  WARNING: This will permanently destroy server '%s' (%s)\n", cfg.Server.Name, serverIP)
		fmt.Printf("   All data will be lost. This cannot be undone.\n\n")
		fmt.Print("Type the server name to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		if strings.TrimSpace(input) != cfg.Server.Name {
			fmt.Println("Aborted.")
			return nil
		}
	}

	printInfo(fmt.Sprintf("Destroying server %s...", cfg.Server.Name))

	if err := hc.DeleteServer(ctx, cfg.Server.Name); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Server %s has been destroyed", cfg.Server.Name))
	return nil
}
