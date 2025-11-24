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

	"github.com/google/uuid"
	"nexsign.mini/nsm/internal/types"
)

// Client is responsible for communicating with the local Anthias instance.
type Client struct {
	// anthiasURL is the local Anthias HTTP API endpoint
	anthiasURL string
	// id is the unique identifier for this node
	id string
}

// NewClient creates a new Anthias client.
func NewClient() *Client {
	// Load or generate persistent ID
	idFile := "identity.id"
	var id string
	if data, err := os.ReadFile(idFile); err == nil {
		id = strings.TrimSpace(string(data))
	}

	if id == "" {
		id = uuid.New().String()
		if err := os.WriteFile(idFile, []byte(id), 0o644); err != nil {
			fmt.Printf("Warning: failed to save identity file: %v\n", err)
		}
	}

	// TODO: Allow configuration of Anthias URL via env var or config
	return &Client{
		anthiasURL: "http://localhost:8080", // Default Anthias port
		id:         id,
	}
}

// GetMetadata fetches the metadata from the local system and Anthias instance.
// This includes hostname, IP address, Anthias version/status, etc.
func (c *Client) GetMetadata() (*types.Host, error) {
	host := &types.Host{}
	host.ID = c.id

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	host.Nickname = hostname
	host.Hostname = hostname

	// Get primary IP address (first non-loopback IPv4)
	host.IPAddress = getPrimaryIP()
	host.DashboardURL = fmt.Sprintf("http://%s:8080", host.IPAddress)
	host.Status = types.StatusUnreachable
	host.NSMStatus = "NSM Offline"
	host.NSMVersion = "unknown"
	host.CMSStatus = types.CMSUnknown
	host.VPNIPAddress = ""
	host.CMSStatusVPN = types.CMSUnknown
	host.NSMStatusVPN = ""
	host.NSMVersionVPN = ""
	host.DashboardURLVPN = ""

	// Try to get Anthias version and status
	// For now, we'll use system checks since Anthias API may not be running
	host.AnthiasVersion = getAnthiasVersion()
	host.AnthiasStatus = getAnthiasStatus()

	return host, nil
}

// getPrimaryIP returns the first non-loopback IPv4 address
func getPrimaryIP() string {
	if ip := os.Getenv("NSM_HOST_IP"); ip != "" {
		return ip
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// Ignore WSL/Hyper-V virtual switch IP
				if ipnet.IP.String() == "10.255.255.254" {
					continue
				}
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
