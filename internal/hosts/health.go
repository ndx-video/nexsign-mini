// Package hosts provides health checking functionality for remote hosts
package hosts

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"nexsign.mini/nsm/internal/types"
)

// CheckHealth performs a health check on a host and returns its status
func CheckHealth(host *types.Host) types.HostStatus {
	// Try to connect to the host's dashboard port
	timeout := 3 * time.Second
	address := fmt.Sprintf("%s:8080", host.IPAddress)

	// First, try TCP connection
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		// Check if it's a connection refused vs unreachable
		if opErr, ok := err.(*net.OpError); ok {
			if _, ok := opErr.Err.(*net.DNSError); ok {
				return types.StatusUnreachable
			}
			// Connection refused typically means host is up but service isn't running
			return types.StatusConnectionRefused
		}
		return types.StatusUnreachable
	}
	conn.Close()

	// TCP connection succeeded, try HTTP health check
	client := &http.Client{Timeout: timeout}
	healthURL := fmt.Sprintf("http://%s:8080/api/health", host.IPAddress)

	resp, err := client.Get(healthURL)
	if err != nil {
		// HTTP failed but TCP worked - service might be unhealthy
		return types.StatusUnhealthy
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode == http.StatusOK {
		return types.StatusHealthy
	}

	return types.StatusUnhealthy
}

// CheckAllHosts checks health of all hosts and updates their status
func (s *Store) CheckAllHosts() {
	hosts := s.GetAll()

	for i := range hosts {
		hosts[i].Status = CheckHealth(&hosts[i])
		hosts[i].LastChecked = time.Now()
	}

	s.ReplaceAll(hosts)
}
