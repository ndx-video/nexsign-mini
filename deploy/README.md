# deploy

Deployment resources and sample configuration files for nexSign mini.

## Purpose

This directory contains sample configuration files and deployment resources for running `nsm` in production environments.

## Contents

### config.json.sample

Sample central configuration file for nexSign mini.

**Location**: Copy to `/etc/nsm/config.json` for production use.

**Contents**:
```json
{
  "key_file": "nsm_key.pem",
  "host_data_file": "test-hosts.json",
  "port": 8080,
  "mdns_service_name": "_nsm._tcp",
  "tendermint_peers_file": "tendermint_persistent_peers",
  "log_file": "nsm.log",
  "restart_command": "systemctl restart nsm",
  "enable_actions": false
}
```

**Usage**:
```bash
# Copy sample to production location
sudo mkdir -p /etc/nsm
sudo cp deploy/config.json.sample /etc/nsm/config.json

# Edit for your environment
sudo nano /etc/nsm/config.json

# Set proper permissions
sudo chmod 600 /etc/nsm/config.json
sudo chown nsm:nsm /etc/nsm/config.json
```

**Important Fields**:
- `enable_actions`: Set to `true` only in production after testing
- `restart_command`: Use agent URL (`http://localhost:9001/action`) for privilege separation
- `key_file`: Use absolute path in production (e.g., `/var/lib/nsm/nsm_key.pem`)

### nsm.service.sample

Sample systemd unit file for running `nsm` as a system service.

**Location**: Copy to `/etc/systemd/system/nsm.service` for production use.

**Contents**:
```ini
[Unit]
Description=nexSign mini node
After=network.target

[Service]
Type=simple
User=nsm
WorkingDirectory=/home/nsm/.nsm
ExecStart=/home/nsm/.nsm/nsm
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Usage**:
```bash
# Copy sample to systemd directory
sudo cp deploy/nsm.service.sample /etc/systemd/system/nsm.service

# Edit if needed (change paths, user, etc.)
sudo nano /etc/systemd/system/nsm.service

# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable nsm

# Start service
sudo systemctl start nsm

# Check status
sudo systemctl status nsm
```

## Production Deployment Steps

### 1. Create System User

```bash
sudo useradd -r -m -d /home/nsm -s /bin/bash nsm
```

### 2. Create Directory Structure

```bash
sudo mkdir -p /home/nsm/.nsm
sudo mkdir -p /var/lib/nsm
sudo mkdir -p /var/log/nsm
sudo mkdir -p /etc/nsm

sudo chown nsm:nsm /home/nsm/.nsm
sudo chown nsm:nsm /var/lib/nsm
sudo chown nsm:nsm /var/log/nsm
sudo chown root:nsm /etc/nsm
sudo chmod 750 /etc/nsm
```

### 3. Build and Install Binary

```bash
# Build for production
go build -o nsm cmd/nsm/main.go

# Install binary
sudo cp nsm /home/nsm/.nsm/nsm
sudo chown nsm:nsm /home/nsm/.nsm/nsm
sudo chmod 755 /home/nsm/.nsm/nsm
```

### 4. Install Configuration

```bash
# Copy and customize config
sudo cp deploy/config.json.sample /etc/nsm/config.json
sudo nano /etc/nsm/config.json

# Update paths for production
# - key_file: /var/lib/nsm/nsm_key.pem
# - host_data_file: /var/lib/nsm/hosts.json
# - log_file: /var/log/nsm/nsm.log

# Set permissions
sudo chmod 600 /etc/nsm/config.json
sudo chown nsm:nsm /etc/nsm/config.json
```

### 5. Generate or Copy Identity Key

```bash
# Option 1: Let the service generate a new key on first run
# (Key will be created at the configured key_file path)

# Option 2: Copy existing key
sudo cp nsm_key.pem /var/lib/nsm/nsm_key.pem
sudo chown nsm:nsm /var/lib/nsm/nsm_key.pem
sudo chmod 600 /var/lib/nsm/nsm_key.pem
```

### 6. Install Systemd Service

```bash
# Install service file
sudo cp deploy/nsm.service.sample /etc/systemd/system/nsm.service

# Edit if paths changed
sudo nano /etc/systemd/system/nsm.service

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable nsm
sudo systemctl start nsm

# Verify
sudo systemctl status nsm
```

### 7. Configure Firewall

```bash
# Allow HTTP port
sudo ufw allow 8080/tcp

# Allow mDNS
sudo ufw allow 5353/udp

# Reload firewall
sudo ufw reload
```

### 8. Verify Deployment

```bash
# Check service status
sudo systemctl status nsm

# View logs
sudo journalctl -u nsm -f

# Check web dashboard
curl http://localhost:8080

