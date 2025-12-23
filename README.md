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

## Setting up as a systemd Service

To run the links server as a systemd service on Linux (compatible with Raspberry Pi and Ubuntu):

### 1. Install the binary

First, build and install the binary to a system location:

```bash
# Build the binary
go build -o linksserver ./cmd/linksserver

# Move binary to system location
sudo mv linksserver /usr/local/bin/linksserver

# Make it executable
sudo chmod +x /usr/local/bin/linksserver
```

### 2. Create a systemd service file

```bash
sudo tee /etc/systemd/system/linksserver.service > /dev/null <<EOF
[Unit]
Description=Links Server - Simple HTTP links display service
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/lib/linksserver
ExecStart=/usr/local/bin/linksserver --port 8080
Restart=always
RestartSec=5
Environment="PORT=8080"

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/linksserver

[Install]
WantedBy=multi-user.target
EOF
```

### 3. Create the working directory and set permissions

```bash
# Create directory for the database
sudo mkdir -p /var/lib/linksserver

# Set ownership
sudo chown www-data:www-data /var/lib/linksserver
```

### 4. Enable and start the service

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable linksserver

# Start the service
sudo systemctl start linksserver

# Check status
sudo systemctl status linksserver
```

### 5. View logs

```bash
# View real-time logs
sudo journalctl -u linksserver -f

# View recent logs
sudo journalctl -u linksserver -n 50
```

### Managing the service

```bash
# Stop the service
sudo systemctl stop linksserver

# Restart the service
sudo systemctl restart linksserver

# Disable service from starting on boot
sudo systemctl disable linksserver
```

**Note:** The service is configured to run on port 8080 by default (instead of port 80) to avoid requiring root privileges. You can change the port by modifying the `--port` flag or `PORT` environment variable in the service file, then run `sudo systemctl daemon-reload` and `sudo systemctl restart linksserver`.
