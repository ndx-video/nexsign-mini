// Package discovery provides small helpers and a thread-safe PeerStore for
// storing peers discovered via mDNS. The PeerStore exposes simple APIs to
// enumerate peers and produce host:port address strings suitable for
// seeding network components like Tendermint.
package discovery

import (
	"fmt"
	"net"
	"sync"

	"github.com/grandcat/zeroconf"
)

// Peer represents a discovered peer's basic information.
type Peer struct {
	Instance string
	Hostname string
	Port     int
	Addrs    []net.IP
	Txt      map[string]string
}

// PeerStore is a thread-safe store of discovered peers.
type PeerStore struct {
	mtx   sync.RWMutex
	peers map[string]*Peer // keyed by instance
}

// NewPeerStore creates an empty PeerStore.
func NewPeerStore() *PeerStore {
	return &PeerStore{peers: make(map[string]*Peer)}
}

// AddFromServiceEntry adds or updates a peer using a zeroconf ServiceEntry.
func (ps *PeerStore) AddFromServiceEntry(e *zeroconf.ServiceEntry) {
	if e == nil {
		return
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	txt := make(map[string]string)
	for _, t := range e.Text {
		// txt records are typically key=value
		for i := 0; i < len(t); i++ {
			// simple split on '='
		}
		// naive parsing
		if kv := splitOnce(t, '='); kv != nil {
			txt[kv[0]] = kv[1]
		}
	}

	peer := &Peer{
		Instance: e.Instance,
		Hostname: e.HostName,
		Port:     e.Port,
		Addrs:    append([]net.IP(nil), e.AddrIPv4...),
		Txt:      txt,
	}
	ps.peers[e.Instance] = peer
}

// Remove removes a peer by instance name.
func (ps *PeerStore) Remove(instance string) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	delete(ps.peers, instance)
}

// List returns a snapshot of known peers.
func (ps *PeerStore) List() []*Peer {
	ps.mtx.RLock()
	defer ps.mtx.RUnlock()
	out := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		out = append(out, p)
	}
	return out
}

// Addresses returns a list of network addresses in host:port format suitable for seeding
func (ps *PeerStore) Addresses() []string {
	ps.mtx.RLock()
	defer ps.mtx.RUnlock()
	out := make([]string, 0, len(ps.peers))
	for _, p := range ps.peers {
		// prefer IPv4 if available
		if len(p.Addrs) > 0 {
			out = append(out, p.Addrs[0].String()+":"+itoa(p.Port))
		}
	}
	return out
}

// simple itoa avoiding import of strconv in this small file
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// Helper: splitOnce splits s on first sep, returns [key,value] or nil
func splitOnce(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}
