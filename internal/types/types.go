// Package types defines the core domain models for nexSign mini (nsm).
// It contains the Host data model and status constants used across the
// application. Hosts are manually managed and synchronized via HTTP POST.
package types

import (
	"time"
)

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
	IPAddress      string           `json:"ip_address"`      // Required: IP address of the host
	Hostname       string           `json:"hostname"`        // Optional: friendly name for the host
	Status         HostStatus       `json:"status"`          // Health status: unreachable, connection_refused, unhealthy, healthy
	AnthiasVersion string           `json:"anthias_version"` // Detected Anthias version
	AnthiasStatus  string           `json:"anthias_status"`  // Anthias service status (online, offline, unknown)
	CMSStatus      AnthiasCMSStatus `json:"cms_status"`      // Anthias CMS status (CMS Online, CMS Offline, CMS Unknown)
	DashboardURL   string           `json:"dashboard_url"`   // URL to host's dashboard (port 80)
	LastChecked    time.Time        `json:"last_checked"`    // Last time status was checked
}
