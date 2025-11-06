// Package main is the entry point for nexSign mini (nsm).
// It initializes the host store, Anthias client integration, and web dashboard.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nexsign.mini/nsm/internal/anthias"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/types"
	"nexsign.mini/nsm/internal/web"
)

func main() {
	log.Println("nexSign mini starting...")

	// Initialize host store
	store, err := hosts.NewStore("")
	if err != nil {
		log.Fatalf("Failed to initialize host store: %v", err)
	}
	log.Println("Host store initialized")

	// Initialize Anthias client for local monitoring
	anthiasClient := anthias.NewClient()
	log.Println("Anthias client initialized")

	port := resolvePort(8080)
	if err := ensurePortAvailable(port); err != nil {
		log.Fatalf("Port %d unavailable: %v", port, err)
	}

	// Initialize web server
	server, err := web.NewServer(store, anthiasClient, port)
	if err != nil {
		log.Fatalf("Failed to initialize web server: %v", err)
	}

	// Start web server
	serverErrors := server.Start()
	go func() {
		if err := <-serverErrors; err != nil {
			log.Fatalf("Web server exited: %v", err)
		}
	}()
	log.Printf("Web dashboard available at http://localhost:%d", port)

	// Start background Anthias polling
	go pollAnthias(store, anthiasClient)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

// pollAnthias periodically checks local Anthias status and updates localhost entry
func pollAnthias(store *hosts.Store, client *anthias.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Do initial check immediately
	updateLocalHost(store, client)

	for range ticker.C {
		updateLocalHost(store, client)
	}
}

// updateLocalHost updates the localhost entry with current Anthias data
func updateLocalHost(store *hosts.Store, client *anthias.Client) {
	metadata, err := client.GetMetadata()
	if err != nil {
		log.Printf("Warning: failed to get Anthias metadata: %v", err)
		return
	}

	// Check if localhost exists in store
	allHosts := store.GetAll()
	localhostExists := false

	for _, h := range allHosts {
		if h.IPAddress == "127.0.0.1" || h.IPAddress == metadata.IPAddress {
			localhostExists = true
			break
		}
	}

	if !localhostExists {
		// Add localhost entry
		localhost := types.Host{
			Nickname:       metadata.Nickname,
			IPAddress:      metadata.IPAddress,
			VPNIPAddress:   metadata.VPNIPAddress,
			Hostname:       metadata.Hostname,
			Notes:          "",
			Status:         types.StatusUnreachable,
			NSMStatus:      "NSM Offline",
			NSMVersion:     "unknown",
			CMSStatus:      types.CMSUnknown,
			AnthiasVersion: metadata.AnthiasVersion,
			AnthiasStatus:  metadata.AnthiasStatus,
			DashboardURL:   metadata.DashboardURL,
		}
		if metadata.VPNIPAddress != "" {
			localhost.StatusVPN = types.StatusUnreachable
			localhost.NSMStatusVPN = "NSM Offline"
			localhost.NSMVersionVPN = "unknown"
			localhost.CMSStatusVPN = types.CMSUnknown
			localhost.DashboardURLVPN = fmt.Sprintf("http://%s:8080", metadata.VPNIPAddress)
		}
		if err := store.Add(localhost); err != nil {
			log.Printf("Warning: failed to add localhost: %v", err)
		} else {
			log.Println("Added localhost to host list")
		}
	} else {
		// Update existing localhost entry
		err := store.Update(metadata.IPAddress, func(h *types.Host) {
			if metadata.Hostname != "" {
				h.Hostname = metadata.Hostname
			}
			if h.Nickname == "" && metadata.Nickname != "" {
				h.Nickname = metadata.Nickname
			}
			h.AnthiasVersion = metadata.AnthiasVersion
			h.AnthiasStatus = metadata.AnthiasStatus
			h.DashboardURL = metadata.DashboardURL
			if metadata.VPNIPAddress != "" {
				h.VPNIPAddress = metadata.VPNIPAddress
				if h.DashboardURLVPN == "" {
					h.DashboardURLVPN = fmt.Sprintf("http://%s:8080", metadata.VPNIPAddress)
				}
			}
		})
		if err != nil {
			log.Printf("Warning: failed to update localhost: %v", err)
		}
	}
}

func resolvePort(defaultPort int) int {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		return defaultPort
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		log.Printf("Warning: invalid PORT value %q, using %d", portStr, defaultPort)
		return defaultPort
	}

	return port
}

func ensurePortAvailable(port int) error {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return listener.Close()
}
