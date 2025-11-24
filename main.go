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

	// We are running, so we are online
	metadata.Status = types.StatusHealthy
	metadata.NSMStatus = "NSM Online"
	metadata.NSMVersion = types.Version
	metadata.LastChecked = time.Now()

	// Check if we already have an entry for this ID
	existing, err := store.GetByID(metadata.ID)
	if err == nil {
		// Update existing entry
		// Preserve user-editable fields
		if existing.Nickname != "" {
			metadata.Nickname = existing.Nickname
		}
		if existing.Notes != "" {
			metadata.Notes = existing.Notes
		}
		
		// Respect existing IP if different (user manual override)
		if existing.IPAddress != metadata.IPAddress {
			metadata.IPAddress = existing.IPAddress
			metadata.DashboardURL = existing.DashboardURL
		}

		if err := store.Upsert(*metadata); err != nil {
			log.Printf("Warning: failed to update localhost: %v", err)
		}
	} else {
		// New ID. Check for hostname conflict.
		allHosts := store.GetAll()
		for _, h := range allHosts {
			if h.Hostname == metadata.Hostname && h.Hostname != "" && h.Hostname != "localhost" && h.Hostname != "unknown" {
				log.Printf("Localhost %s (ID: %s) matches existing host %s (ID: %s). Skipping self-registration.", metadata.Hostname, metadata.ID, h.Hostname, h.ID)
				return
			}
		}

		// No conflict, add new
		if err := store.Upsert(*metadata); err != nil {
			log.Printf("Warning: failed to add localhost: %v", err)
		} else {
			log.Println("Added localhost to host list")
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
