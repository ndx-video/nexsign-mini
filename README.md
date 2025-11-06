# nexSign mini (nsm)

`nsm` is a lightweight service written in Go that provides manual network management and monitoring for a network of [Anthias](https://www.anthias.io/) digital signage players.

## Project Goals

The primary goal of `nsm` is to create a simple, lightweight monitoring and management solution for Anthias hosts, especially those running on System-on-Chip (SoC) hardware. It achieves this by:

1.  **Manual Host Management:** `nsm` provides a simple web dashboard where users can manually add, edit, and remove Anthias hosts on their network.
2.  **Network Synchronization:** Host lists can be manually pushed to all other `nsm` instances on the network, keeping the fleet in sync.
3.  **Centralized Interface:** It provides a simple web dashboard that displays all managed Anthias players, allowing users to monitor status and manage hosts from a single interface.
4.  **API First:** The service includes a simple REST API to allow for integration with third-party services (e.g., n8n, Kestra).
5.  **Tailnet-Ready:** Designed to work seamlessly with Tailscale/Tailnet for secure network access without complex authentication.

## Architecture

The `nsm` service is composed of several key components:

* **Host Store:** Manages the `hosts.json` file which stores the list of all Anthias hosts. If the file doesn't exist at startup, it's created automatically.
* **Health Checker:** Periodically checks the health status of each host (unreachable, connection refused, unhealthy, healthy).
* **Anthias Client:** A component that polls the local Anthias instance to gather its status and metadata.
* **Web Server:** A native Go web server serving:
    * A web dashboard (using HTMX) for managing the host list.
    * A JSON REST API for external integrations and host synchronization.

### Host Data Model

The core data structure for each host stored in `hosts.json`:

```go
type Host struct {
    IPAddress      string     `json:"ip_address"`      // Required: IP address of the host
    Hostname       string     `json:"hostname"`        // Optional: friendly name for the host
    Status         HostStatus `json:"status"`          // Health status
    AnthiasVersion string     `json:"anthias_version"` // Detected Anthias version
    AnthiasStatus  string     `json:"anthias_status"`  // Anthias service status
    DashboardURL   string     `json:"dashboard_url"`   // URL to host's dashboard
    LastChecked    time.Time  `json:"last_checked"`    // Last time status was checked
}
```

Status values: `unreachable`, `connection_refused`, `unhealthy`, `healthy`

---

## Quick start

Prereqs

- Linux native filesystem recommended.
- Go 1.21+ installed.

Run the nsm service

1) Start nsm:

   ```bash
   go run main.go
   ```

   On first run, if `hosts.json` doesn't exist, it will be created automatically. The localhost entry is added automatically.

2) Open the dashboard at http://localhost:8080 (override with `PORT` env var).

3) Add hosts manually using the form at the bottom of the dashboard.

4) Check host health status by clicking "Check All Hosts".

5) When ready, push the host list to all other hosts using the "Push to Other Hosts" button.

Useful env vars

- `PORT` (default `8080`)

## Managing Hosts

### Adding a Host

Use the web form at the bottom of the host list to add a new host. Only the IP address is required; the hostname is optional.

### Editing a Host

Click the edit icon (✏️) next to any host to edit its IP address or hostname inline. Click save (💾) to commit changes or cancel (❌) to discard.

### Deleting a Host

Click the delete icon (🗑️) next to any host to remove it from the list.

### Checking Host Health

Click "Check All Hosts" to trigger a health check on all hosts. Status indicators will update automatically:

- 🟢 Green: Healthy
- 🟡 Yellow: Unhealthy
- 🟠 Orange: Connection Refused
- 🔴 Red: Unreachable

### Synchronizing the Network

When you have more than one host in your list, a "Push to Other Hosts" button appears. Click this to send your current host list to all other hosts on the network. They will automatically update their `hosts.json` files.

## API Endpoints

- `GET /api/health` - Health check endpoint
- `GET /api/hosts` - Get all hosts
- `POST /api/hosts/add` - Add a new host
- `POST /api/hosts/update` - Update an existing host
- `POST /api/hosts/delete?ip=<ip>` - Delete a host
- `POST /api/hosts/check` - Trigger health check on all hosts
- `POST /api/hosts/push` - Push host list to all other hosts
- `POST /api/hosts/receive` - Receive pushed host list (internal)

## Anthias polling

`nsm` includes a background poller that periodically updates the localhost entry with current Anthias status information every 30 seconds.

## ⚖️ Licensing

`nsm` is a dual-licensed project.

* **Community Edition (Open Source):** Licensed under the **GPLv3** (see `LICENSE`). We chose the GPLv3 to ensure the project and its core remain open and free forever.
* **Commercial License:** For businesses and use cases incompatible with the GPLv3 (e.g., closed-source applications, proprietary firmware), a commercial license is available from NDX Pty Ltd. See `COMMERCIAL-LICENSE.md` for details.

### Contributing

We welcome community contributions! Please note that all contributors are required to sign a **Contributor License Agreement (CLA)**. This is necessary to allow NDX Pty Ltd to offer the dual-license model that funds the project's long-term development.

For more details, please see our `CONTRIBUTING.md` file.