// Package discovery implements local network discovery using mDNS (zeroconf).
// It announces the local _nsm._tcp service and browses for other nsm
// instances on the LAN. Discovered peers are stored in a thread-safe PeerStore
// and can be used to seed Tendermint or provide peer information to the UI.
package discovery

import (
	"context"
	"log"
	"os"

	"github.com/grandcat/zeroconf"
)

// DiscoveryService handles the mDNS registration and browsing.
type DiscoveryService struct {
	serviceName string
	resolver    *zeroconf.Resolver
	server      *zeroconf.Server
	peerStore   *PeerStore
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
		peerStore:   NewPeerStore(),
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
			if entry.TTL == 0 {
				log.Printf("mDNS: Peer removed: %s", entry.Instance)
				s.peerStore.Remove(entry.Instance)
			} else {
				if len(entry.AddrIPv4) > 0 {
					log.Printf("mDNS: Peer discovered: %s (%s:%d)", entry.Instance, entry.AddrIPv4[0], entry.Port)
				} else {
					log.Printf("mDNS: Peer discovered: %s (no addr yet:%d)", entry.Instance, entry.Port)
				}
				s.peerStore.AddFromServiceEntry(entry)
			}

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

// GetPeers returns a snapshot of discovered peers as Peer entries.
func (s *DiscoveryService) GetPeers() []*Peer {
	if s.peerStore == nil {
		return nil
	}
	return s.peerStore.List()
}

// GetPeerAddresses returns host:port strings usable for seeding Tendermint
func (s *DiscoveryService) GetPeerAddresses() []string {
	if s.peerStore == nil {
		return nil
	}
	return s.peerStore.Addresses()
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
