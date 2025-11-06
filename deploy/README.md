# deploy resources

This directory contains reference files for installing nexSign mini on bare hosts. The current path to production is intentionally simple: copy the compiled binary, templates, and static assets into `/home/nsm/nsm-app` and run it under a non-privileged user.

## Recommended layout

```text
/home/nsm/nsm-app
├── backups/
├── hosts.db
├── internal/web/*
└── nsm
```

The deployer (`cmd/deployer`) automatically maintains this layout when pointed at a host. It removes the old directory, syncs the new assets with `rsync`, launches the binary with `setsid -f nohup`, and checks that the process stays alive. On first launch the binary will migrate any legacy `hosts.json` into `hosts.db` and keep the JSON file renamed with a `.migrated` suffix for reference.

## Using the Go deployer

```bash
go run cmd/deployer/main.go --hosts 192.168.10.147,192.168.10.174 --parallel 2
```

Flags of note:

- `--remote-dir` overrides the default `/home/nsm/nsm-app`
- `--skip-build` reuses an existing `./nsm` binary
- `--key` sets the SSH private key (defaults to `~/.ssh/nsm-vbox.key`)

Each deployment run:

1. stops any process named `nsm`
2. recreates the remote directory and static asset path
3. synchronises the binary and the `internal/web` subtree
4. relaunches the service and verifies it with `pgrep`

If `setsid` cannot detach or the process fails to start, the deployer surfaces the SSH command output so you can debug the remote host.

## Optional systemd wrapper

The sample `nsm.service.sample` file shows how to supervise the binary with systemd. Adjust paths if you prefer `/opt/nsm` or another location. Reload systemd and enable the unit:

```bash
sudo cp deploy/nsm.service.sample /etc/systemd/system/nsm.service
sudo systemctl daemon-reload
sudo systemctl enable nsm
sudo systemctl start nsm
```

When the service is managed by systemd you can still use the deployer. After syncing files, systemd will notice the new binary on the next restart, or you can call `systemctl restart nsm` in a post-deploy hook.

## Firewall and verification

- open TCP 8080 for the dashboard (`ufw allow 8080/tcp`)
- confirm the service responds: `curl http://localhost:8080/api/health`
- tail the remote log for startup errors: `tail -f /home/nsm/nsm-app/nsm.log`

## Backups

Every time a host receives a pushed roster, the current `hosts.db` is copied into `backups/hosts-<epoch>.db`. Old snapshots are pruned so only the newest twenty remain. If you manage hosts centrally, keep an eye on disk usage under `/home/nsm/nsm-app/backups` and archive or rotate as needed.

