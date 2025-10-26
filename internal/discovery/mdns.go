package discovery

import (
	"log"
	"os"

	"github.com/grandcat/zeroconf"
)

// DiscoveryService handles the mDNS registration and browsing.
type DiscoveryService struct {
	server      *zeroconf.Server
	serviceName string
}

// NewDiscoveryService creates a new mDNS discovery service.
func NewDiscoveryService(serviceName string) (*DiscoveryService, error) {
	return &DiscoveryService{
		serviceName: serviceName,
	}, nil
}

// Start registers the local nsm instance for discovery.
func (s *DiscoveryService) Start() error {
	hostname, _ := os.Hostname()
	log.Printf("mDNS: Announcing service %s on the network from host %s", s.serviceName, hostname)

	// TODO: Implement the actual mDNS server initialization and registration.
	// We will need to handle discovered peers and add them to our ledger.
	return nil
}

// Stop gracefully shuts down the mDNS service.
func (s *DiscoveryService) Stop() {
	log.Println("mDNS: Stopping service discovery.")
	if s.server != nil {
		s.server.Shutdown()
	}
}
