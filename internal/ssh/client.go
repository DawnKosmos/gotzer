package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client handles SSH connections and file transfers
type Client struct {
	host      string
	user      string
	keyPath   string
	sshClient *ssh.Client
	connected bool
}

// NewClient creates a new SSH client
func NewClient(host, user, keyPath string) *Client {
	return &Client{
		host:    host,
		user:    user,
		keyPath: keyPath,
	}
}

// Connect establishes an SSH connection
func (c *Client) Connect(ctx context.Context) error {
	keyPath := expandPath(c.keyPath)
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse SSH key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: c.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: proper host key verification
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:22", c.host)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.sshClient = conn
	c.connected = true
	return nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.sshClient != nil {
		return c.sshClient.Close()
	}
	return nil
}

// Run executes a command on the remote server
func (c *Client) Run(ctx context.Context, cmd string) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, output)
	}

	return string(output), nil
}

// RunInteractive runs a command with stdin/stdout attached
func (c *Client) RunInteractive(ctx context.Context, cmd string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// Upload copies a local file to the remote server via SCP
func (c *Client) Upload(ctx context.Context, localPath, remotePath string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// Read local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create remote directory
	dir := filepath.Dir(remotePath)
	if _, err := c.Run(ctx, fmt.Sprintf("mkdir -p %s", dir)); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Create session
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Start SCP
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// SCP protocol: send file header
		fmt.Fprintf(w, "C0755 %d %s\n", stat.Size(), filepath.Base(remotePath))

		// Send file content
		io.Copy(w, localFile)

		// Send completion
		fmt.Fprint(w, "\x00")
	}()

	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("SCP failed: %w", err)
	}

	return nil
}

// Shell opens an interactive SSH shell
func (c *Client) Shell() error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up terminal
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", 80, 40, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	return session.Wait()
}

// WaitForSSH waits for SSH to become available
func WaitForSSH(ctx context.Context, host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", host), 5*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("SSH not available after %v", timeout)
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
