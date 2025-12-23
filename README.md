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
Environment="PORT=8080"

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable --now linksserver
sudo systemctl status linksserver
```
