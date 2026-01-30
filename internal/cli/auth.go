package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var authToken string

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Configure Hetzner API authentication",
	Long: `Configure your Hetzner Cloud API token.

You can get an API token from the Hetzner Cloud Console:
https://console.hetzner.cloud/projects -> Select project -> Security -> API Tokens`,
	RunE: runAuth,
}

func init() {
	authCmd.Flags().StringVar(&authToken, "token", "", "Hetzner API token (will prompt if not provided)")
}

type globalConfig struct {
	Token         string `yaml:"token"`
	DefaultSSHKey string `yaml:"default_ssh_key,omitempty"`
}

func runAuth(cmd *cobra.Command, args []string) error {
	token := authToken

	// Prompt for token if not provided
	if token == "" {
		fmt.Print("Enter your Hetzner API token: ")
		if term.IsTerminal(int(syscall.Stdin)) {
			// Read password without echo
			tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read token: %w", err)
			}
			fmt.Println() // New line after password input
			token = string(tokenBytes)
		} else {
			// Non-interactive mode
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read token: %w", err)
			}
			token = strings.TrimSpace(input)
		}
	}

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Create config directory
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	configPath := filepath.Join(configDir, "config.yaml")
	config := globalConfig{
		Token:         token,
		DefaultSSHKey: "~/.ssh/id_ed25519",
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	printSuccess(fmt.Sprintf("Token saved to %s", configPath))
	return nil
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".gotzer"), nil
}

func loadGlobalConfig() (*globalConfig, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not authenticated. Run 'gotzer auth' first")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config globalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}
