# nexSign mini (nsm)

nexSign mini is a lightweight Go service that keeps a manually curated roster of Anthias signage hosts in sync. It provides a single-page dashboard for status checks, inline edits, and one-click pushes to the rest of the fleet.

## Features

- Manual host management with inline edits, deletions, and instant NSM dashboard links
- Health checks that capture TCP reachability, NSM API status, Anthias CMS state, and asset counts
- Push-to-fleet workflow that snapshots the previous SQLite database into `backups/hosts-<epoch>.db` and trims the archive to the newest twenty copies
- Port guard that refuses to start when another process already owns port `8080`
- HTMX-driven web UI served directly from the Go standard library
- REST API for automations and fleet tooling

## Architecture

- **Host store** – thread-safe wrapper around `hosts.db` (SQLite). It self-heals on launch by restoring the most recent backup or rebuilding an empty database, and rotates snapshots in `backups/`
- **Health checker** – runs targeted TCP and HTTP probes to classify hosts as `unreachable`, `connection_refused`, `unhealthy`, `healthy`, or `stale`
- **Anthias client** – polls the local player for metadata and ensures the localhost entry is always present
- **Web server** – renders the HTMX dashboard, exposes the REST API, and streams Server-Sent Events during health sweeps

### Host data model

```go
type Host struct {
    Nickname          string           `json:"nickname"`
    IPAddress         string           `json:"ip_address"`
    VPNIPAddress      string           `json:"vpn_ip_address"`
    Hostname          string           `json:"hostname"`
    Notes             string           `json:"notes"`
    Status            HostStatus       `json:"status"`
    StatusVPN         HostStatus       `json:"status_vpn"`
    NSMStatus         string           `json:"nsm_status"`
    NSMStatusVPN      string           `json:"nsm_status_vpn"`
    NSMVersion        string           `json:"nsm_version"`
    NSMVersionVPN     string           `json:"nsm_version_vpn"`
    AnthiasVersion    string           `json:"anthias_version"`
    AnthiasVersionVPN string           `json:"anthias_version_vpn"`
    AnthiasStatus     string           `json:"anthias_status"`
    AnthiasStatusVPN  string           `json:"anthias_status_vpn"`
    CMSStatus         AnthiasCMSStatus `json:"cms_status"`
    CMSStatusVPN      AnthiasCMSStatus `json:"cms_status_vpn"`
    AssetCount        int              `json:"asset_count"`
    AssetCountVPN     int              `json:"asset_count_vpn"`
    DashboardURL      string           `json:"dashboard_url"`
    DashboardURLVPN   string           `json:"dashboard_url_vpn"`
    LastChecked       time.Time        `json:"last_checked"`
    LastCheckedVPN    time.Time        `json:"last_checked_vpn"`
}
```

Status reference:

- `healthy` – NSM dashboard reachable and responsive
- `stale` – NSM running but reporting an older build
- `unhealthy` – TCP open but health endpoint failed
- `connection_refused` – host reachable but nothing listening on `8080`
- `unreachable` – network or DNS failure

When a host is healthy, the dashboard renders a green “NSM Online” link that points to `http://<ip>:8080`.

## Quick start

Prerequisites:

- Go 1.24+
- Linux or WSL2 on a native filesystem (to satisfy file permission requirements)

Start a local instance:

```bash
go run main.go
```

The first launch creates `hosts.db`, migrates any legacy `hosts.json`, and adds the current node. Visit `http://localhost:8080` to manage the roster. Set `PORT` to override the default listener.

## Deploying to a fleet

Use the Go-based deployer to build once and copy the binary plus web assets to every test host:

```bash
go run cmd/deployer/main.go --hosts all --parallel 4
```

Key flags:

- `--hosts` – comma-separated IPs or `all` for the default VirtualBox lab
- `--parallel` – concurrency limit for rsync + SSH operations
- `--skip-build` – reuse the existing `nsm` binary
- `--remote-dir` – target directory on the remote host (default `/home/nsm/nsm-app`)

The deployer stops any running instance, wipes the remote directory, synchronizes the binary and HTMX assets, and relaunches the service in the background using `setsid` + `nohup`.

## API surface

- `GET /api/health` – health probe for the local service
- `GET /api/version` – returns the running NSM build identifier
- `GET /api/hosts` – returns the current host list
- `POST /api/hosts/add` – append a host (expects JSON payload)
- `POST /api/hosts/update` – edit an existing host by IP
- `POST /api/hosts/delete?ip=<ip>` – remove a host
- `POST /api/hosts/check` – trigger a background health sweep
- `POST /api/hosts/check-stream` – Server-Sent Events stream for sweep progress
- `POST /api/hosts/push` – broadcast the current list to every other host
- `POST /api/hosts/receive` – replace the local list (creates a timestamped SQLite snapshot first and rotates prior copies)
- `POST /api/hosts/reboot` – forward reboot requests to an Anthias player
- `POST /api/hosts/upgrade` – forward package upgrade requests

## Development workflow

- Run the test suite: `go test ./...`
- Format Go code with `gofmt -w`
- Dashboard tweaks live in `internal/web/home-view.html` and `internal/web/layout.html`
- Host persistence (`hosts.db` + backups), health checks, and snapshot helpers live under `internal/hosts`

## Licensing and commercial support

- Community edition: GPLv3 (see `LICENSE`)
- Commercial license: contact NDX Pty Ltd (details in `COMMERCIAL-LICENSE.md`)

## Contributing

Read `CONTRIBUTING.md` for the dual-licensing rationale and CLA process. Contributions that include tests and documentation updates are greatly appreciated.
