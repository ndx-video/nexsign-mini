# nexSign mini (nsm) Development Roadmap

---

### Phase 1: Foundation and Prototyping (✓ Complete)

- **[✓]** Set up a standard Go project structure.
- **[✓]** Implemented a dynamic web dashboard using Go templates and HTMX.
- **[✓]** Established a non-blocking development cycle with background process execution.
- **[✓]** Implemented a robust, configurable logging system.
- **[✓]** Externalized all configuration and test data.
- **[✓]** Created skeleton modules for all major components.

---

### Phase 2: Identity, P2P Network, and Consensus

This phase focuses on node identity and integrating the core Tendermint P2P and consensus engine.

- **[ ] Implement Node Identity:**
    - Create an `identity` package that loads or generates a persistent `ed25519` keypair (`nsm_key.pem`).
    - Expose the public key as the node's canonical ID.
- **[ ] Full mDNS Discovery:** Implement logic in the `discovery` service to browse for other `_nsm._tcp` instances and maintain a thread-safe list of discovered peers.
- **[ ] Define Data Models:**
    - Create a `types` package defining the `Host` struct.
    - Define the transaction structures, e.g., `StateTransaction` (for `add_host`, `update_status`) and `ActionTransaction` (for `restart_host`). Both must include a `Signature` field.
- **[ ] Tendermint Core Integration:**
    - Integrate Tendermint Core as an in-process component.
    - Establish the connection between our custom ABCI application and the Tendermint engine.
    - Use the discovered peer list from mDNS to seed Tendermint's P2P layer (persistent peers).

---

### Phase 3: State Management and ABCI Logic

This phase implements the "brains" of the application—the business logic that runs on the blockchain.

- **[ ] Implement ABCI Signature Logic:**
    - Implement `CheckTx`: This is the security gate. It **must** deserialize the transaction, and for all `ActionTransaction` types, it must verify the `Signature` against the originating node's public key (stored in the ledger state). Reject invalid signatures.
- **[ ] Implement ABCI State Logic:**
    - Implement `DeliverTx`: This is the business logic.
    - For `StateTransaction`: Update the ledger (e.g., add a new `Host` entry, update the `Status` or `LastSeen` time of an existing `Host`).
    - For `ActionTransaction`: Check if the *target* host (in the transaction payload) matches the *local* node's public key. If it matches, **execute the privileged command** (e.g., `exec.Command("systemctl", "restart", "anthias")`).
- **[ ] Implement Anthias Client:** Replace mock data with real logic to poll the local Anthias instance for its status.
- **[ ] Create the Main Event Loop:** Implement a central loop that periodically:
    1.  Polls the local Anthias instance.
    2.  Compares its status with the last known status in the ledger.
    3.  Signs and broadcasts a `StateTransaction` (`update_status`) if a change is detected.

---

### Phase 4: Enhancing the Dashboard and API

This phase makes the UI dynamic and functional for management.

- **[ ] Real-Time UI Updates:** Use HTMX polling or WebSockets (`hx-ws`) to make the dashboard's host list update automatically as the Tendermint ledger state changes.
- **[ ] Implement Host Actions:**
    - Build out the "Actions" (e.g., "Restart") links in the dashboard.
    - Clicking an action will trigger the *local* `nsm` service (the one serving the UI) to:
        1.  Create an `ActionTransaction` (e.g., `restart_host` for target `[pubkey]`).
        2.  Sign this transaction with its *own* private key.
        3.  Broadcast it to the Tendermint network.
- **[ ] Implement Friendly Names:** Add the UI and API endpoints to allow the user to set/update the `FriendlyName` field for any host in the ledger.

---

### Phase 5: Hardening and Deployment

This phase focuses on making the application reliable and easy to deploy.

- **[ ] Security:**
    - Ensure the `nsm_key.pem` file is saved with `0600` permissions.
    - *Note: Inter-node comms security is handled by the assumed Tailscale/WireGuard overlay.*
- **[ ] Comprehensive Testing:** Write unit and integration tests.
- **[ ] Build & Packaging:** Create a `Makefile` to standardize building the `nsm` binary.
- **[ ] Deployment Script:** Create a `deploy.sh` script that:
    1.  Builds the binary (or pulls from a release).
    2.  Copies it to `/usr/local/bin`.
    3.  Creates and enables a `systemd` service file (`nsm.service`) to run the service as `root` (as required for privileged actions) and ensure it starts on boot.
- **[ ] Deployment Documentation:** Write clear instructions for the `deploy.sh` script.