# nexSign mini (nsm) Development Roadmap

---

## Phase 1: Foundation and Prototyping (✅ Complete)

- **[✅]** Set up a standard Go project structure.
- **[✅]** Implemented a dynamic web dashboard using Go templates and HTMX.
- **[✅]** Established a non-blocking development cycle with background process execution.
- **[✅]** Implemented a robust, configurable logging system with rotation (lumberjack).
- **[✅]** Externalized all configuration and test data.
- **[✅]** Created skeleton modules for all major components.
- **[✅]** Added comprehensive README documentation to every package.

---

## Phase 2: Identity, Discovery, and Data Models (✅ Complete)

This phase established node identity, local network discovery, and core data structures.

- **[✅] Node Identity (`internal/identity`):**
    - Implemented `Identity` type with ED25519 keypair management.
    - Load or generate persistent keypair from `nsm_key.pem`.
    - Sign and verify arbitrary data with the node's private key.
    - Enforce 0600 permissions on key files.
    - Public key serves as the node's canonical ID (hex-encoded).

- **[✅] mDNS Discovery (`internal/discovery`):**
    - Announce local service via mDNS (`_nsm._tcp`).
    - Browse and discover peer nodes on the LAN.
    - Thread-safe `PeerStore` maintains discovered peers.
    - Export peer addresses for Tendermint seeding.

- **[✅] Data Models (`internal/types`):**
    - `Host` struct with hostname, IP, status, version, dashboard URL, and public key.
    - Transaction types: `TxAddHost`, `TxUpdateStatus`, `TxRestartHost`.
    - `SignedTransaction` with signature verification.
    - Type-specific payloads: `UpdateStatusPayload`, `RestartHostPayload`.

- **[🟡] Tendermint Integration (Partial):**
    - **[✅]** ABCI application implemented (`internal/abci`).
    - **[✅]** Peer addresses written to file for Tendermint config.
    - **[⏳]** In-process Tendermint Core integration (pending).
    - **[⏳]** Actual transaction broadcasting to Tendermint (pending).

---

## Phase 3: Consensus, State Management, and Action Execution (🟡 Partial)

This phase implements the consensus layer and business logic.

- **[✅] ABCI Application (`internal/abci`):**
    - **[✅]** `CheckTx`: Validates transaction signatures (ED25519), enforces authorization.
    - **[✅]** `DeliverTx`: Executes transactions and updates state.
    - **[✅]** Handles `TxAddHost` (register new nodes).
    - **[✅]** Handles `TxUpdateStatus` (update host status and last-seen).
    - **[✅]** Handles `TxRestartHost` (trigger action if targeting local node).
    - **[✅]** In-memory state indexed by public key.
    - **[✅]** Unit tests for signature validation and state transitions.

- **[✅] Centralized Configuration (`internal/config`):**
    - **[✅]** JSON config file with environment variable overrides.
    - **[✅]** Sensible defaults for development.
    - **[✅]** Graceful degradation when config file missing.
    - **[✅]** Configuration precedence: JSON file → env vars → defaults.

- **[✅] Action Execution (`internal/actions`):**
    - **[✅]** Action router with safe defaults (log-only mode).
    - **[✅]** Direct shell command execution mode.
    - **[✅]** HTTP-based agent delegation mode.
    - **[✅]** Configuration-driven execution via `enable_actions` flag.

- **[✅] Privileged Agent (`internal/agent`):**
    - **[✅]** HTTP server for privileged action execution.
    - **[✅]** Runs as root/elevated privileges for systemctl access.
    - **[✅]** POST `/action` endpoint for action requests.
    - **[✅]** Privilege separation between main process and agent.

- **[⏳] Anthias Client (`internal/anthias`):**
    - **[🟡]** Package structure exists.
    - **[⏳]** Implement actual HTTP polling of local Anthias.
    - **[⏳]** Parse Anthias status and version.
    - **[⏳]** Create and broadcast `TxUpdateStatus` on changes.

- **[⏳] Main Event Loop:**
    - **[⏳]** Periodic polling of local Anthias instance.
    - **[⏳]** Compare current status with ledger state.
    - **[⏳]** Sign and broadcast status update transactions.
    - **[⏳]** Configurable poll interval.

---

## Phase 4: Dashboard Enhancement and Interactivity (⏳ In Progress)

This phase makes the UI functional, interactive, and user-friendly.

### Core Functionality

- **[⏳] Real-Time UI Updates:**
    - **[⏳]** Implement HTMX polling for automatic host list refresh.
    - **[⏳]** Add WebSocket support (`hx-ws`) for instant updates.
    - **[⏳]** Show connection status and last update time.
    - **[⏳]** Add loading states and error handling.

- **[⏳] Host Actions:**
    - **[⏳]** Wire "Restart" button to create and broadcast `TxRestartHost`.
    - **[⏳]** Add confirmation dialogs for destructive actions.
    - **[⏳]** Show action status (pending, success, failed).
    - **[⏳]** Display action history/audit log per host.

- **[⏳] Host Management:**
    - **[⏳]** Add/edit friendly names for hosts.
    - **[⏳]** Add host grouping/tagging.
    - **[⏳]** Bulk actions (restart multiple hosts).
    - **[⏳]** Host filtering and search.

### Dashboard Enhancements

- **[⏳] Navigation Menu:**
    - **[⏳]** Add top navigation bar with menu items:
        - **Dashboard** (home/host list)
        - **Network** (topology view, peer list)
        - **Actions** (action history, pending actions)
        - **Configuration** (view/edit config, manage identity)
        - **Logs** (view application logs)
        - **About** (version, node info, help)

