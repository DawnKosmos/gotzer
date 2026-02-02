package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/docker"
	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Manage local development environment",
	Long:  `Manage local Docker services for development.`,
}

var localUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start local services",
	RunE:  runLocalUp,
}

var localDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove local services",
	RunE:  runLocalDown,
}

var localGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate docker-compose.yml only",
	RunE:  runLocalGen,
}

func init() {
	localCmd.AddCommand(localUpCmd)
	localCmd.AddCommand(localDownCmd)
	localCmd.AddCommand(localGenCmd)
}

func runLocalUp(cmd *cobra.Command, args []string) error {
	printInfo("Starting local services...")

	// 1. Generate docker-compose.yml
	if err := generateLocalCompose(); err != nil {
		return err
	}

	// 2. Run docker compose up -d
	c := exec.Command("docker", "compose", "up", "-d")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	printSuccess("Local services started!")
	return nil
}

func runLocalDown(cmd *cobra.Command, args []string) error {
	printInfo("Stopping local services...")

	c := exec.Command("docker", "compose", "down")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	printSuccess("Local services stopped")
	return nil
}

func runLocalGen(cmd *cobra.Command, args []string) error {
	printInfo("Generating docker-compose.yml...")
	if err := generateLocalCompose(); err != nil {
		return err
	}
	printSuccess("Generated docker-compose.yml")
	return nil
}

func generateLocalCompose() error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Generate content
	content := docker.GenerateCompose(cfg)

	// Write file
	if err := os.WriteFile("docker-compose.yml", []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	return nil
}
