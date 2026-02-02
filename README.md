# Gotzer

A Go CLI tool + library for deploying Go applications and Static frontends to Hetzner Cloud servers.

## Features

- 🚀 **Single-command deployment** - Build and deploy with `gotzer deploy`
- 🌐 **Frontend Support** - Native support for Vite, React, and static sites
- 🐳 **Docker services** - PostgreSQL, Typesense, Redis via Docker Compose
- 🔄 **Cross-compilation** - Supports ARM64 and AMD64 Linux targets
- 🔐 **Secure** - SSH-based deployment, no agents required
- 📦 **Direct deployment** - Native Go binary (no Docker wrapper for your app)
- ⚙️ **Systemd integration** - Automatic service management for Go apps

## Installation

```bash
go install github.com/DawnKosmos/gotzer@latest
```

## Quick Start

### 1. Initialize your project

```bash
gotzer init
```

### 2. Configure Hetzner API token

```bash
gotzer auth
```

### 3. Provision a server

```bash
gotzer provision
```

### 4. Deploy your app

```bash
gotzer deploy
```

## Frontend (Static) Support

Gotzer can deploy your Vite/React frontend. It bundles your folder into a compressed stream for efficient transfer.

```yaml
build:
  type: static
  command: "npm install && npm run build"
  dir: "./dist"

deploy:
  type: static
  remote_path: /var/www/html
```

## Configuration

### `.gotzer.yaml`

```yaml
name: my-app
description: "My application"

server:
  name: my-server
  location: nbg1
  type: cax11
  image: ubuntu-24.04
  architecture: arm64

build:
  type: go                    # "go" (default) or "static"
  main: ./cmd/server          # (Go only)
  output: app                 # (Go only)
  command: "npm run build"    # (Static only)
  dir: "./dist"               # (Static only)

deploy:
  type: service               # "service" (default) or "static"
  remote_path: /opt/apps/my-app
  service_name: my-app        # (Service only)
  command: ["serve"]          # Arguments for your binary
  env:
    PORT: "80"
```

## Commands

| Command | Description |
|---------|-------------|
| `gotzer init` | Create `.gotzer.yaml` config |
| `gotzer auth` | Configure Hetzner API token |
| `gotzer provision` | Create server + setup services |
| `gotzer provision --update` | Sync services on existing server |
| `gotzer deploy` | Build & deploy (detects type) |
| `gotzer stop/start/restart` | Manage the application service |
| `gotzer status` | Show server and app status |
| `gotzer logs [-f]` | View application logs |
| `gotzer ssh` | SSH into the server |
| `gotzer destroy` | Delete the server |

## Library Usage

Gotzer can also be used as a Go library:

```go
package main

import (
    "context"
    "os"

    "github.com/DawnKosmos/gotzer/pkg/gotzer"
)

func main() {
    ctx := context.Background()

    client := gotzer.NewClient(
        gotzer.WithToken(os.Getenv("HETZNER_TOKEN")),
    )

    // Provision a server
    server, _ := client.Provision(ctx, gotzer.ProvisionOpts{
        Name:       "my-server",
        Location:   "fsn1",
        ServerType: "cpx11",
        Image:      "ubuntu-24.04",
    })

    // Deploy an app
    client.Deploy(ctx, gotzer.DeployOpts{
        ServerIP:     server.IP,
        MainPkg:      "./cmd/server",
        BinaryName:   "app",
        RemotePath:   "/opt/apps/my-app",
        ServiceName:  "my-app",
        Architecture: "amd64",
    })
}
```

## Requirements

- Go 1.21+
- SSH key registered with Hetzner Cloud
- Hetzner Cloud API token

## License

MIT
