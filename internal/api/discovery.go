package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"nexsign.mini/nsm/internal/discovery"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/types"
)

// @Title: Scan Network
// @Route: POST /api/discovery/scan
// @Description: Scan local network for other NSM instances
// @Response: 204 No Content
func (s *Service) HandleDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for interface override from query param
	overrideIP := r.URL.Query().Get("interface_ip")
	if overrideIP == "" {
		overrideIP = os.Getenv("NSM_HOST_IP")
	}

	// We need the port. Assuming 8080 for now as it's standard, 
	// or we could pass it in Service struct if variable.
	port := 8080 

	go func() {
		s.logger.Info("API: Starting network discovery scan...")
		scanner := discovery.NewScanner(port, overrideIP, s.logger)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		results, err := scanner.Scan(ctx)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Discovery scan failed: %v", err))
			return
		}

		count := 0
		var wg sync.WaitGroup

		for host := range results {
			// Try to get remote details
			var remoteHost types.Host
			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s:%d/api/host/local", host.IP, host.Port))
			
			var hostToSave types.Host
			var isNew bool

			if err == nil {
				if json.NewDecoder(resp.Body).Decode(&remoteHost) == nil {
					// We have full details!
					remoteHost.IPAddress = host.IP
					remoteHost.DashboardURL = fmt.Sprintf("http://%s:%d", host.IP, host.Port)
					
					// Reset status fields to ensure local health check is authoritative
					remoteHost.Status = types.StatusUnreachable
					remoteHost.CMSStatus = types.CMSUnknown
					remoteHost.NSMStatus = "NSM Offline"
					remoteHost.AssetCount = 0
					
					hostToSave = remoteHost
					isNew = true 
				}
				resp.Body.Close()
			}

			// If we didn't get full details, try fallback or create new
			if hostToSave.IPAddress == "" {
				// Try to get remote ID (fallback)
				var remoteID string
				resp, err = client.Get(fmt.Sprintf("http://%s:%d/api/version", host.IP, host.Port))
				if err == nil {
					var v struct {
						ID string `json:"id"`
					}
					if json.NewDecoder(resp.Body).Decode(&v) == nil {
						remoteID = v.ID
					}
					resp.Body.Close()
				}

				// Check if we already have this host
				if remoteID != "" {
					if existing, err := s.store.GetByID(remoteID); err == nil {
						hostToSave = *existing
						if hostToSave.IPAddress != host.IP {
							hostToSave.IPAddress = host.IP
							hostToSave.DashboardURL = fmt.Sprintf("http://%s:%d", host.IP, host.Port)
						}
					}
				}
				
				if hostToSave.IPAddress == "" {
					// Check by IP
					if existing, err := s.store.GetByIP(host.IP); err == nil {
						hostToSave = *existing
					} else {
						// Create new
						hostToSave = types.Host{
							ID:            remoteID,
							Nickname:      "Discovered Host",
							IPAddress:     host.IP,
							Status:        types.StatusUnreachable,
							NSMStatus:     "NSM Offline",
							NSMVersion:    "unknown",
							CMSStatus:     types.CMSUnknown,
							DashboardURL:  fmt.Sprintf("http://%s:%d", host.IP, host.Port),
							LastChecked:   time.Time{},
						}
						isNew = true
					}
				}
			}

			// Ensure ID exists
			if hostToSave.ID == "" {
				hostToSave.ID = uuid.New().String()
			}

			// Handle stale entries (same IP, different ID)
			if oldHost, err := s.store.GetByIP(host.IP); err == nil && oldHost.ID != hostToSave.ID {
				s.logger.Warning(fmt.Sprintf("Replacing stale host %s (ID: %s) with discovered ID %s", oldHost.IPAddress, oldHost.ID, hostToSave.ID))
				s.store.Delete(oldHost.IPAddress)
			}

			// Upsert the host immediately so it appears in the list
			if err := s.store.Upsert(hostToSave); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to upsert discovered host: %v", err))
				continue
			}

			if isNew {
				count++
				s.logger.Info(fmt.Sprintf("Discovered/Updated host: %s (ID: %s)", host.IP, hostToSave.ID))
			}

			// Trigger health check for EVERY discovered host
			wg.Add(1)
			go func(h types.Host) {
				defer wg.Done()
				hosts.CheckHealth(&h)
				if err := s.store.Upsert(h); err != nil {
					s.logger.Error(fmt.Sprintf("Error updating health for %s: %v", h.IPAddress, err))
				}
			}(hostToSave)

			// Mutual discovery: Push ourselves to them if we got details via /api/host/local
			if remoteHost.IPAddress != "" {
				go func(targetIP string) {
					if local, err := s.anthias.GetMetadata(); err == nil {
						if stored, err := s.store.GetByID(local.ID); err == nil {
							local = stored
						}
						hosts := []types.Host{*local}
						body, _ := json.Marshal(hosts)
						http.Post(fmt.Sprintf("http://%s:8080/api/hosts/receive?merge=true", targetIP), "application/json", bytes.NewBuffer(body))
					}
				}(host.IP)
			}
		}
		
		// Wait for all health checks to complete
		wg.Wait()

		// Finally, check ourselves (the local host)
		if local, err := s.anthias.GetMetadata(); err == nil {
			if stored, err := s.store.GetByID(local.ID); err == nil {
				updated := *stored
				hosts.CheckHealth(&updated)
				s.store.Upsert(updated)
				s.logger.Info("Local host health check complete.")
			}
		}

		s.logger.Info(fmt.Sprintf("Discovery scan complete. Processed %d hosts.", count))
	}()

	w.WriteHeader(http.StatusNoContent)
}
