# nexSign mini roadmap

The roadmap is intentionally short and focuses on the pieces that keep the manual fleet manager reliable, understandable, and easy to deploy. Items move from “Delivered” → “In progress” → “Queued”.

## Delivered

- Go deployer (`cmd/deployer`) that builds once, syncs HTMX assets via rsync, and relaunches remote services with `setsid`
- Port guard in `main.go` that fails fast when port `8080` is already in use
- SQLite-backed host store with automatic recovery, WAL snapshots, and rolling backups in `backups/`
- HTMX dashboard refinements: NSM Online deep link, aligned action controls, countdown pause while editing, SSE progress feed for health sweeps
- Host health updates that report NSM version, NSM status text, Anthias CMS status, and asset counts

## In progress

- Broader test coverage for the host store, health checker edge cases, and dashboard handlers
- Improved error surfacing in the UI (inline toasts instead of relying on the browser alert)
- Deployment notes for non-lab environments (systemd unit, log rotation, default firewall rules)

## Queued

- Optional HTTPS termination and basic auth for deployments outside a Tailnet
- Real-time push notifications (websocket or HTMX SSE channel) for host list mutations
- Dashboard filtering and search for large fleets
- Export/import tooling for host roster snapshots
- Packaging work: multi-arch binaries, `.deb` wrapper, and release automation

If a task is missing or needs reprioritising, open an issue with the context that prompted the change.
