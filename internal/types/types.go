// Package types defines the core domain models for nexSign mini (nsm).
// It contains the Host data model and status constants used across the
// application. Hosts are manually managed and synchronized via HTTP POST.
package types

import (
	"time"
)

// Version is the current version of NSM
const Version = "0.2.0"

// BuildTime is set at build time via -ldflags
var BuildTime = "dev"

// HostStatus represents the current health/reachability status of a host
type HostStatus string

const (
	StatusUnreachable       HostStatus = "unreachable"
	StatusConnectionRefused HostStatus = "connection_refused"
	StatusUnhealthy         HostStatus = "unhealthy"
	StatusHealthy           HostStatus = "healthy"
	StatusStale             HostStatus = "stale" // NSM binary needs updating (apt update && apt upgrade)
)

// AnthiasCMSStatus represents the status of the Anthias CMS
type AnthiasCMSStatus string

const (
	CMSOnline  AnthiasCMSStatus = "CMS Online"
	CMSOffline AnthiasCMSStatus = "CMS Offline"
	CMSUnknown AnthiasCMSStatus = "CMS Unknown"
)

// Host represents a single Anthias digital signage host on the network.
// Hosts are identified by IP address and managed manually via the dashboard.
type Host struct {
	ID                string           `json:"id"`                            // Unique identifier for the host (UUID)
	Nickname          string           `json:"nickname"`                      // Optional: user-friendly label displayed in UI
	IPAddress         string           `json:"ip_address"`                    // Required: LAN IP address of the host
	VPNIPAddress      string           `json:"vpn_ip_address,omitempty"`      // Optional: Tailnet/Tailscale IP address
	Hostname          string           `json:"hostname"`                      // Detected UNIX hostname from remote node
	Notes             string           `json:"notes,omitempty"`               // Optional operator notes surfaced in UI
	Status            HostStatus       `json:"status"`                        // LAN health status: unreachable, connection_refused, unhealthy, healthy, stale
	StatusVPN         HostStatus       `json:"status_vpn,omitempty"`          // VPN health status when VPN IP is configured
	NSMStatus         string           `json:"nsm_status"`                    // Textual representation of LAN NSM dashboard state
	NSMStatusVPN      string           `json:"nsm_status_vpn,omitempty"`      // Textual representation of VPN NSM dashboard state
	NSMVersion        string           `json:"nsm_version"`                   // Detected LAN NSM version
	NSMVersionVPN     string           `json:"nsm_version_vpn,omitempty"`     // Detected VPN NSM version
	AnthiasVersion    string           `json:"anthias_version"`               // Detected LAN Anthias version
	AnthiasVersionVPN string           `json:"anthias_version_vpn,omitempty"` // Detected VPN Anthias version
	AnthiasStatus     string           `json:"anthias_status"`                // Anthias service status (LAN)
	AnthiasStatusVPN  string           `json:"anthias_status_vpn,omitempty"`  // Anthias service status (VPN)
	CMSStatus         AnthiasCMSStatus `json:"cms_status"`                    // Anthias CMS status over LAN
	CMSStatusVPN      AnthiasCMSStatus `json:"cms_status_vpn,omitempty"`      // Anthias CMS status over VPN
	AssetCount        int              `json:"asset_count"`                   // Number of assets reachable via LAN
	AssetCountVPN     int              `json:"asset_count_vpn,omitempty"`     // Number of assets reachable via VPN
	DashboardURL      string           `json:"dashboard_url"`                 // URL to host's NSM dashboard over LAN
	DashboardURLVPN   string           `json:"dashboard_url_vpn,omitempty"`   // URL to host's NSM dashboard over VPN
	LastChecked       time.Time        `json:"last_checked"`                  // Last time LAN status was checked
	LastCheckedVPN    time.Time        `json:"last_checked_vpn,omitempty"`    // Last time VPN status was checked
}
