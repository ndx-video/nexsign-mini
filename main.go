// Package main is the entry point for nexSign mini (nsm).
// It initializes the host store, Anthias client integration, and web dashboard.
package main

import (
	"log"
	"os"
	"os/signal"
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
	store, err := hosts.NewStore("hosts.json")
	if err != nil {
		log.Fatalf("Failed to initialize host store: %v", err)
	}
	log.Println("Host store initialized")

	// Initialize Anthias client for local monitoring
	anthiasClient := anthias.NewClient()
	log.Println("Anthias client initialized")

	// Get port from environment or use default
	port := 8080
	if portStr := os.Getenv("PORT"); portStr != "" {
		_, err := os.Stat(portStr)
		if err == nil {
			port = 8080 // keep default if parsing fails
		}
	}

	// Initialize web server
	server, err := web.NewServer(store, anthiasClient, port)
	if err != nil {
		log.Fatalf("Failed to initialize web server: %v", err)
	}

	// Start web server
	server.Start()
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
			IPAddress:      metadata.IPAddress,
			Hostname:       metadata.Hostname,
			Status:         types.StatusHealthy,
			AnthiasVersion: metadata.AnthiasVersion,
			AnthiasStatus:  metadata.AnthiasStatus,
			DashboardURL:   metadata.DashboardURL,
			LastChecked:    time.Now(),
		}
		if err := store.Add(localhost); err != nil {
			log.Printf("Warning: failed to add localhost: %v", err)
		} else {
			log.Println("Added localhost to host list")
		}
	} else {
		// Update existing localhost entry
		err := store.Update(metadata.IPAddress, func(h *types.Host) {
			h.Hostname = metadata.Hostname
			h.AnthiasVersion = metadata.AnthiasVersion
			h.AnthiasStatus = metadata.AnthiasStatus
			h.DashboardURL = metadata.DashboardURL
			h.Status = types.StatusHealthy
			h.LastChecked = time.Now()
		})
		if err != nil {
			log.Printf("Warning: failed to update localhost: %v", err)
		}
	}
}
