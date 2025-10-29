// Package anthias contains a small client used to query the local Anthias
// digital-signage service for metadata and status. The Anthias client is used
// by the main event loop to collect local host information which may then be
// turned into signed StateTransactions and broadcast to the network.
package anthias

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"nexsign.mini/nsm/internal/types"
)

// Client is responsible for communicating with the local Anthias instance.
type Client struct {
	// anthiasURL is the local Anthias HTTP API endpoint
	anthiasURL string
}

// NewClient creates a new Anthias client.
func NewClient() *Client {
	// TODO: Allow configuration of Anthias URL via env var or config
	return &Client{
		anthiasURL: "http://localhost:8080", // Default Anthias port
	}
}

// GetMetadata fetches the metadata from the local system and Anthias instance.
// This includes hostname, IP address, Anthias version/status, etc.
func (c *Client) GetMetadata() (*types.Host, error) {
	host := &types.Host{}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	host.Hostname = hostname

	// Get primary IP address (first non-loopback IPv4)
	host.IPAddress = getPrimaryIP()

	// Try to get Anthias version and status
	// For now, we'll use system checks since Anthias API may not be running
	host.AnthiasVersion = getAnthiasVersion()
	host.AnthiasStatus = getAnthiasStatus()
	
	// Set dashboard URL
	if host.IPAddress != "" && host.IPAddress != "127.0.0.1" {
		host.DashboardURL = fmt.Sprintf("http://%s:8080", host.IPAddress)
	} else {
		host.DashboardURL = "http://localhost:8080"
	}

	return host, nil
}

// getPrimaryIP returns the first non-loopback IPv4 address
func getPrimaryIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// getAnthiasVersion attempts to detect the Anthias version
func getAnthiasVersion() string {
	// TODO: Query actual Anthias API when available
	// For now, check if anthias is installed
	if _, err := exec.LookPath("anthias"); err == nil {
		return "detected"
	}
	return "unknown"
}

// getAnthiasStatus checks if Anthias services are running
func getAnthiasStatus() string {
	// TODO: Query actual Anthias API health endpoint when available
	// For now, we'll check if we can connect to the expected port
	
	// Try to check systemd service status
	cmd := exec.Command("systemctl", "is-active", "anthias")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "active" {
		return "online"
	}

	// Fallback: check if something is listening on port 8080
	conn, err := net.DialTimeout("tcp", "localhost:8080", 1000000000) // 1 second
	if err == nil {
		conn.Close()
		return "online"
	}

	return "unknown"
}
