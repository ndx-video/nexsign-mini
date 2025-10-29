// Package types - Host health status definitions
package types

import "time"

// HealthStatus represents the operational health of a host
type HealthStatus string

const (
	// HealthOnline - Host is fully operational and responsive
	// - Heartbeat received within last 30 seconds
	// - Anthias service responding to HTTP requests
	// - All critical services running normally
	HealthOnline HealthStatus = "online"

	// HealthDegraded - Host is partially operational with issues
	// - Heartbeat received but delayed (30-90 seconds)
	// - Anthias responding but with errors or warnings
	// - Some non-critical services may be down
	// - Display may be showing content but with issues
	HealthDegraded HealthStatus = "degraded"

	// HealthOffline - Host is not responding
	// - No heartbeat received in 90+ seconds
	// - Anthias HTTP endpoint unreachable
	// - May indicate network issue, power loss, or crash
	HealthOffline HealthStatus = "offline"

	// HealthStarting - Host is booting or initializing
	// - First heartbeat received but not fully operational
	// - Services still starting up
	// - Transitional state, should move to Online within 60s
	HealthStarting HealthStatus = "starting"

	// HealthMaintenance - Host is intentionally offline for maintenance
	// - Manually set by operator
	// - Expected to be offline
	// - No alerts should be generated
	HealthMaintenance HealthStatus = "maintenance"

	// HealthUnknown - Initial state or unable to determine health
	// - No data available yet
	// - Host just added to ledger
	HealthUnknown HealthStatus = "unknown"
)

// HealthThresholds defines time-based health determination
type HealthThresholds struct {
	OnlineWindow      time.Duration // Max time since last seen for "online" (default: 30s)
	DegradedWindow    time.Duration // Max time for "degraded" before "offline" (default: 90s)
	StartingWindow    time.Duration // Max time in "starting" before moving to online/degraded (default: 60s)
	HeartbeatInterval time.Duration // Expected interval between heartbeats (default: 10s)
}

// DefaultHealthThresholds returns sensible defaults for health checking
func DefaultHealthThresholds() HealthThresholds {
	return HealthThresholds{
		OnlineWindow:      30 * time.Second,
		DegradedWindow:    90 * time.Second,
		StartingWindow:    60 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}
}

// DetermineHealth calculates the health status based on last seen time and current state
func DetermineHealth(lastSeen time.Time, currentStatus string, thresholds HealthThresholds) HealthStatus {
	timeSinceLastSeen := time.Since(lastSeen)

	// Manual states take precedence
	if currentStatus == string(HealthMaintenance) {
		return HealthMaintenance
	}

	// Brand new host with zero time
	if lastSeen.IsZero() {
		return HealthUnknown
	}

	// Starting state has a time window
	if currentStatus == string(HealthStarting) {
		if timeSinceLastSeen > thresholds.StartingWindow {
			// Transition out of starting based on time
			if timeSinceLastSeen <= thresholds.OnlineWindow {
				return HealthOnline
			}
			return HealthOffline
		}
		return HealthStarting
	}

	// Time-based health determination
	if timeSinceLastSeen <= thresholds.OnlineWindow {
		return HealthOnline
	} else if timeSinceLastSeen <= thresholds.DegradedWindow {
		return HealthDegraded
	}

	return HealthOffline
}

// HealthColor returns the Tailwind CSS color class for a given health status
func HealthColor(status HealthStatus) string {
	switch status {
	case HealthOnline:
		return "bg-green-500"
	case HealthDegraded:
		return "bg-yellow-500"
	case HealthOffline:
		return "bg-red-500"
	case HealthStarting:
		return "bg-blue-500"
	case HealthMaintenance:
		return "bg-purple-500"
	case HealthUnknown:
		return "bg-gray-500"
	default:
		return "bg-gray-500"
	}
}

// HealthDescription returns a human-readable description of the health status
func HealthDescription(status HealthStatus) string {
	switch status {
	case HealthOnline:
		return "Fully operational"
	case HealthDegraded:
		return "Responding with issues"
	case HealthOffline:
		return "Not responding"
	case HealthStarting:
		return "Initializing"
	case HealthMaintenance:
		return "Scheduled maintenance"
	case HealthUnknown:
		return "Status unknown"
	default:
		return "Unknown"
	}
}
