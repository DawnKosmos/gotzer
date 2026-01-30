# Gotzer

A Go CLI tool + library for deploying Go applications to Hetzner Cloud servers.

## Features

- 🚀 **Single-command deployment** - Build and deploy with `gotzer deploy`
- 🐳 **Docker services** - PostgreSQL, Typesense, Redis via Docker Compose
- 🔄 **Cross-compilation** - Supports ARM64 and AMD64 Linux targets
- 🔐 **Secure** - SSH-based deployment, no agents required
- 📦 **Direct deployment** - Native Go binary (no Docker wrapper for your app)
- ⚙️ **Systemd integration** - Automatic service management

## Installation

```bash
go install github.com/DawnKosmos/gotzer/cmd/gotzer@latest
```

Or build from source:

```bash
git clone https://github.com/DawnKosmos/gotzer
cd gotzer
go build -o gotzer ./cmd/gotzer
```

## Quick Start

### 1. Initialize your project

```bash
cd your-go-project
gotzer init
```

This creates a `.gotzer.yaml` configuration file.

### 2. Configure Hetzner API token

Get a token from [Hetzner Cloud Console](https://console.hetzner.cloud/) → Select project → Security → API Tokens

```bash
gotzer auth
```

### 3. Provision a server

```bash
gotzer provision
```

This creates a Hetzner server and sets up:
- Docker & Docker Compose
- Systemd service for your app
- PostgreSQL, Typesense (if enabled)
- UFW firewall rules

### 4. Deploy your app

```bash
gotzer deploy
```

This is the **default workflow** - builds and deploys only your Go app.

## Configuration

### `.gotzer.yaml`

```yaml
name: my-go-app
description: "My Go application"

server:
  name: my-app-server
  location: fsn1              # fsn1, nbg1, hel1, ash, hil
  type: cpx11                 # cpx11, cpx21 | ARM: cax11, cax21
  image: ubuntu-24.04
  architecture: x64           # x64 or arm64

build:
  main: ./cmd/server          # Path to main package
  output: app                 # Binary name
  ldflags: "-s -w"

deploy:
  remote_path: /opt/apps/my-app
  service_name: my-app
  user: app
  env:
    APP_ENV: production
    DATABASE_URL: "postgres://..."

services:
  postgres:
    enabled: true
    image: postgres:16
    port: 5432
    env:
      POSTGRES_DB: myapp
      POSTGRES_USER: myapp
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"

  typesense:
    enabled: true
    image: typesense/typesense:27.1
    port: 8108
    env:
      TYPESENSE_API_KEY: "${TYPESENSE_API_KEY}"
```

## Commands

| Command | Description |
|---------|-------------|
| `gotzer init` | Create `.gotzer.yaml` config |
| `gotzer auth` | Configure Hetzner API token |
| `gotzer provision` | Create server + setup services |
| `gotzer deploy` | Build & deploy Go app (default) |
| `gotzer ssh` | SSH into the server |
| `gotzer status` | Show server and app status |
| `gotzer logs [-f]` | View application logs |
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
