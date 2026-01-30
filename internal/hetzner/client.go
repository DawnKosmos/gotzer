package hetzner

import (
	"context"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// Client wraps the Hetzner Cloud API client
type Client struct {
	client *hcloud.Client
}

// NewClient creates a new Hetzner API client
func NewClient(token string) *Client {
	client := hcloud.NewClient(
		hcloud.WithToken(token),
		hcloud.WithApplication("gotzer", "1.0.0"),
	)
	return &Client{client: client}
}

// ServerOpts contains options for creating a server
type ServerOpts struct {
	Name        string
	Location    string
	ServerType  string
	Image       string
	SSHKeyNames []string
}

// CreateServer provisions a new Hetzner Cloud server
func (c *Client) CreateServer(ctx context.Context, opts ServerOpts) (*hcloud.Server, error) {
	// Get SSH keys
	var sshKeys []*hcloud.SSHKey
	for _, keyName := range opts.SSHKeyNames {
		key, _, err := c.client.SSHKey.GetByName(ctx, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to get SSH key %s: %w", keyName, err)
		}
		if key == nil {
			return nil, fmt.Errorf("SSH key not found: %s", keyName)
		}
		sshKeys = append(sshKeys, key)
	}

	// Create server
	result, _, err := c.client.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:       opts.Name,
		ServerType: &hcloud.ServerType{Name: opts.ServerType},
		Image:      &hcloud.Image{Name: opts.Image},
		Location:   &hcloud.Location{Name: opts.Location},
		SSHKeys:    sshKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Wait for server to be ready
	if err := c.waitForAction(ctx, result.Action); err != nil {
		return nil, fmt.Errorf("failed waiting for server creation: %w", err)
	}

	// Get full server info
	server, _, err := c.client.Server.GetByID(ctx, result.Server.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	return server, nil
}

// GetServer retrieves a server by name
func (c *Client) GetServer(ctx context.Context, name string) (*hcloud.Server, error) {
	server, _, err := c.client.Server.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}
	return server, nil
}

// DeleteServer destroys a server
func (c *Client) DeleteServer(ctx context.Context, name string) error {
	server, err := c.GetServer(ctx, name)
	if err != nil {
		return err
	}
	if server == nil {
		return fmt.Errorf("server not found: %s", name)
	}

	result, _, err := c.client.Server.DeleteWithResult(ctx, server)
	if err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	if err := c.waitForAction(ctx, result.Action); err != nil {
		return fmt.Errorf("failed waiting for server deletion: %w", err)
	}

	return nil
}

// ListServers returns all servers
func (c *Client) ListServers(ctx context.Context) ([]*hcloud.Server, error) {
	servers, err := c.client.Server.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	return servers, nil
}

// ListSSHKeys returns all SSH keys
func (c *Client) ListSSHKeys(ctx context.Context) ([]*hcloud.SSHKey, error) {
	keys, err := c.client.SSHKey.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSH keys: %w", err)
	}
	return keys, nil
}

// CreateSSHKey creates a new SSH key
func (c *Client) CreateSSHKey(ctx context.Context, name, publicKey string) (*hcloud.SSHKey, error) {
	key, _, err := c.client.SSHKey.Create(ctx, hcloud.SSHKeyCreateOpts{
		Name:      name,
		PublicKey: publicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}
	return key, nil
}

// waitForAction waits for a Hetzner action to complete
func (c *Client) waitForAction(ctx context.Context, action *hcloud.Action) error {
	if action == nil {
		return nil
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			a, _, err := c.client.Action.GetByID(ctx, action.ID)
			if err != nil {
				return err
			}
			if a.Status == hcloud.ActionStatusSuccess {
				return nil
			}
			if a.Status == hcloud.ActionStatusError {
				return fmt.Errorf("action failed: %s", a.ErrorMessage)
			}
		}
	}
}