- **[⏳] Network Utilities:**
    - **[⏳]** Network topology visualization (graph of discovered nodes).
    - **[⏳]** Peer connection status and latency.
    - **[⏳]** Manual peer management (add/remove persistent peers).
    - **[⏳]** mDNS discovery status and statistics.

- **[⏳] System Utilities:**
    - **[⏳]** Node health dashboard (CPU, memory, disk, network).
    - **[⏳]** Log viewer with filtering and search.
    - **[⏳]** Configuration editor with validation.
    - **[⏳]** Identity management (view public key, regenerate key with warning).
    - **[⏳]** Backup/restore functionality (export/import state and config).

- **[⏳] Monitoring and Alerts:**
    - **[⏳]** Host status timeline (show status changes over time).
    - **[⏳]** Alert rules (notify when host goes offline, etc.).
    - **[⏳]** Dashboard widgets (total hosts, online/offline counts, recent events).
    - **[⏳]** Export metrics for Prometheus/Grafana.

- **[⏳] UI/UX Improvements:**
    - **[⏳]** Add CSS framework (Bootstrap, Tailwind, or custom).
    - **[⏳]** Responsive design for mobile/tablet.
    - **[⏳]** Dark mode support.
    - **[⏳]** Keyboard shortcuts for common actions.
    - **[⏳]** Accessibility improvements (ARIA labels, screen reader support).

---

## Phase 5: Hardening and Production Deployment (🟡 Partial)

This phase focuses on security, testing, and production readiness.

- **[✅] Security:**
    - **[✅]** Key file permissions enforced (0600).
    - **[✅]** Config file protection documented.
    - **[✅]** Privilege separation via agent.
    - **[⏳]** Add HTTPS/TLS support for web dashboard.
    - **[⏳]** Add basic auth or token-based authentication.
    - **[⏳]** CSRF protection for action submissions.
    - **[⏳]** Rate limiting for API endpoints.

- **[✅] Testing:**
    - **[✅]** Unit tests for identity, types, discovery, ABCI (15 tests).
    - **[⏳]** Integration tests for full transaction flow.
    - **[⏳]** End-to-end tests with multiple nodes.
    - **[⏳]** Load testing for consensus performance.
    - **[⏳]** Chaos testing (network partitions, node failures).

- **[✅] Build & Packaging:**
    - **[✅]** Sample systemd service unit (`deploy/nsm.service.sample`).
    - **[✅]** Sample configuration file (`deploy/config.json.sample`).
    - **[⏳]** Create `Makefile` for build automation.
    - **[⏳]** Cross-platform builds (Linux ARM64, AMD64).
    - **[⏳]** Release artifacts and versioning.

- **[✅] Deployment:**
    - **[✅]** Automated deployment script (`test-deploy.sh`).
    - **[✅]** Comprehensive deployment documentation (`deploy/README.md`).
    - **[✅]** Support for privilege separation (main + agent).
    - **[⏳]** Docker/container deployment option.
    - **[⏳]** Kubernetes manifests.
    - **[⏳]** Ansible playbook for fleet deployment.

---

## Phase 6: State Persistence and Advanced Features (⏳ Future)

This phase adds persistence, advanced networking, and production-grade features.

### State Persistence (`internal/ledger`)

- **[⏳]** Implement `StateStore` interface.
- **[⏳]** JSON file backend for simple persistence.
- **[⏳]** BadgerDB backend for better performance.
- **[⏳]** Periodic snapshots (configurable interval).
- **[⏳]** Snapshot restore on startup.
- **[⏳]** State migration tools for schema changes.

### WAN Discovery and Federation

- **[⏳]** Bootstrap node support for WAN discovery.
- **[⏳]** DHT-based peer discovery (beyond LAN).
- **[⏳]** Federation between multiple nsm clusters.
- **[⏳]** Peer reputation and scoring.
- **[⏳]** Geographic/latency-aware peer selection.

### Advanced Consensus Features

- **[⏳]** Consensus parameter tuning (block time, gas limits).
- **[⏳]** Validator set management.
- **[⏳]** Slashing for misbehaving nodes.
- **[⏳]** Upgrade coordination via consensus.

### Observability and Operations

- **[⏳]** Prometheus metrics endpoint.
- **[⏳]** OpenTelemetry tracing.
- **[⏳]** Structured logging (JSON output).
- **[⏳]** Health check endpoints for load balancers.
- **[⏳]** Graceful shutdown and state persistence.
- **[⏳]** Automatic backup scheduling.

### API and Integration

- **[⏳]** RESTful API for all operations.
- **[⏳]** GraphQL API for flexible queries.
- **[⏳]** WebSocket API for real-time events.
- **[⏳]** CLI tool for administrative tasks.
- **[⏳]** Python/JavaScript SDK for external integration.

### Advanced Dashboard Features

- **[⏳]** Multi-node comparison view.
- **[⏳]** Historical analytics and reporting.
- **[⏳]** Scheduled actions (restart at specific time).
- **[⏳]** Custom scripts/workflows.
- **[⏳]** Mobile app (iOS/Android).
- **[⏳]** Progressive Web App (PWA) support.

---

## Current Status Summary

**Completed:** Phase 1, Phase 2, majority of Phase 3  
**In Progress:** Phase 3 (Anthias client, event loop), Phase 4 (dashboard), Phase 5 (hardening)  
**Next Priority:**
1. Complete Tendermint in-process integration
2. Implement Anthias polling and status broadcasting
3. Add real-time dashboard updates
4. Wire action buttons to transaction creation
5. Add navigation menu and utility pages

**Lines of Code:** ~3,000+ (excluding tests and docs)  
**Test Coverage:** 15 unit tests (identity, types, discovery, ABCI)  
**Documentation:** Comprehensive README files in all packages