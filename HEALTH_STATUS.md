# Host Health Status System

## Overview

The nexSign mini dashboard now includes a comprehensive health monitoring system with real-time visual indicators. The dashboard polls every 3 seconds for updates, ensuring near-instant visibility when hosts go offline or come back online.

## Health Status Types

### 🟢 Online
**Color:** Green  
**Description:** Fully operational  
**Criteria:**
- Heartbeat received within last 30 seconds
- Anthias service responding to HTTP requests  
- All critical services running normally

**Visual:** Solid green indicator dot

---

### 🟡 Degraded
**Color:** Yellow/Amber  
**Description:** Responding with issues  
**Criteria:**
- Heartbeat received but delayed (30-90 seconds)
- Anthias responding but with errors or warnings
- Some non-critical services may be down
- Display may be showing content but with issues

**Visual:** Solid yellow indicator dot  
**Action:** Investigate logs; host may need attention soon

---

### 🔴 Offline
**Color:** Red  
**Description:** Not responding  
**Criteria:**
- No heartbeat received in 90+ seconds
- Anthias HTTP endpoint unreachable
- May indicate network issue, power loss, or crash

**Visual:** Solid red indicator dot  
**Action:** Immediate investigation required

---

### 🔵 Starting
**Color:** Blue  
**Description:** Initializing  
**Criteria:**
- First heartbeat received but not fully operational
- Services still starting up
- Transitional state, should move to Online within 60s

**Visual:** Solid blue indicator dot  
**Note:** This is a temporary state during boot

---

### 🟣 Maintenance
**Color:** Purple  
**Description:** Scheduled maintenance  
**Criteria:**
- Manually set by operator
- Expected to be offline
- No alerts should be generated

**Visual:** Solid purple indicator dot  
**Note:** Set manually via API or dashboard action

---

### ⚪ Unknown
**Color:** Gray  
**Description:** Status unknown  
**Criteria:**
- No data available yet
- Host just added to ledger
- Initial state before first heartbeat

**Visual:** Solid gray indicator dot

---

## Time Thresholds (Configurable)

The health determination uses these default thresholds:

```go
OnlineWindow:      30 seconds   // Max time for "online" status
DegradedWindow:    90 seconds   // Max time before transitioning to "offline"
StartingWindow:    60 seconds   // Max time in "starting" before auto-transition
HeartbeatInterval: 10 seconds   // Expected interval between heartbeats
```

These can be adjusted via `types.DefaultHealthThresholds()` if needed for different network conditions or monitoring requirements.

---

## Dashboard Features

### Real-Time Polling
- HTMX polling every 3 seconds
- Automatic UI updates without page refresh
- Only the host list div is updated (minimal DOM changes)

### Visual Indicators
Each host displays:
1. **Status dot** — Color-coded health indicator (2px circle)
2. **Status text** — Current health status name
3. **Last seen** — Human-readable time since last heartbeat
   - "just now" (< 1 minute)
   - "5m ago" (< 1 hour)
   - "2h ago" (< 24 hours)
   - "3d ago" (24+ hours)

### Tooltip
Hovering over the status indicator shows a detailed description of the health state.

---

## Implementation Details

### Server-Side Calculation
Health status is calculated server-side on every page render using `types.DetermineHealth()`:

```go
func DetermineHealth(lastSeen time.Time, currentStatus string, thresholds HealthThresholds) HealthStatus
```

### Template Helpers
The dashboard uses custom template functions:
- `healthStatus` — Determines health from Host data
- `healthColor` — Returns Tailwind CSS color class
- `healthDesc` — Returns human-readable description
- `timeSince` — Formats duration since last seen

---

## Future Enhancements

Potential additions:
1. **Alert Thresholds** — Generate notifications when hosts transition to Degraded/Offline
2. **Health History** — Store and visualize health state changes over time
3. **Aggregate Stats** — Dashboard summary showing total hosts online/offline/degraded
4. **Custom Thresholds per Host** — Different SLAs for different hardware/network conditions
5. **Flapping Detection** — Detect and alert on hosts rapidly transitioning between states

---

## Configuration

To adjust health check thresholds, modify `internal/types/health.go`:

```go
func DefaultHealthThresholds() HealthThresholds {
    return HealthThresholds{
        OnlineWindow:      30 * time.Second,  // Your custom value
        DegradedWindow:    90 * time.Second,  // Your custom value
        StartingWindow:    60 * time.Second,  // Your custom value
        HeartbeatInterval: 10 * time.Second,  // Your custom value
    }
}
```

Restart the `nsm` service after making changes.

---

## API Response

The `/api/hosts` endpoint returns health data automatically calculated from `LastSeen`:

```json
{
  "75d84206f3e2dbae107bb19b15212...": {
    "hostname": "lobby-display",
    "ip_address": "192.168.1.100",
    "anthias_version": "v2.0.1",
    "anthias_status": "online",
    "dashboard_url": "http://192.168.1.100:8080",
    "last_seen": "2025-10-29T15:30:45Z",
    "public_key": "75d84206f3e2dbae107bb19b15212..."
  }
}
```

Health status can be calculated client-side using the same thresholds.
