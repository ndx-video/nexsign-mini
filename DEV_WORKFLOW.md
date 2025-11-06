# Development workflow

This document captures day-to-day practices for the VirtualBox lab and outlines how to test, deploy, and troubleshoot nexSign mini.

## Lab hosts

- 192.168.10.147 – nsm01
- 192.168.10.174 – nsm02
- 192.168.10.135 – nsm03
- 192.168.10.211 – nsm04

All hosts run in bridged mode and expose SSH for the `nsm` user.

## Iteration loop

1. Make changes locally and run `go test ./...`.
2. Start a local instance with `go run main.go` and sanity-check the dashboard at `http://localhost:8080`.
3. Deploy to the lab with the Go deployer:

   ```bash
   go run cmd/deployer/main.go --hosts all --parallel 4
   ```

4. After deployment verify:

    - the dashboard shows “NSM Online” links for healthy hosts
    - “Push to Other Hosts” succeeds and remote instances create `backups/hosts-<timestamp>.db` snapshots
    - port conflicts are reported immediately if a stale process is still running

## Dashboard status cheatsheet

- `NSM Online` – health check succeeded; link opens `http://<ip>:8080`
- `NSM Online (Update Required)` – service responds but reports an older build
- `unhealthy` – TCP open, API/health returned an error
- `connection_refused` – device reachable but nothing listening on port 8080
- `unreachable` – TCP handshake failed (offline, routing, or DNS issue)
- `CMS Online` – Anthias API reachable and returned assets (count displayed)
- `CMS Offline` – Anthias API failed; reboot action enabled

## Useful commands

```bash
# Tail the live log on a remote host
ssh nsm@192.168.10.147 "tail -f /home/nsm/nsm-app/nsm.log"

# Check which process owns port 8080 (port guard messages will appear locally too)
ssh nsm@192.168.10.147 "sudo lsof -i :8080"

# Manually trigger the health SSE stream
curl -N -X POST http://localhost:8080/api/hosts/check-stream

# Push a specific host list (JSON array) to another node
curl -X POST http://192.168.10.147:8080/api/hosts/receive \
  -H 'Content-Type: application/json' \
  -d @hosts.json
```

## Troubleshooting playbook

- **Binary did not start** – check `/home/nsm/nsm-app/nsm.log`; the port guard logs the failure reason before exit.
- **Backups missing** – confirm `/api/hosts/receive` is hit (dashboard button or manual `curl`); the store only rotates when receiving a push.
- **Database refused to open** – check `backups/` for snapshots. On startup the service restores the newest `hosts-*.db` file or creates an empty `hosts.db` if none exist.
- **Health sweep stuck** – open browser dev tools and watch the SSE stream; errors surface in the network tab.
- **RSYNC failures** – re-run the deployer with `--parallel 1` to isolate the failing host and inspect `~/.ssh/nsm-vbox.key` permissions.

## SSH setup

Passwordless SSH speeds up deployments:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/nsm-vbox.key
for host in 192.168.10.{147,174,135,211}; do
  ssh-copy-id -i ~/.ssh/nsm-vbox.key.pub nsm@$host
done
```

