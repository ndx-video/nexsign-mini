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

## Phase 2: Core Architecture (✅ Complete)

This phase established the simplified manual host management architecture.

- **[✅] Data Models (`internal/types`):**
    - `Host` struct with IP address, hostname, status, version, dashboard URL, and health timestamps.
    - Status types: `unreachable`, `connection_refused`, `unhealthy`, `healthy`.
    - Removed transaction-based model in favor of direct HTTP API.

- **[✅] Host Store (`internal/hosts`):**
    - Thread-safe host list management.
    - JSON file persistence (`hosts.json`).
    - CRUD operations: Add, Update, Delete, ReplaceAll.
    - Automatic creation of empty store on first run.

- **[✅] Health Checking:**
    - TCP connection testing.
    - HTTP health endpoint verification.
    - Status determination logic.
    - Bulk health check support.

- **[✅] Anthias Client (`internal/anthias`):**
    - Package for polling local Anthias instance.
    - Metadata collection (hostname, IP, version, status).
    - Automatic localhost registration.

---

## Phase 3: Web Dashboard and API (✅ Complete)

This phase implements the user-facing dashboard and synchronization API.

- **[✅] Web Server (`internal/web`):**
    - HTMX-based dashboard for host management.
    - Table view with inline editing capabilities.
    - Manual add host form with IP (required) and hostname (optional).
    - Delete host functionality with confirmation.
    - Auto-refresh host list every 5 seconds.

- **[✅] API Endpoints:**
    - `GET /api/health` - Service health check.
    - `GET /api/hosts` - Get all hosts.
    - `POST /api/hosts/add` - Add new host.
    - `POST /api/hosts/update` - Update existing host (inline editing).
    - `POST /api/hosts/delete` - Delete a host.
    - `POST /api/hosts/check` - Trigger health check on all hosts.
    - `POST /api/hosts/push` - Push host list to all other hosts.
    - `POST /api/hosts/receive` - Receive pushed host list from peers.

- **[✅] Host Management Features:**
    - Inline editing of IP and hostname (click edit icon).
    - Status indicators with color coding (green=healthy, red=unreachable, etc.).
    - Manual health check trigger button.
    - Push to network button (appears when 2+ hosts exist).

---

## Phase 4: Dashboard Enhancement and Usability (⏳ In Progress)

This phase improves the UI/UX and adds convenience features.

### Core Functionality

- **[⏳] Real-Time UI Updates:**
    - **[✅]** HTMX polling for automatic host list refresh (5s interval).
    - **[⏳]** Add WebSocket support for instant push notifications.
    - **[⏳]** Show last update timestamp.
    - **[⏳]** Add loading states and error handling.

- **[⏳] Host Management:**
    - **[⏳]** Add host grouping/tagging.
    - **[⏳]** Bulk actions (check multiple hosts, delete multiple).
    - **[⏳]** Host filtering and search.
    - **[⏳]** Sort by IP, hostname, or status.
    - **[⏳]** Import/export host list (CSV, JSON).

### Dashboard Enhancements

- **[⏳] Navigation Menu:**
    - **[⏳]** Add top navigation bar with menu items:
        - **Dashboard** (home/host list)
        - **Network** (sync status, push history)
        - **Configuration** (view/edit config)
        - **Logs** (view application logs)
        - **About** (version, node info, help)

- **[⏳] Network Utilities:**
    - **[⏳]** Push history (track when lists were synchronized).
    - **[⏳]** Show which hosts are reachable for push operations.
    - **[⏳]** Manual sync retry for failed pushes.
    - **[⏳]** Conflict resolution UI (if receiving different host lists).

- **[⏳] System Utilities:**
    - **[⏳]** Node health dashboard (CPU, memory, disk, network).
    - **[⏳]** Log viewer with filtering and search.
    - **[⏳]** Configuration editor with validation.
    - **[⏳]** Backup/restore functionality (export/import hosts.json).

