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
// It also checks the Anthias CMS status by querying the /api/v1/assets endpoint
func CheckHealth(host *types.Host) types.HostStatus {
	// Try to connect to the nsm dashboard port (8080)
	timeout := 3 * time.Second
	nsmAddress := fmt.Sprintf("%s:8080", host.IPAddress)

	// First, try TCP connection to nsm port
	conn, err := net.DialTimeout("tcp", nsmAddress, timeout)
	if err != nil {
		// Check if it's a connection refused vs unreachable
		if opErr, ok := err.(*net.OpError); ok {
			if _, ok := opErr.Err.(*net.DNSError); ok {
				host.CMSStatus = types.CMSUnknown
				return types.StatusUnreachable
			}
			// Connection refused typically means host is up but service isn't running
			host.CMSStatus = types.CMSUnknown
			return types.StatusConnectionRefused
		}
		host.CMSStatus = types.CMSUnknown
		return types.StatusUnreachable
	}
	conn.Close()

	// NSM is reachable, now check Anthias CMS on port 80
	checkAnthiasCMS(host)

	// HTTP health check for nsm
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

// checkAnthiasCMS checks if the Anthias CMS is online by querying /api/v1/assets on port 80
func checkAnthiasCMS(host *types.Host) {
	timeout := 3 * time.Second
	client := &http.Client{Timeout: timeout}

	// Try to query Anthias API
	anthiasURL := fmt.Sprintf("http://%s/api/v1/assets", host.IPAddress)

	resp, err := client.Get(anthiasURL)
	if err != nil {
		// Cannot reach Anthias API
		host.CMSStatus = types.CMSOffline
		return
	}
	defer resp.Body.Close()

	// If we get a 200 response, CMS is online
	if resp.StatusCode == http.StatusOK {
		host.CMSStatus = types.CMSOnline
		return
	}

	// Any other status code means offline or error
	host.CMSStatus = types.CMSOffline
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
