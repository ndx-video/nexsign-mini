package discovery

import (
	"log"
	"os"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceName is the unique identifier for the nsm service via mDNS.
	ServiceName = "_nsm._tcp"
)

// DiscoveryService handles the mDNS registration and browsing.
type DiscoveryService struct {
	server *zeroconf.Server
}

// NewDiscoveryService creates a new mDNS discovery service.
func NewDiscoveryService() (*DiscoveryService, error) {
	// For now, we just instantiate the struct.
	// The actual server will be started in the Start() method.
	return &DiscoveryService{}, nil
}

// Start registers the local nsm instance for discovery.
func (s *DiscoveryService) Start() {
	hostname, _ := os.Hostname()
	log.Printf("mDNS: Announcing service %s on the network from host %s", ServiceName, hostname)

	// TODO: Implement the actual mDNS server initialization and registration.
	// We will need to handle discovered peers and add them to our ledger.
}

// Stop gracefully shuts down the mDNS service.
func (s *DiscoveryService) Stop() {
	log.Println("mDNS: Stopping service discovery.")
	if s.server != nil {
		s.server.Shutdown()
	}
}