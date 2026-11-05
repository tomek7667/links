# links

small displayable links server — a single self-contained binary (UI embedded, no build step, works offline on your LAN).

## Features

- **Links dashboard** — tiles with favicons, emoji or custom icons, groups (collapsible, renameable), descriptions, drag & drop reordering (drop onto another group to move a link there).
- **Edit everything** — title, URL, group, description, icon. The editor splits the URL into separate **Host / IP** and **Port** fields, so changing just the IP of a link is one field.
- **`{ip}` placeholder** — write `http://{ip}:8096` and the link always points at this server's current IP. `{host}` uses the hostname you opened the dashboard with (e.g. `pi.local`).
- **Server IP** — auto-detected (container bridges like `docker0`/`br-*` are ignored), or pick another interface / type a custom IP in Settings. Click the IP in the header to copy it.
- **Change IP in links** — Settings lists every host/IP your links use; replace one to update all of its links at once (port and path are kept), or convert a literal IP to `{ip}`.
- **Status checks** — the server pings each link every 30 s and shows an online/offline dot with HTTP status and latency.
- **Search** (`/` or `Ctrl+K`, `Enter` opens the first match), `N` adds a link, delete with **undo**.
- **System monitor** — CPU (model, cores, clocks, temperature, load, top process), memory & swap, network throughput, GPUs, disks, uptime; 30-minute history chart with 1m/5m/15m/30m ranges, hover tooltip, click-to-pin, toggleable series and CSV export.
- **Backup** — export links + settings as JSON, import (merge or replace; old `links.db.json` files work too).
- Dark / light / auto theme, responsive down to phone width.

## Installation

```bash
go install github.com/tomek7667/links/cmd/linksserver@latest
```

On a Raspberry Pi (or whenever `go install` fails inside `runtime/cgo`, e.g. `size of array '_check_for_64_bit_pointer_matching_GoInt' is negative`), disable cgo — linksserver doesn't need it:

```bash
CGO_ENABLED=0 go install github.com/tomek7667/links/cmd/linksserver@latest
```

Prebuilt static binaries (including `linux_arm64` and `linux_armv7` for the Pi) are attached to every [release](https://github.com/tomek7667/links/releases).

Or build from source:

```bash
git clone https://github.com/tomek7667/links.git
cd links
go build ./cmd/linksserver
```

## Updating

If you installed `linksserver` as a standalone binary, you can update it in-place:

```bash
linksserver update
# ...test the new staged binary (e.g. ./linksserver-<version>.exe)...
linksserver complete-update
```

`update` builds with `CGO_ENABLED=0` (unless you set it yourself), so it also works on a Raspberry Pi. It drops a versioned binary next to the current one (e.g. `linksserver-v1.1.0.exe`), keeps a backup of the existing binary, and backs up `links.db.json` when present. Run the staged binary to test, then `complete-update` to promote it and delete the backups.

## systemd Service (Raspberry Pi / Ubuntu)

```bash
# Install
go install github.com/tomek7667/links/cmd/linksserver@latest

# Create service file
sudo tee /etc/systemd/system/linksserver.service > /dev/null <<EOF
[Unit]
Description=Links Server
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$(pwd)
ExecStart=$(go env GOPATH)/bin/linksserver
Restart=always
Environment="PORT=80"

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable --now linksserver
sudo systemctl status linksserver

# Logs
sudo journalctl -u linksserver -f
sudo journalctl -u linksserver -n 100 --no-pager
```

## Data

Links and settings live in `links.db.json` in the working directory. Files from older versions are migrated automatically (links get an `id`). Writes are atomic (temp file + rename).

## HTTP API

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/links` | all links, in display order |
| `POST` | `/api/links[?at=N]` | `{"title","url","group?","description?","icon?"}` — creates a link (optionally at position `N`) |
| `PUT` | `/api/links/{id}` | same body — updates a link |
| `DELETE` | `/api/links/{id}` | deletes a link (`DELETE /api/links` with `{"url": ...}` still works) |
| `PUT` | `/api/links/order` | `{"ids": [...]}` |
| `POST` | `/api/links/replace-host` | `{"from": "192.168.1.20", "to": "{ip}"}` — swaps the host in every matching link |
| `GET` | `/api/links/status` | reachability of each link, keyed by id |
| `POST` | `/api/groups/rename` | `{"from": "Media", "to": "Streaming"}` |
| `GET` / `PUT` | `/api/settings` | `{"hostIp": "", "title": ""}` — empty `hostIp` means auto-detect |
| `GET` | `/api/export` | JSON backup |
| `POST` | `/api/import[?replace=1]` | a backup, a `links.db.json`, or a bare array of links |
| `GET` | `/api/resources[?history=1]` | system snapshot (and history) |

## Releasing

Push a `v1.x.y` tag; the release workflow builds and attaches binaries. Keep tags on `v1` — `go install …@latest` (and therefore `linksserver update`) ignores `v2+` tags unless the module path ends in `/v2`.
