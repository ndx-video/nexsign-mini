# nexSign mini (nsm)

`nsm` is a decentralized service written in Go that provides automatic discovery, monitoring, and *management* for a network of [Anthias](https://www.anthias.io/) digital signage players.

## Project Goals

The primary goal of `nsm` is to create a zero-configuration, resilient, and lightweight monitoring and management solution for Anthias hosts, especially those running on System-on-Chip (SoC) hardware. It achieves this by:

1.  **Automatic Discovery:** `nsm` instances automatically find each other on the local network using mDNS, requiring no central server.
2.  **Distributed State & Audit:** It maintains a distributed ledger of all known Anthias hosts using the Tendermint consensus engine. This ledger stores host metadata and provides an **immutable, signed audit log** for all management actions.
3.  **Centralized Interface:** It provides a simple web dashboard that aggregates all discovered Anthias players, allowing users to monitor status and manage hosts from a single interface.
4.  **API First:** The service includes a simple REST API to allow for integration with third-party services (e.g., n8n, Kestra).

## Architecture

The `nsm` service is composed of several key components:

* **Node Identity:** On first boot, each `nsm` instance generates a persistent `ed25519` keypair (`nsm_key.pem`). The **public key** serves as the node's permanent, unique ID within the distributed network.
* **mDNS Discovery:** A service that constantly browses for and announces `_nsm._tcp` services on the local network.
* **Tendermint Consensus (ABCI):** An application that interfaces with Tendermint Core. It manages the state of the distributed ledger and processes two types of transactions:
    * **State Transactions:** (e.g., `add_host`, `update_status`) for updating host metadata.
    * **Action Transactions:** (e.g., `restart_host`) which are signed by the originating node and executed by the target node, providing a built-in audit trail.
* **Anthias Client:** A component that polls the local Anthias instance to gather its status and metadata.
* **Web Server:** A native Go web server serving:
    * A web dashboard (using HTMX) with an `<iframe>` to display the selected Anthias host's UI.
    * A JSON REST API for external integrations.

### Host Data Model

The core data structure for each host stored in the ledger (mirrors `internal/types/Host`):

```go
type Host struct {
    Hostname       string `json:"hostname"`
    IPAddress      string `json:"ip_address"`
    AnthiasVersion string `json:"anthias_version"`
    AnthiasStatus  string `json:"anthias_status"`
    DashboardURL   string `json:"dashboard_url"`
    PublicKey      string `json:"public_key"` // hex-encoded ED25519 public key
}
```

---

## Quick start

Prereqs

- Linux native filesystem recommended (to avoid key-permission issues).
- Go 1.21+ installed.
- Tendermint Core v0.35.x installed and on PATH.

Run the nsm ABCI app and web UI

1) Start nsm (generates `nsm_key.pem` on first run and starts ABCI on `unix://nsm.sock`):

   - `go run cmd/nsm/main.go`

2) Initialize and start Tendermint in another terminal:

   - `tendermint init --home $HOME/.tendermint`
   - `tendermint node --home $HOME/.tendermint --proxy_app=unix://nsm.sock`

3) Open the dashboard at http://localhost:8080 (override with `PORT` env var).

Useful env vars

- `KEY_FILE` (default `nsm_key.pem`)
- `HOST_DATA_FILE` (default `test-hosts.json`)
- `PORT` (default `8080`)
- `MDNS_SERVICE_NAME` (default `_nsm._tcp`)
- `ANTHIAS_POLL_INTERVAL_SECS` (default `30`)
- `TENDERMINT_RPC` (default `http://localhost:26657`)

## Broadcasting transactions

Transactions are signed JSON and submitted to Tendermint RPC as base64-encoded bytes.

High-level flow:

- Build a `types.Transaction` (e.g., `TxUpdateStatus`).
- Sign it with your node identity to get `types.SignedTransaction`.
- Broadcast via `internal/tendermint.BroadcastClient`.

Example (Go):

```go
id := identity.NewIdentity(privKey)
bc := tendermint.NewBroadcastClient("http://localhost:26657")

payload := types.UpdateStatusPayload{Status: "Online", LastSeen: time.Now()}
payloadBytes, _ := json.Marshal(payload)

tx := &types.Transaction{
    Type:      types.TxUpdateStatus,
    Timestamp: time.Now(),
    Payload:   payloadBytes,
}

stx, _ := tx.Sign(id)
hash, err := bc.BroadcastSignedTransaction(stx)
if err != nil {
    log.Fatalf("broadcast failed: %v", err)
}
log.Printf("tx hash: %s", hash)
```

Notes

- Tendermint JSON-RPC expects the `tx` parameter to be base64, even if the payload itself is JSON. The included client handles this automatically.
- For critical paths, use `BroadcastSignedTransactionCommit` to wait for inclusion in a block.

## Anthias polling

`nsm` includes a background poller that periodically:

- Commits an `add_host` for this node (once, to register the signer), and
- Broadcasts `update_status` when the local Anthias status changes.

Knobs

- `ANTHIAS_POLL_INTERVAL_SECS` controls the poll cadence.
- `TENDERMINT_RPC` points to the Tendermint RPC endpoint (default `http://localhost:26657`).

## ⚖️ Licensing

`nsm` is a dual-licensed project.

* **Community Edition (Open Source):** Licensed under the **GPLv3** (see `LICENSE`). We chose the GPLv3 to ensure the project and its core remain open and free forever.
* **Commercial License:** For businesses and use cases incompatible with the GPLv3 (e.g., closed-source applications, proprietary firmware), a commercial license is available from NDX Pty Ltd. See `COMMERCIAL-LICENSE.md` for details.

### Contributing

We welcome community contributions! Please note that all contributors are required to sign a **Contributor License Agreement (CLA)**. This is necessary to allow NDX Pty Ltd to offer the dual-license model that funds the project's long-term development.

For more details, please see our `CONTRIBUTING.md` file.