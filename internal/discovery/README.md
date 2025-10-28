# internal/discovery

Local network discovery using mDNS (multicast DNS) for automatic peer detection.

## Purpose

This package enables nexSign mini nodes to automatically discover each other on the local network without requiring manual configuration. It uses the Zeroconf protocol (mDNS/DNS-SD) to announce and browse for other `nsm` instances.

## Key Components

### DiscoveryService

Manages mDNS announcement and browsing:

```go
type DiscoveryService struct {
    serviceName string           // mDNS service type (e.g., "_nsm._tcp")
    resolver    *zeroconf.Resolver
    server      *zeroconf.Server
    peerStore   *PeerStore       // Thread-safe storage of discovered peers
    cancel      context.CancelFunc
}
```

### PeerStore

Thread-safe store of discovered peers:

```go
type PeerStore struct {
    peers map[string]*Peer  // Keyed by instance name
}

type Peer struct {
    Instance string
    Hostname string
    Port     int
    Addrs    []net.IP
    Txt      map[string]string
}
```

## Usage

### Starting Discovery

```go
import "nexsign.mini/nsm/internal/discovery"

// Create service with mDNS service name
service, err := discovery.NewDiscoveryService("_nsm._tcp")
if err != nil {
    log.Fatal(err)
}

// Start announcing and browsing (port is the HTTP port to advertise)
if err := service.Start(8080); err != nil {
    log.Fatal(err)
}

// Later, get discovered peers
peers := service.GetPeers()
for _, peer := range peers {
    fmt.Printf("Discovered: %s at %s:%d\n", peer.Instance, peer.Addrs[0], peer.Port)
}

// Get host:port strings for Tendermint seeding
addresses := service.GetPeerAddresses()
// ["192.168.1.10:8080", "192.168.1.20:8080"]
```

### Stopping Discovery

```go
service.Stop()
```

## How It Works

### Announcement (Registration)

When `Start()` is called:

1. The service registers itself with mDNS using the local hostname
2. Announces the `_nsm._tcp` service type on the `.local.` domain
3. Advertises the HTTP port for other services to connect to
4. Includes TXT records for metadata (currently minimal)

### Browsing (Discovery)

A background goroutine continuously browses for other `_nsm._tcp` services:

1. Listens for mDNS service announcements
2. When a peer is discovered, adds it to the PeerStore
3. When a peer's TTL expires (peer goes offline), removes it from the PeerStore
4. Logs all discovery events

### Peer Lifecycle

- **Discovered**: Peer announces itself → added to PeerStore
- **Active**: Peer periodically refreshes its announcement
- **Removed**: Peer stops announcing or TTL=0 → removed from PeerStore

## Integration with Tendermint

The discovery service provides peer addresses in a format suitable for Tendermint:

```go
addresses := service.GetPeerAddresses()
// Write to file for Tendermint config
writePeersFile(addresses, "tendermint_persistent_peers")
```

Tendermint can then read this file and seed its peer list. See `cmd/nsm/main.go` for the implementation.

## Network Requirements

### Multicast Support

mDNS requires multicast UDP support on the network:

- **Works**: Local networks, home networks, most corporate LANs
- **May Not Work**: Some cloud environments, VPNs, networks with multicast disabled

### Firewall Rules

Ensure these ports are open on the local firewall:

- **UDP 5353**: mDNS protocol
- **TCP <http_port>**: HTTP service (e.g., 8080)

## Configuration

The service name is configurable via the main config:

```json
{
  "mdns_service_name": "_nsm._tcp"
}
```

Or via environment variable:

```bash
export MDNS_SERVICE_NAME="_custom._tcp"
```

## Concurrency Safety

The PeerStore is thread-safe and can be accessed from multiple goroutines:

- Uses `sync.RWMutex` for safe concurrent access
- All methods (Add, Remove, List, Addresses) are protected

## Testing

### Unit Tests

The package includes unit tests for PeerStore concurrency:

```bash
go test ./internal/discovery/...
```

### Manual Testing

To test discovery across nodes:

1. Start node 1:
```bash
PORT=8080 go run cmd/nsm/main.go
```

2. Start node 2 on a different port:
```bash
PORT=8081 go run cmd/nsm/main.go
```

3. Check logs for discovery messages:
```
mDNS: Peer discovered: hostname-1 (192.168.1.10:8080)
```

### Testing Without mDNS

If mDNS is not available in your environment, you can manually populate the peer list by editing the Tendermint peers file.

## Limitations and Future Enhancements

### Current Limitations

1. **Local Network Only**: mDNS only works on the local broadcast domain
2. **No WAN Discovery**: Cannot discover peers across the internet
3. **No Peer Verification**: All discovered peers are trusted
4. **No Persistence**: Peer list is rebuilt on restart

### Planned Enhancements

1. **WAN Discovery**: Add support for a bootstrap node or DHT
2. **Peer Verification**: Validate peer identities using public keys
3. **Peer Scoring**: Track peer reliability and latency
4. **Persistent Peers**: Save and reload known peers across restarts
5. **Service Metadata**: Include more details in TXT records (version, capabilities)

## Troubleshooting

### No peers discovered

1. Check that nodes are on the same local network
2. Verify firewall allows UDP 5353
3. Ensure multicast is enabled on the network interface
4. Check logs for mDNS errors

### Peers discovered but can't connect

1. Verify the advertised port is accessible
2. Check firewall rules for the HTTP port
3. Ensure the service is actually listening on the advertised port

### Duplicate entries in PeerStore

This shouldn't happen, but if it does:
- Peers are keyed by instance name (hostname)
- Multiple entries indicate hostname conflicts or a bug

## Dependencies

- `github.com/grandcat/zeroconf`: mDNS/DNS-SD implementation
- Standard library: `net`, `sync`, `context`

## Example Output

```
INFO: mDNS: Announced service _nsm._tcp on the network from host node1
INFO: mDNS: Browsing for other peers...
INFO: mDNS: Peer discovered: node2 (192.168.1.20:8080)
INFO: mDNS: Peer discovered: node3 (192.168.1.30:8080)
INFO: Wrote 2 peer addresses to tendermint_persistent_peers
```
