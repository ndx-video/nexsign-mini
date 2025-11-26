package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/types"
)

// @Title: Get All Hosts
// @Route: GET /api/hosts
// @Description: Get all hosts in the fleet
// @Response: Array of Host objects
func (s *Service) HandleHosts(w http.ResponseWriter, r *http.Request) {
	// s.logger.Info("Retrieving all hosts") // Too noisy for polling?
	// The user said "especially the api endpoints".
	// Let's log it but maybe we can filter it out in the UI if it's too much.
	// Actually, HandleHosts is polled by the UI? No, UI uses SSE.
	// So this is likely manual or external.
	s.logger.Info("API: Get all hosts")
	s.writeJSON(w, http.StatusOK, s.store.GetAll())
}

// @Title: Add Host
// @Route: POST /api/hosts/add
// @Description: Add a new host to the fleet
// @Response: 204 No Content
func (s *Service) HandleAddHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Nickname    string `json:"nickname"`
		IPAddress   string `json:"ip_address"`
		VPNIPAddress string `json:"vpn_ip_address"`
		Notes       string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Basic validation
	if req.IPAddress == "" && req.VPNIPAddress == "" {
		s.writeError(w, http.StatusBadRequest, "At least one IP address is required")
		return
	}

	newHost := types.Host{
		ID:           uuid.New().String(),
		Nickname:     req.Nickname,
		IPAddress:    req.IPAddress,
		VPNIPAddress: req.VPNIPAddress,
		Notes:        req.Notes,
		Status:       types.StatusUnreachable,
		StatusVPN:    types.StatusUnreachable, // Default
		CMSStatus:    types.CMSUnknown,
		LastChecked:  time.Now(),
	}

	// Initial health check
	hosts.CheckHealth(&newHost)

	if err := s.store.Add(newHost); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add host: %v", err))
		return
	}

	s.logger.Info(fmt.Sprintf("Added new host: %s (%s)", req.Nickname, req.IPAddress))
	w.WriteHeader(http.StatusNoContent)
}

// @Title: Update Host
// @Route: POST /api/hosts/update
// @Description: Update an existing host
// @Response: 204 No Content
func (s *Service) HandleUpdateHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID           string `json:"id"`
		Nickname     string `json:"nickname"`
		IPAddress    string `json:"ip_address"`
		VPNIPAddress string `json:"vpn_ip_address"`
		Notes        string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	host, err := s.store.GetByID(req.ID)
	if err != nil {
		// Fallback: try to find by IP if ID is missing (legacy support)
		if req.ID == "" && req.IPAddress != "" {
			host, err = s.store.GetByIP(req.IPAddress)
		}
		if err != nil {
			s.writeError(w, http.StatusNotFound, "Host not found")
			return
		}
	}

	// Update fields
	host.Nickname = req.Nickname
	host.IPAddress = req.IPAddress
	host.VPNIPAddress = req.VPNIPAddress
	host.Notes = req.Notes

	// Re-check health if IPs changed
	hosts.CheckHealth(host)

	if err := s.store.Upsert(*host); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update host: %v", err))
		return
	}

	s.logger.Info(fmt.Sprintf("Updated host: %s", host.ID))
	w.WriteHeader(http.StatusNoContent)
}

// @Title: Delete Host
// @Route: POST /api/hosts/delete?ip=...
// @Description: Delete a host from the fleet
// @Response: 204 No Content
func (s *Service) HandleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		s.writeError(w, http.StatusBadRequest, "Missing 'ip' query parameter")
		return
	}

	if err := s.store.Delete(ip); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete host: %v", err))
		return
	}

	s.logger.Info(fmt.Sprintf("API: Deleted host: %s", ip))
	w.WriteHeader(http.StatusNoContent)
}

// @Title: Set Primary Host
// @Route: POST /api/hosts/set-primary?id=...
// @Description: Set a host as primary and remove duplicates
// @Response: 204 No Content
func (s *Service) HandleSetPrimaryHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "Missing 'id' query parameter")
		return
	}

	primary, err := s.store.GetByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Find duplicates by hostname
	allHosts := s.store.GetAll()
	count := 0
	for _, h := range allHosts {
		if h.Hostname == primary.Hostname && h.ID != primary.ID {
			s.store.Delete(h.IPAddress)
			count++
		}
	}

	s.logger.Info(fmt.Sprintf("API: Set %s as primary for %s, removed %d duplicates", primary.IPAddress, primary.Hostname, count))
	w.WriteHeader(http.StatusNoContent)
}

// @Title: Check All Hosts
// @Route: POST /api/hosts/check
// @Description: Trigger health check on all hosts
// @Response: 204 No Content
func (s *Service) HandleCheckHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		s.logger.Info("API: Starting manual health check of all hosts...")
		s.store.CheckAllHosts()
		s.logger.Info("Manual health check complete")
	}()

	w.WriteHeader(http.StatusNoContent)
}

// @Title: Check Single Host
// @Route: POST /api/hosts/check-one?ip=...
// @Description: Trigger health check for a specific host
// @Response: 204 No Content
func (s *Service) HandleCheckHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		s.writeError(w, http.StatusBadRequest, "Missing 'ip' query parameter")
		return
	}

	host, err := s.store.GetByIP(ip)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Host not found")
		return
	}

	go func(h types.Host) {
		s.logger.Info(fmt.Sprintf("API: Checking health for %s...", h.IPAddress))
		updated := h
		hosts.CheckHealth(&updated)
		if err := s.store.Upsert(updated); err != nil {
			s.logger.Error(fmt.Sprintf("Error updating health for %s: %v", h.IPAddress, err))
		}
	}(*host)

	w.WriteHeader(http.StatusNoContent)
}