# Check log file
tail -f /var/log/nsm/nsm.log
```

## Privilege Separation (Optional but Recommended)

For secure restart action execution, deploy the privileged agent:

### 1. Build Agent

```bash
go build -o nsm-agent internal/agent/agent.go
sudo cp nsm-agent /usr/local/bin/nsm-agent
sudo chown root:root /usr/local/bin/nsm-agent
sudo chmod 755 /usr/local/bin/nsm-agent
```

### 2. Create Agent Service

Create `/etc/systemd/system/nsm-agent.service`:

```ini
[Unit]
Description=nexSign mini privileged agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/nsm-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### 3. Enable Agent

```bash
sudo systemctl daemon-reload
sudo systemctl enable nsm-agent
sudo systemctl start nsm-agent
```

### 4. Configure Main Service to Use Agent

Edit `/etc/nsm/config.json`:

```json
{
  "restart_command": "http://localhost:9001/action",
  "enable_actions": true
}
```

Restart the main service:

```bash
sudo systemctl restart nsm
```

## Monitoring

### View Logs

```bash
# Systemd journal
sudo journalctl -u nsm -f

# Log file
tail -f /var/log/nsm/nsm.log

# Check for errors
sudo journalctl -u nsm -p err
```

### Check Status

```bash
# Service status
sudo systemctl status nsm

# Is service active?
systemctl is-active nsm

# Uptime and restarts
systemctl show nsm --property=ActiveEnterTimestamp,NRestarts
```

### Resource Usage

```bash
# Memory and CPU
systemctl status nsm

# Detailed stats
systemd-cgtop | grep nsm
```

## Backup

### Critical Files to Backup

1. **Identity Key**:
   ```bash
   sudo cp /var/lib/nsm/nsm_key.pem /backup/nsm_key.pem.backup
   ```

2. **Configuration**:
   ```bash
   sudo cp /etc/nsm/config.json /backup/config.json.backup
   ```

3. **State File** (if using persistent state):
   ```bash
   sudo cp /var/lib/nsm/hosts.json /backup/hosts.json.backup
   ```

### Restore

```bash
# Restore key
sudo cp /backup/nsm_key.pem.backup /var/lib/nsm/nsm_key.pem
sudo chown nsm:nsm /var/lib/nsm/nsm_key.pem
sudo chmod 600 /var/lib/nsm/nsm_key.pem

# Restore config
sudo cp /backup/config.json.backup /etc/nsm/config.json
sudo chown nsm:nsm /etc/nsm/config.json
sudo chmod 600 /etc/nsm/config.json

# Restart service
sudo systemctl restart nsm
```

## Upgrading

### Binary Upgrade

```bash
# Build new version
go build -o nsm cmd/nsm/main.go

# Stop service
sudo systemctl stop nsm

# Backup current binary
sudo cp /home/nsm/.nsm/nsm /home/nsm/.nsm/nsm.backup

# Install new binary
sudo cp nsm /home/nsm/.nsm/nsm
sudo chown nsm:nsm /home/nsm/.nsm/nsm

# Start service
sudo systemctl start nsm

# Check status
sudo systemctl status nsm
```

### Rollback

```bash
sudo systemctl stop nsm
sudo cp /home/nsm/.nsm/nsm.backup /home/nsm/.nsm/nsm
sudo systemctl start nsm
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -u nsm -n 50

# Check config syntax
cat /etc/nsm/config.json | jq .

# Check binary permissions
ls -la /home/nsm/.nsm/nsm

# Try running manually
sudo -u nsm /home/nsm/.nsm/nsm
```

### Permission Errors

```bash
# Fix ownership
sudo chown -R nsm:nsm /home/nsm/.nsm
sudo chown -R nsm:nsm /var/lib/nsm
sudo chown -R nsm:nsm /var/log/nsm

# Fix key permissions
sudo chmod 600 /var/lib/nsm/nsm_key.pem
```

### Port Conflicts

If port 8080 is in use:

```bash
# Check what's using the port
sudo lsof -i :8080

# Either change nsm port in config or stop conflicting service
```

## Security Checklist

- [ ] Service runs as non-root user (`nsm`)
- [ ] Identity key has 0600 permissions
- [ ] Config file has 0600 permissions
- [ ] Binary directory not world-writable
- [ ] Firewall rules configured
- [ ] `enable_actions` only enabled in production
- [ ] Agent runs as root (if using privilege separation)
- [ ] Regular backups of identity key
- [ ] Logs monitored for errors

## Multiple Node Deployment

For deploying multiple nodes:

1. Use unique key files for each node
2. Use different ports if on same host
3. Ensure mDNS service name matches across nodes
4. Configure Tendermint persistent peers appropriately

See `test-deploy.sh` for automated multi-host deployment.
