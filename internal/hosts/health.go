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
	host.Status = checkNetwork(host, host.IPAddress, false)

	if host.VPNIPAddress != "" {
		host.StatusVPN = checkNetwork(host, host.VPNIPAddress, true)
	} else {
		host.StatusVPN = ""
		host.NSMStatusVPN = ""
		host.NSMVersionVPN = ""
		host.CMSStatusVPN = types.CMSUnknown
		host.AssetCountVPN = 0
		host.DashboardURLVPN = ""
		host.LastCheckedVPN = time.Time{}
	}

	return host.Status
}

func checkNetwork(host *types.Host, ip string, isVPN bool) types.HostStatus {
	now := time.Now()

	dashboardURL := ""
	if ip != "" {
		dashboardURL = fmt.Sprintf("http://%s:8080", ip)
	}

	cmsStatus, assetCount := checkAnthiasCMSByIP(ip)

	status := types.StatusUnreachable
	nsmStatusText := "NSM Offline"
	nsmVersion := "unknown"

	if ip == "" {
		applyNetworkResults(host, isVPN, status, cmsStatus, assetCount, nsmStatusText, nsmVersion, dashboardURL, now)
		return status
	}

	timeout := 3 * time.Second
	nsmAddress := fmt.Sprintf("%s:8080", ip)

	conn, err := net.DialTimeout("tcp", nsmAddress, timeout)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if _, ok := opErr.Err.(*net.DNSError); ok {
				status = types.StatusUnreachable
			} else {
				status = types.StatusConnectionRefused
				nsmStatusText = "NSM Connection Refused"
			}
		} else {
			status = types.StatusUnreachable
		}
		applyNetworkResults(host, isVPN, status, cmsStatus, assetCount, nsmStatusText, nsmVersion, dashboardURL, now)
		return status
	}
	conn.Close()

	status = types.StatusUnhealthy

	client := &http.Client{Timeout: timeout}
	versionURL := fmt.Sprintf("http://%s:8080/api/version", ip)

	versionResp, err := client.Get(versionURL)
	if err == nil {
		defer versionResp.Body.Close()
		if versionResp.StatusCode == http.StatusOK {
			var versionData struct {
				Version  string `json:"version"`
				Hostname string `json:"hostname"`
			}
			if err := json.NewDecoder(versionResp.Body).Decode(&versionData); err == nil {
				if versionData.Version != "" {
					nsmVersion = versionData.Version
					if compareVersions(versionData.Version, types.Version) < 0 {
						status = types.StatusStale
						nsmStatusText = "NSM Online (Update Required)"
					}
				}
				if versionData.Hostname != "" {
					host.Hostname = versionData.Hostname
				}
			}
		}
	}

	if nsmStatusText == "NSM Offline" {
		nsmStatusText = "NSM Unhealthy"
	}

	healthURL := fmt.Sprintf("http://%s:8080/api/health", ip)
	resp, err := client.Get(healthURL)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			if status != types.StatusStale {
				status = types.StatusHealthy
				nsmStatusText = "NSM Online"
			}
		}
	}

	if status == types.StatusUnhealthy && nsmStatusText == "NSM Unhealthy" {
		nsmStatusText = "NSM Degraded"
	}

	applyNetworkResults(host, isVPN, status, cmsStatus, assetCount, nsmStatusText, nsmVersion, dashboardURL, now)

	return status
}

func applyNetworkResults(host *types.Host, isVPN bool, status types.HostStatus, cmsStatus types.AnthiasCMSStatus, assetCount int, nsmStatus string, nsmVersion string, dashboardURL string, checkedAt time.Time) {
	if isVPN {
		host.StatusVPN = status
		host.CMSStatusVPN = cmsStatus
		host.AssetCountVPN = assetCount
		host.NSMStatusVPN = nsmStatus
		host.NSMVersionVPN = nsmVersion
		host.DashboardURLVPN = dashboardURL
		host.LastCheckedVPN = checkedAt
	} else {
		host.Status = status
		host.CMSStatus = cmsStatus
		host.AssetCount = assetCount
		host.NSMStatus = nsmStatus
		host.NSMVersion = nsmVersion
		host.DashboardURL = dashboardURL
		host.LastChecked = checkedAt
	}
}

// checkAnthiasCMSByIP checks CMS availability for a specific IP address.
func checkAnthiasCMSByIP(ip string) (types.AnthiasCMSStatus, int) {
	if ip == "" {
		return types.CMSUnknown, 0
	}

	timeout := 3 * time.Second
	client := &http.Client{Timeout: timeout}
	anthiasURL := fmt.Sprintf("http://%s/api/v1/assets?format=json", ip)

	resp, err := client.Get(anthiasURL)
	if err != nil {
		return types.CMSOffline, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var assets []map[string]interface{}
		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(&assets); err != nil {
			return types.CMSOnline, 0
		}

		return types.CMSOnline, len(assets)
	}

	return types.CMSOffline, 0
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
		CheckHealth(&hosts[i])
	}

	s.ReplaceAll(hosts)
}
