package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "gotzer",
	Short: "Deploy Go applications to Hetzner Cloud",
	Long: `Gotzer is a CLI tool for deploying Go applications to Hetzner Cloud servers.

It handles:
  - Server provisioning with Docker services (PostgreSQL, Typesense, etc.)
  - Go cross-compilation for ARM64 and AMD64
  - Direct deployment via SSH (stop → upload → start)

Quick start:
  gotzer init       # Create .gotzer.yaml config
  gotzer auth       # Configure Hetzner API token
  gotzer provision  # Create server with services
  gotzer deploy     # Build and deploy your app`,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .gotzer.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(destroyCmd)
}

func printSuccess(msg string) {
	fmt.Fprintf(os.Stdout, "✓ %s\n", msg)
}

func printInfo(msg string) {
	fmt.Fprintf(os.Stdout, "→ %s\n", msg)
}

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
}
