package discovery

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/grandcat/zeroconf"
)

// DiscoveryService handles the mDNS registration and browsing.
type DiscoveryService struct {
	serviceName string
	resolver    *zeroconf.Resolver
	server      *zeroconf.Server
	peers       map[string]*zeroconf.ServiceEntry
	mtx         sync.RWMutex
	cancel      context.CancelFunc
}

// NewDiscoveryService creates a new mDNS discovery service.
func NewDiscoveryService(serviceName string) (*DiscoveryService, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	return &DiscoveryService{
		serviceName: serviceName,
		resolver:    resolver,
		peers:       make(map[string]*zeroconf.ServiceEntry),
	}, nil
}

// Start announces the local service and begins browsing for remote services.
func (s *DiscoveryService) Start(port int) error {
	hostname, _ := os.Hostname()

	server, err := zeroconf.Register(hostname, s.serviceName, "local.", port, []string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		return err
	}
	s.server = server
	log.Printf("mDNS: Announced service %s on the network from host %s", s.serviceName, hostname)

	go s.browseForPeers()

	return nil
}

func (s *DiscoveryService) browseForPeers() {
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			s.mtx.Lock()
			if entry.TTL == 0 {
				log.Printf("mDNS: Peer removed: %s", entry.Instance)
				delete(s.peers, entry.Instance)
			} else {
				log.Printf("mDNS: Peer discovered: %s (%s:%d)", entry.Instance, entry.AddrIPv4[0], entry.Port)
				s.peers[entry.Instance] = entry
			}
			s.mtx.Unlock()
		}
	}(entries)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	log.Println("mDNS: Browsing for other peers...")
	if err := s.resolver.Browse(ctx, s.serviceName, "local.", entries); err != nil {
		log.Printf("ERROR: Failed to browse for mDNS services: %v", err)
	}
	<-ctx.Done()
	log.Println("mDNS: Peer browsing stopped.")
}

// GetPeers returns a thread-safe copy of the discovered peers.
func (s *DiscoveryService) GetPeers() []*zeroconf.ServiceEntry {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	peers := make([]*zeroconf.ServiceEntry, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	return peers
}

// Stop gracefully shuts down the mDNS service.
func (s *DiscoveryService) Stop() {
	log.Println("mDNS: Stopping service discovery...")
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		s.server.Shutdown()
	}
}