- **[⏳] Monitoring and Alerts:**
    - **[⏳]** Host status timeline (show status changes over time).
    - **[⏳]** Alert rules (notify when host goes offline).
    - **[⏳]** Dashboard widgets (total hosts, online/offline counts).
    - **[⏳]** Export metrics for Prometheus/Grafana.

- **[⏳] UI/UX Improvements:**
    - **[✅]** Tailwind CSS for styling.
    - **[⏳]** Responsive design for mobile/tablet.
    - **[⏳]** Dark mode support.
    - **[⏳]** Keyboard shortcuts for common actions.
    - **[⏳]** Accessibility improvements (ARIA labels).

---

## Phase 5: Hardening and Production Deployment (⏳ In Progress)

This phase focuses on security, testing, and production readiness.

- **[⏳] Security:**
    - **[✅]** Tailnet-ready design (no built-in authentication).
    - **[⏳]** Add HTTPS/TLS support for web dashboard.
    - **[⏳]** Optional basic auth for non-Tailnet deployments.
    - **[⏳]** CSRF protection for state-changing operations.
    - **[⏳]** Rate limiting for API endpoints.

- **[⏳] Testing:**
    - **[⏳]** Unit tests for host store operations.
    - **[⏳]** Integration tests for API endpoints.
    - **[⏳]** End-to-end tests with multiple nodes.
    - **[⏳]** Load testing for multi-host scenarios.

- **[⏳] Build & Packaging:**
    - **[✅]** Sample systemd service unit.
    - **[⏳]** Create `Makefile` for build automation.
    - **[⏳]** Cross-platform builds (Linux ARM64, AMD64).
    - **[⏳]** Release artifacts and versioning.

- **[⏳] Deployment:**
    - **[⏳]** Deployment documentation updates.
    - **[⏳]** Docker/container deployment option.
    - **[⏳]** Ansible playbook for fleet deployment.
    - **[⏳]** Auto-update mechanism.

---

## Phase 6: Advanced Features (⏳ Future)

This phase adds advanced capabilities for larger deployments.

### State Management

- **[⏳]** Conflict resolution strategies for concurrent updates.
- **[⏳]** Version tracking for host list changes.
- **[⏳]** Audit log for all modifications.
- **[⏳]** Rollback capability (restore previous host list).

### Federation and Multi-Site

- **[⏳]** Support for multiple independent networks.
- **[⏳]** Cross-site host list synchronization.
- **[⏳]** Site-specific host grouping.
- **[⏳]** Geographic/latency-aware health checking.

### Observability and Operations

- **[⏳]** Prometheus metrics endpoint.
- **[⏳]** OpenTelemetry tracing.
- **[⏳]** Structured logging (JSON output).
- **[⏳]** Health check endpoints for load balancers.
- **[⏳]** Automatic backup scheduling.

### API and Integration

- **[⏳]** RESTful API expansion for all operations.
- **[⏳]** WebSocket API for real-time events.
- **[⏳]** CLI tool for administrative tasks.
- **[⏳]** Python/JavaScript SDK for external integration.
- **[⏳]** n8n/Kestra integration examples.

### Advanced Dashboard Features

- **[⏳]** Multi-host comparison view.
- **[⏳]** Historical analytics and reporting.
- **[⏳]** Custom scripts/workflows.
- **[⏳]** Mobile app (iOS/Android).
- **[⏳]** Progressive Web App (PWA) support.

---

## Current Status Summary

**Completed:** Phase 1, Phase 2, Phase 3 (core functionality)  
**In Progress:** Phase 4 (UI enhancements), Phase 5 (hardening)  
**Next Priority:**
1. Add WebSocket support for real-time updates
2. Improve error handling and user feedback
3. Add comprehensive testing
4. Deployment automation and documentation

**Architecture:** Manual host management with HTTP-based synchronization  
**Security Model:** Tailnet-based (no authentication required)  
**Lines of Code:** ~1,500+ (simplified architecture)  
**Documentation:** Comprehensive README and inline documentation