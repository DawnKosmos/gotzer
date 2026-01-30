package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/DawnKosmos/gotzer/internal/config"
	"github.com/DawnKosmos/gotzer/internal/ssh"
)

// Configure updates or creates the systemd service file and reloads systemd
func Configure(ctx context.Context, sc *ssh.Client, cfg *config.Config) error {
	// Build environment string
	var envLines []string
	for k, v := range cfg.Deploy.Env {
		envLines = append(envLines, fmt.Sprintf("Environment=%s=%s", k, v))
	}
	envSection := strings.Join(envLines, "\n")

	// Build command string
	execCmd := fmt.Sprintf("%s/%s", cfg.Deploy.RemotePath, cfg.Build.Output)
	if len(cfg.Deploy.Command) > 0 {
		execCmd = fmt.Sprintf("%s %s", execCmd, strings.Join(cfg.Deploy.Command, " "))
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target docker.service

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5
%%s

[Install]
WantedBy=multi-user.target
`, cfg.Name, cfg.Deploy.User, cfg.Deploy.User, cfg.Deploy.RemotePath, execCmd)

	// Since we use fmt.Sprintf above, we need to handle the %s for envSection separately or escape it
	serviceContent = fmt.Sprintf(serviceContent, envSection)

	// Write service file
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", cfg.Deploy.ServiceName)
	cmd := fmt.Sprintf(`echo '%s' | sudo tee %s > /dev/null`, serviceContent, servicePath)
	if _, err := sc.Run(ctx, cmd); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if _, err := sc.Run(ctx, "sudo systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if _, err := sc.Run(ctx, fmt.Sprintf("sudo systemctl enable %s", cfg.Deploy.ServiceName)); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}
