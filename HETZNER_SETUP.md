# Hetzner Cloud Configuration Guide

To use Gotzer successfully, you need to set up a few things in your [Hetzner Cloud Console](https://console.hetzner.cloud/).

## 1. Create an API Token
Gotzer uses the Hetzner API to provision servers and manage infrastructure.

1. Go to your project in the Hetzner Cloud Console.
2. Select **Security** (sidebar) -> **API Tokens**.
3. Click **Generate API Token**.
4. Give it a name (e.g., `gotzer-token`).
5. **Permissions**: Set to **Read & Write**.
6. Copy the token immediately; you won't see it again.
7. Run `gotzer auth` and paste this token when prompted.

## 2. Register your SSH Key
Gotzer needs an SSH key to access the servers it creates.

1. Go to **Security** -> **SSH Keys**.
2. Click **Add SSH Key**.
3. Paste your public key (usually found at `~/.ssh/id_ed25519.pub` or `~/.ssh/id_rsa.pub`).
4. Give it a name that matches what you use locally (or what's in your `.gotzer.yaml` under `ssh_key_name`).
5. **Important**: By default, Gotzer looks for `~/.ssh/id_ed25519`. If you use a different path, update `~/.gotzer/config.yaml` or provide the `--ssh-key-path` flag.

## 3. Choose a Location
Hetzner has several data centers. Note the short name of the one you want to use:
- `fsn1` (Falkenstein, Germany)
- `nbg1` (Nuremberg, Germany)
- `hel1` (Helsinki, Finland)
- `ash` (Ashburn, VA, USA)
- `hil` (Hillsboro, OR, USA)

Update the `server.location` field in your `.gotzer.yaml` with your choice.

## 4. Default Limits
New Hetzner accounts sometimes have low limits for the number of servers or specific types. If `gotzer provision` fails with a "limit exceeded" error, you may need to contact Hetzner support to increase your limits.
