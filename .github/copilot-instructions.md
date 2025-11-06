# copilot-instructions for nexSign mini (nsm)

This file contains concise, actionable notes for automated coding agents working on `nexsign-mini`.

Key guidance (keep answers short and code-focused):

- Read entrypoint: `main.go` to understand startup order (host store -> Anthias client -> web server).
- Primary domain code lives under `internal/` (hosts, anthias, web, types).
- Hosts are indexed by IP address. See `internal/types/types.go` and `internal/hosts/store.go` for host management.

Essential files to reference in pull requests and changes:

- `main.go` — startup, environment knobs, logger configuration
- `internal/hosts/store.go` — host list management, JSON persistence
- `internal/hosts/health.go` — health checking logic
- `internal/types/types.go` — definitions for `Host` and `HostStatus`
- `internal/web/server.go` — API handlers and routes
- `internal/web/home-view.html` — HTMX dashboard template

Environments and common dev commands

- Defaults found in code (override with env vars):
  - `PORT=8080`

- Run locally:
```bash
go run main.go
```
- Build & test:
```bash
go build
go test ./...
```

Key files and where to look

- `main.go` — startup order and env knobs
- `internal/hosts/store.go` — thread-safe host list management with JSON persistence
- `internal/hosts/health.go` — health checking (TCP + HTTP verification)
- `internal/types/types.go` — `Host` struct and `HostStatus` constants
- `internal/web/` — HTMX templates and `server.go` (dashboard + API)

Conventions and important rules (project-specific)

- Data Storage: `hosts.json` is the source of truth for the host list. Created automatically if missing.
- Host Management: All operations go through the `hosts.Store` for thread-safety.
- Status Values: `unreachable`, `connection_refused`, `unhealthy`, `healthy`
- API Authentication: None - designed for Tailnet deployments
- Synchronization: Manual push via "Push to Other Hosts" button

Roadmap & priorities (developer-facing highlights)

- Phase 1-3: Core functionality complete (manual host management, health checking, web UI)
- Phase 4 priority: UI/UX improvements, WebSocket support, better error handling
- Phase 5 priority: Testing, deployment automation, optional HTTPS/auth for non-Tailnet deployments

Practical snippets and commands

- Build:
```bash
go build -o nsm
```
- Run with custom port:
```bash
PORT=9090 go run main.go
```

Checklist for automated edits

1. Run `go test ./...` before creating a PR.
2. Add unit tests when modifying host store or health checking logic.
3. Keep changes minimal and focused on one feature at a time.

Architecture notes

- No distributed ledger - simple HTTP-based synchronization
- No authentication - relies on Tailnet for security
- No mDNS discovery - hosts added manually
- No cryptographic signing - trust-all model
- Localhost always included automatically from Anthias client polling
```bash
```markdown
# copilot-instructions for nexSign mini (nsm)

Concise, actionable notes for automated coding agents working on this repository.

Quick start (local, Linux native filesystem required)

- Ensure the project lives on a native Linux filesystem (not a Windows-mounted drive). Files like `nsm_key.pem` require POSIX permissions; Windows mounts can cause permission and SSH key issues.
- Run the service:
```bash
go run cmd/nsm/main.go
```
- Build & test:
```bash
go build ./...
go test ./...
```

Key files and where to look

- `cmd/nsm/main.go` — startup order and env knobs
- `internal/identity/identity.go` — keypair load/generation and helper `GetPublicKeyHex`
- `internal/abci/app.go` — CheckTx/DeliverTx, ABCI response codes, and state mutations
- `internal/types/types.go` — `Host`, transaction payloads, `SignedTransaction` shape
- `internal/web/` — HTMX templates and `internal/web/server.go` (dashboard + API)

Conventions and important rules (project-specific)

- Identity: `nsm_key.pem` is the permanent node identity. Treat as sensitive; ensure it is created with `0600` permissions.
- Transactions: All state-changing operations are signed. `SignedTransaction` contains `PublicKey`, `Tx` (bytes), and `Signature`. `CheckTx` must validate signatures using `ed25519.Verify`.
- State keys: Hosts are indexed by their public key hex string — do not use ephemeral IPs as unique identifiers.
- ABCI codes: use the codes defined in `internal/abci/app.go` (0 OK, 1 encoding error, 2 auth error, 3 invalid tx).

MCP / Agentic development

- A minimal MCP scaffold exists at `.mcp-workspace/` for local agent testing (JSON-RPC `/rpc` initialize and `/ping`). Default listen: `localhost:4000`.
- If an agent reports repeated "Waiting for server to respond to `initialize`":
  1. Confirm `curl http://localhost:4000/ping` returns `pong`.
 2. Confirm the agent is configured to use `http://localhost:4000/rpc` for JSON-RPC initialize.
 3. Increase client timeouts during debugging or add a health endpoint.

Roadmap & priorities (developer-facing highlights)

- Phase 2 priority: implement Node Identity, mDNS discovery, and transaction data models (`types`).
- Tendermint integration: wire an ABCI app to Tendermint and seed peers from mDNS.
- Phase 3 priority: implement strict `CheckTx` signature checks and `DeliverTx` state logic (State vs Action transactions). If action targets local node, execute privileged actions.
- Phase 4: real-time UI updates (HTMX polling or `hx-ws`) and host actions that create/signed/broadcast `ActionTransaction`.
- Phase 5: hardening — permissions for `nsm_key.pem`, packaging, `Makefile`, and `deploy.sh` with `systemd` unit.

Practical snippets and commands

- Build & run MCP scaffold:
```bash
cd .mcp-workspace
go build -o mcpserver
./mcpserver
```
- Test MCP endpoints:
```bash
curl http://localhost:4000/ping
curl -sS -X POST http://localhost:4000/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

Notes about GEMINI.md and ROADMAP.md

- `GEMINI.md`: contains developer prompt-level rules (markdown style, sensitivity of identity file, logging, resource constraints). If you want these rules merged differently, paste any additional sections here and I'll integrate them.
- `ROADMAP.md` was audited and its phase priorities are summarized above — use them to prioritize PRs and tests.

Checklist for automated edits

1. Run `go test ./...` before creating a PR.
2. Add unit tests when modifying ABCI behavior, especially `CheckTx` and `DeliverTx`.
3. Keep changes minimal and add one integration test when touching consensus integration.

If anything is missing or you want a stricter template, tell me which file to edit and I'll update the instructions.
```