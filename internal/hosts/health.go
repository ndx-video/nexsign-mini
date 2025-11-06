// Package hosts provides health checking functionality for remote hosts
package hosts

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
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

	// Check NSM version
	client := &http.Client{Timeout: timeout}
	versionURL := fmt.Sprintf("http://%s:8080/api/version", host.IPAddress)

	versionResp, err := client.Get(versionURL)
	if err != nil {
		// Cannot get version - service might not have /api/version endpoint
		host.NSMVersion = "unknown"
		return types.StatusUnhealthy
	}
	defer versionResp.Body.Close()

	if versionResp.StatusCode == http.StatusOK {
		var versionData struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(versionResp.Body).Decode(&versionData); err == nil {
			host.NSMVersion = versionData.Version

			// Compare versions - if remote is older than current, mark as stale
			if compareVersions(versionData.Version, types.Version) < 0 {
				return types.StatusStale
			}
		} else {
			host.NSMVersion = "unknown"
			return types.StatusUnhealthy
		}
	} else {
		host.NSMVersion = "unknown"
		return types.StatusUnhealthy
	}

	// HTTP health check for nsm
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

// compareVersions compares two semantic version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Simple version comparison for semantic versioning (e.g., "0.1.0")
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 < p2 {
			return -1
		} else if p1 > p2 {
			return 1
		}
	}

	return 0
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
