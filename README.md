# links

small displayable links server

## Installation

```bash
go install github.com/tomek7667/links/cmd/linksserver@latest
```

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

`update` drops a versioned binary next to the current one (e.g. `linksserver-v1.1.0.exe`), keeps a backup of the existing binary, and backs up `links.db.json` when present. Run the staged binary to test, then `complete-update` to promote it and delete the backups.

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
```
