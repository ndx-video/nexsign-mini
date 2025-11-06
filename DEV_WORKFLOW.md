# Development Workflow for nexSign mini

This document describes the development workflow for testing and deploying nsm updates to VirtualBox test hosts.

## Test Environment Setup

### VirtualBox Test Hosts
- **nsm01**: 192.168.10.147
- **nsm02**: 192.168.10.174
- **nsm03**: 192.168.10.135
- **nsm04**: 192.168.10.211
- **frodo** (dev host): 192.168.10.240

All hosts are running in bridged network mode for direct LAN access.

## Development Workflow

### 1. Local Development and Testing

```bash
# Make your changes to the code
vim internal/web/server.go

# Build and test locally
go build -o nsm main.go
./nsm

# Test the local instance
curl http://localhost:8080/api/health
```

### 2. Quick Deployment to Test Hosts

Use the provided `dev-deploy.sh` script for rapid iteration:

```bash
# Deploy to all test hosts
./dev-deploy.sh

# Or manually deploy to a single host
go build -o nsm main.go
scp nsm nsm@192.168.10.147:/tmp/
ssh nsm@192.168.10.147 "sudo systemctl stop nsm.service && sudo mv /tmp/nsm /usr/local/bin/nsm && sudo systemctl start nsm.service"
```

### 3. Testing with Real Hosts

After deployment:

1. **Check Dashboard**: Visit http://localhost:8080
2. **Verify Health Checks**: Click "🔄 Check All Hosts" 
3. **Test CMS Status**: Verify Anthias CMS status for each host
4. **Test Network Sync**: Click "📤 Push to Other Hosts"
5. **Test Reboot**: (⚠️ Use with caution) Test reboot on a single host

### 4. Host Status Management

The dashboard displays several status types:

- 🟢 **Healthy**: NSM service running and responsive
- 🟣 **Stale**: Host needs nsm binary update (manual flag)
- 🟡 **Unhealthy**: NSM service running but not responding correctly
- 🟠 **Connection Refused**: Host reachable but NSM service not running
- 🔴 **Unreachable**: Cannot connect to host

### 5. Marking Hosts as Stale

When you deploy a new version to some hosts but not others:

1. Edit `hosts.json` directly, or
2. Use the API to update status:

```bash
# Mark a host as stale (needs manual edit for now)
# This will be preserved during health checks
vim hosts.json  # Change status to "stale"
```

The health checker will preserve the "stale" status while still checking CMS status.

## Production Deployment Strategy

In production, nsm will be distributed via Debian APT repository:

### Initial Setup (one-time per host)

```bash
# Add NSM repository to APT sources
echo "deb [trusted=yes] https://apt.nexsign.example.com stable main" | sudo tee /etc/apt/sources.list.d/nsm.list

# Install nsm
sudo apt update
sudo apt install nsm

# Enable and start service
sudo systemctl enable nsm.service
sudo systemctl start nsm.service
```

### Updates

```bash
# Update to latest version
sudo apt update
sudo apt upgrade nsm

# Service restarts automatically via systemd
```

### Version Checking

NSM will eventually report its own version. When a new version is available:
1. Dashboard shows hosts as "stale" (comparison with latest version)
2. Admin clicks "Update" button
3. NSM sends update command to remote host
4. Remote host runs `apt update && apt upgrade nsm`
5. Systemd restarts the service automatically

## Development Best Practices

### Before Deploying
- ✅ Test locally first
- ✅ Run unit tests: `go test ./...`
- ✅ Build successfully: `go build ./...`
- ✅ Commit changes: `git commit -m "feat: description"`

### During Testing
- 📝 Document any issues found
- 📝 Test on at least 2 hosts before marking complete
- 📝 Verify health checks still work
- 📝 Check logs: `ssh nsm@HOST "tail -f /var/log/nsm.log"`

### After Testing
- ✅ Push changes to git: `git push origin main-dev-simple-rego`
- ✅ Update ROADMAP.md if needed
- ✅ Create PR when feature is complete

## Debugging Remote Hosts

### Check Service Status
```bash
ssh nsm@192.168.10.147 "sudo systemctl status nsm.service"
```

### View Logs
```bash
# Systemd journal
ssh nsm@192.168.10.147 "sudo journalctl -u nsm.service -f"

# Application logs (if redirected)
ssh nsm@192.168.10.147 "tail -f /var/log/nsm.log"
```

### Manual Process Check
```bash
ssh nsm@192.168.10.147 "ps aux | grep nsm"
ssh nsm@192.168.10.147 "netstat -tlnp | grep 8080"
```

### Test API Directly
```bash
# Health check
curl http://192.168.10.147:8080/api/health

# Get hosts list
curl http://192.168.10.147:8080/api/hosts

# Check specific host
ssh nsm@192.168.10.147 "curl http://localhost:8080/api/hosts"
```

## Common Issues and Solutions

### Issue: Port 8080 Already in Use
```bash
ssh nsm@HOST "sudo fuser -k 8080/tcp"
ssh nsm@HOST "sudo systemctl restart nsm.service"
```

### Issue: Binary Not Executable
```bash
ssh nsm@HOST "sudo chmod +x /usr/local/bin/nsm"
```

### Issue: Service Won't Start
```bash
# Check for errors
ssh nsm@HOST "sudo journalctl -u nsm.service -n 50"

# Try manual start to see output
ssh nsm@HOST "cd /opt/nsm && /usr/local/bin/nsm"
```

### Issue: Stale Status Gets Overwritten
This should not happen with the latest code. If it does:
1. Check that you have the latest version deployed
2. Verify the `CheckAllHosts()` function preserves stale status
3. Check if health check is being called too frequently

## Network Synchronization

The "Push to Other Hosts" feature:
1. Collects current host list from localhost
2. Sends POST request to `/api/hosts/receive` on each remote host
3. Remote host replaces its entire host list with received data
4. No conflict resolution - last push wins (manual management model)

This is intentional for the "simple" architecture. For testing:
```bash
# Manually push from one host to another
curl -X POST http://192.168.10.147:8080/api/hosts/receive \
  -H "Content-Type: application/json" \
  -d @hosts.json
```

## SSH Key Setup (Recommended)

To avoid password prompts during deployment:

```bash
# Generate SSH key if you don't have one
ssh-keygen -t ed25519 -C "dev@nexsign-mini"

# Copy to all test hosts
for host in 192.168.10.{147,174,135,211}; do
  ssh-copy-id nsm@$host
done
```

## Quick Reference Commands

```bash
# Build only
go build -o nsm main.go

# Build and run locally
go build -o nsm main.go && ./nsm

# Deploy to all hosts
./dev-deploy.sh

# Deploy to single host (manual)
scp nsm nsm@192.168.10.147:/tmp/ && ssh nsm@192.168.10.147 "sudo mv /tmp/nsm /usr/local/bin/nsm && sudo systemctl restart nsm.service"

# Test dashboard
xdg-open http://localhost:8080  # or just visit in browser

# Check all hosts are reachable
for host in 192.168.10.{147,174,135,211}; do
  echo -n "$host: "
  curl -s http://$host:8080/api/health && echo "OK" || echo "FAIL"
done
```

## Next Steps

1. ✅ Test dashboard with current VirtualBox hosts
2. ⏳ Create automated deployment via SSH
3. ⏳ Add version reporting to nsm binary
4. ⏳ Implement version comparison for auto-stale detection
5. ⏳ Build Debian package for APT distribution
6. ⏳ Set up APT repository server
7. ⏳ Implement remote update trigger via API
