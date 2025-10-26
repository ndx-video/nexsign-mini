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

The core data structure for each host stored in the ledger:

```go
type Host struct {
    PublicKey     string    `json:"public_key"`     // The node's unique, permanent ID
    FriendlyName  string    `json:"friendly_name"`  // A user-editable alias (e.g., "Lobby TV")
    IPAddress     string    `json:"ip_address"`     // The host's last known IP
    MacAddress    string    `json:"mac_address"`    // The MAC address (for stable hardware ID)
    Hostname      string    `json:"hostname"`       // OS hostname
    AnthiasVersion string   `json:"anthias_version"`// Version of the local Anthias instance
    NsmVersion    string    `json:"nsm_version"`    // Version of the nsm service itself
    Status        string    `json:"status"`         // e.g., "Online", "Offline", "Rebooting"
    LastSeen      time.Time `json: "last_seen"`      // Timestamp of the last successful poll
}