# internal/anthias

Client for interacting with the local Anthias digital signage service.

## Purpose

This package provides a lightweight HTTP client for polling status and metadata from a locally running Anthias instance. Anthias is the digital signage software that runs on each node in the nexSign network.

## Status

**Note**: This package is currently a placeholder/stub. The client is initialized but not yet actively polling or updating host status.

## Planned Functionality

The Anthias client will be responsible for:

1. **Periodic Status Polling**: Query the local Anthias instance for current status
2. **Version Detection**: Retrieve the Anthias version running on this node
3. **State Updates**: Create and broadcast `TxUpdateStatus` transactions to the network
4. **Dashboard URL**: Provide the dashboard URL for remote monitoring

## Usage

### Current Usage

```go
import "nexsign.mini/nsm/internal/anthias"

// Initialize the client (currently a no-op)
client := anthias.NewClient()
```

### Planned Usage (Not Yet Implemented)

```go
// Poll status every 30 seconds
client := anthias.NewClient("http://localhost:80")
client.StartPolling(30 * time.Second, func(status anthias.Status) {
    // Create and broadcast TxUpdateStatus transaction
})
```

## Expected Anthias API

The client will interact with these Anthias endpoints (when implemented):

### GET /api/v1/status

Returns the current status of the Anthias instance:

```json
{
  "status": "online",
  "version": "0.18.2",
  "uptime": 3600,
  "current_asset": "welcome.png"
}
```

### GET /api/v1/info

Returns system information:

```json
{
  "hostname": "signage-node-1",
  "ip_address": "192.168.1.10",
  "dashboard_url": "http://192.168.1.10:80"
}
```

## Integration with ABCI

Once implemented, the Anthias client will:

1. Poll local Anthias for status changes
2. Sign status updates with the node's identity
3. Broadcast `TxUpdateStatus` transactions to the ABCI app
4. Update the distributed ledger state

## Configuration

Future configuration options (not yet implemented):

```json
{
  "anthias_url": "http://localhost:80",
  "poll_interval_seconds": 30,
  "enable_status_updates": true
}
```

## Development Notes

- The client should handle Anthias downtime gracefully (log errors, retry)
- Status updates should be rate-limited to avoid spamming the network
- Consider batching multiple status changes into a single transaction
- Add health checks to detect when Anthias is unavailable

## Future Enhancements

- Implement actual HTTP polling
- Add retry logic with exponential backoff
- Support custom Anthias API endpoints
- Add metrics/monitoring for Anthias connectivity
- Implement graceful shutdown/cleanup
