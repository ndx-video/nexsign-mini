// Package web implements the HTTP server and HTMX-backed dashboard for
// nexSign mini. It serves templates and API endpoints for managing the
// host list manually via a web UI.
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"nexsign.mini/nsm/internal/anthias"
	"nexsign.mini/nsm/internal/discovery"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/types"
)

// TemplateData holds the data to be passed to the HTML template.
type TemplateData struct {
	Hosts          []types.Host
	SelectedHost   *types.Host
	CurrentHostIP  string
	CurrentVersion string
	Interfaces     []string
	EnvVarSet          bool
	DuplicateHostnames map[string]bool
}

// Server is the web server for the dashboard and API.
type Server struct {
	store     *hosts.Store
	anthias   *anthias.Client
	port      int
	templates *template.Template
}

// NewServer creates a new web server.
func NewServer(store *hosts.Store, anthiasClient *anthias.Client, port int) (*Server, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	s := &Server{
		store:     store,
		anthias:   anthiasClient,
		port:      port,
		templates: templates,
	}
	return s, nil
}

// Start initializes and runs the web server.
func (s *Server) Start() <-chan error {
	log.Printf("Web UI: Starting dashboard and API server on http://localhost:%d", s.port)

	fs := http.FileServer(http.Dir("internal/web/static"))
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Page routes
	mux.HandleFunc("/", s.handlePageLoad)
	mux.HandleFunc("/views/home", s.handleHomeView)
	mux.HandleFunc("/views/advanced", s.handleAdvancedView)

	// API routes
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/host/local", s.handleGetLocalHost)
	mux.HandleFunc("/api/hosts", s.handleGetHosts)
	mux.HandleFunc("/api/hosts/add", s.handleAddHost)
	mux.HandleFunc("/api/hosts/update", s.handleUpdateHost)
	mux.HandleFunc("/api/hosts/delete", s.handleDeleteHost)
	mux.HandleFunc("/api/hosts/set-primary", s.handleSetPrimaryHost)
	mux.HandleFunc("/api/hosts/check", s.handleCheckHosts)
	mux.HandleFunc("/api/hosts/stream", s.handleHostsStream)
	mux.HandleFunc("/api/hosts/push", s.handlePushHosts)
	mux.HandleFunc("/api/hosts/receive", s.handleReceiveHosts)
	mux.HandleFunc("/api/hosts/reboot", s.handleRebootHost)
	mux.HandleFunc("/api/hosts/upgrade", s.handleUpgradeHost)
	mux.HandleFunc("/api/discovery/scan", s.handleDiscoveryScan)
	mux.HandleFunc("/api/proxy/anthias", s.handleAnthiasProxy)

	addr := fmt.Sprintf(":%d", s.port)
	errCh := make(chan error, 1)

	go func() {
		err := http.ListenAndServe(addr, mux)
		errCh <- err
		close(errCh)
	}()

	return errCh
}

func (s *Server) handlePageLoad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)
	// Pass current version so layout can display it in the header
	err := s.templates.ExecuteTemplate(w, "layout.html", TemplateData{CurrentVersion: types.Version})
	if err != nil {
		log.Printf("Error executing layout template: %s", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *Server) handleHomeView(w http.ResponseWriter, r *http.Request) {
	// Get current host IP based on persistent ID
	currentIP := ""
	if localHost, err := s.anthias.GetMetadata(); err == nil {
		// Try to find this host in the store to get its user-preferred IP
		if storedHost, err := s.store.GetByID(localHost.ID); err == nil {
			currentIP = storedHost.IPAddress
		} else {
			// Fallback to detected IP
			currentIP = localHost.IPAddress
		}
	}

	// Get available interfaces
	var interfaces []string
	if ifaces, err := net.Interfaces(); err == nil {
		for _, i := range ifaces {
			if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := i.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil && ip.To4() != nil {
					interfaces = append(interfaces, ip.String())
				}
			}
		}
	}

	// Identify duplicate hostnames
	allHosts := s.store.GetAll()
	hostnameCounts := make(map[string]int)
	for _, h := range allHosts {
		if h.Hostname != "" && h.Hostname != "localhost" && h.Hostname != "unknown" {
			hostnameCounts[h.Hostname]++
		}
	}
	duplicateHostnames := make(map[string]bool)
	for name, count := range hostnameCounts {
		if count > 1 {
			duplicateHostnames[name] = true
		}
	}

	data := TemplateData{
		Hosts:              allHosts,
		CurrentHostIP:      currentIP,
		CurrentVersion:     types.Version,
		Interfaces:         interfaces,
		EnvVarSet:          os.Getenv("NSM_HOST_IP") != "",
		DuplicateHostnames: duplicateHostnames,
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "home-view.html", data); err != nil {
		log.Printf("Error executing home-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	s.setCacheHeaders(w)

	fmt.Fprintf(w, "event: datastar-merge-fragments\n")
	fmt.Fprintf(w, "data: fragments <div id=\"content-area\">\n")
	
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Fprintf(w, "data: fragments %s\n", line)
	}
	fmt.Fprintf(w, "data: fragments </div>\n\n")
}

func (s *Server) handleAdvancedView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	html := `<div id="content-area" class="p-4 text-desert-tan">
		<h2 class="text-xl font-semibold mb-3">Advanced Settings</h2>
		<p class="text-desert-gray">Not implemented yet.</p>
	</div>`

	fmt.Fprintf(w, "event: datastar-merge-fragments\n")
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "data: fragments %s\n", line)
	}
	fmt.Fprintf(w, "\n")
}

// Health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleVersion returns the current NSM version and Host ID
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Get local ID
	var id string
	if meta, err := s.anthias.GetMetadata(); err == nil {
		id = meta.ID
	}

	json.NewEncoder(w).Encode(map[string]string{
		"version": types.Version,
		"status":  "ok",
		"id":      id,
	})
}

// handleGetLocalHost returns the metadata of this specific host
func (s *Server) handleGetLocalHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	meta, err := s.anthias.GetMetadata()
	if err != nil {
		http.Error(w, "Failed to get local metadata", http.StatusInternalServerError)
		return
	}

	// Try to get stored version to include any user customizations (Nickname, etc)
	if stored, err := s.store.GetByID(meta.ID); err == nil {
		json.NewEncoder(w).Encode(stored)
		return
	}

	// Fallback to raw metadata
	json.NewEncoder(w).Encode(meta)
}

func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.store.GetAll()); err != nil {
		log.Printf("Error encoding hosts to JSON: %s", err)
		http.Error(w, "Failed to retrieve host list", http.StatusInternalServerError)
	}
}

// handleAddHost adds a new host to the list
func (s *Server) handleAddHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Nickname  string `json:"nickname"`
		IPAddress string `json:"ip_address"`
		VPNIP     string `json:"vpn_ip_address"`
		Notes     string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ip := strings.TrimSpace(req.IPAddress)
	vpnIP := strings.TrimSpace(req.VPNIP)
	nickname := strings.TrimSpace(req.Nickname)
	notes := strings.TrimSpace(req.Notes)

	if !isValidIPv4(ip) {
		http.Error(w, "Valid LAN IP address is required", http.StatusBadRequest)
		return
	}

	if vpnIP != "" && !isValidIPv4(vpnIP) {
		http.Error(w, "VPN IP address must be a valid IPv4 address", http.StatusBadRequest)
		return
	}

	host := types.Host{
		Nickname:      nickname,
		IPAddress:     ip,
		VPNIPAddress:  vpnIP,
		Notes:         notes,
		Status:        types.StatusUnreachable,
		StatusVPN:     "",
		NSMStatus:     "NSM Offline",
		NSMStatusVPN:  "",
		NSMVersion:    "unknown",
		NSMVersionVPN: "",
		CMSStatus:     types.CMSUnknown,
		CMSStatusVPN:  types.CMSUnknown,
		DashboardURL:  fmt.Sprintf("http://%s:8080", ip),
		LastChecked:   time.Time{},
	}

	if vpnIP != "" {
		host.StatusVPN = types.StatusUnreachable
		host.NSMStatusVPN = "NSM Offline"
		host.NSMVersionVPN = "unknown"
		host.DashboardURLVPN = fmt.Sprintf("http://%s:8080", vpnIP)
	}

	if err := s.store.Add(host); err != nil {
		log.Printf("Error adding host: %s", err)
		http.Error(w, "Failed to add host", http.StatusInternalServerError)
		return
	}

	// Check health of new host
	go func(base types.Host) {
		updated := base
		hosts.CheckHealth(&updated)
		if err := s.store.Update(base.IPAddress, func(h *types.Host) {
			copyNetworkState(h, &updated)
			if updated.Hostname != "" {
				h.Hostname = updated.Hostname
			}
		}); err != nil {
			log.Printf("Error persisting host health for %s: %v", base.IPAddress, err)
		}
	}(host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleUpdateHost updates an existing host
func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var updateReq struct {
		OldIP        string `json:"old_ip"`
		IPAddress    string `json:"ip_address"`
		VPNIPAddress string `json:"vpn_ip_address"`
		Nickname     string `json:"nickname"`
		Notes        string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	newIP := strings.TrimSpace(updateReq.IPAddress)
	newVPN := strings.TrimSpace(updateReq.VPNIPAddress)
	newNickname := strings.TrimSpace(updateReq.Nickname)
	newNotes := strings.TrimSpace(updateReq.Notes)

	if !isValidIPv4(newIP) {
		http.Error(w, "Valid LAN IP address is required", http.StatusBadRequest)
		return
	}

	if newVPN != "" && !isValidIPv4(newVPN) {
		http.Error(w, "VPN IP address must be a valid IPv4 address", http.StatusBadRequest)
		return
	}

	err := s.store.Update(updateReq.OldIP, func(h *types.Host) {
		if newIP != "" {
			ipChanged := newIP != h.IPAddress
			h.IPAddress = newIP
			h.DashboardURL = fmt.Sprintf("http://%s:8080", newIP)
			if ipChanged {
				h.Status = types.StatusUnreachable
				h.NSMStatus = "NSM Offline"
				h.NSMVersion = "unknown"
				h.CMSStatus = types.CMSUnknown
				h.AssetCount = 0
				h.LastChecked = time.Time{}
			}
		}

		if newVPN == "" {
			h.VPNIPAddress = ""
			h.StatusVPN = ""
			h.NSMStatusVPN = ""
			h.NSMVersionVPN = ""
			h.CMSStatusVPN = types.CMSUnknown
			h.AssetCountVPN = 0
			h.DashboardURLVPN = ""
			h.LastCheckedVPN = time.Time{}
		} else {
			vpnChanged := newVPN != h.VPNIPAddress
			h.VPNIPAddress = newVPN
			h.DashboardURLVPN = fmt.Sprintf("http://%s:8080", newVPN)
			if vpnChanged {
				h.StatusVPN = types.StatusUnreachable
				h.NSMStatusVPN = "NSM Offline"
				h.NSMVersionVPN = "unknown"
				h.CMSStatusVPN = types.CMSUnknown
				h.AssetCountVPN = 0
				h.LastCheckedVPN = time.Time{}
			}
		}

		h.Nickname = newNickname
		h.Notes = newNotes
	})

	if err != nil {
		log.Printf("Error updating host: %s", err)
		http.Error(w, "Failed to update host", http.StatusInternalServerError)
		return
	}

	if updatedHost, getErr := s.store.GetByIP(newIP); getErr == nil {
		go func(toRefresh *types.Host) {
			hosts.CheckHealth(toRefresh)
			if err := s.store.Update(toRefresh.IPAddress, func(h *types.Host) {
				copyNetworkState(h, toRefresh)
				if toRefresh.Hostname != "" {
					h.Hostname = toRefresh.Hostname
				}
			}); err != nil {
				log.Printf("Error refreshing host %s after update: %v", toRefresh.IPAddress, err)
			}
		}(updatedHost)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDeleteHost removes a host from the list
func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "IP address parameter required", http.StatusBadRequest)
		return
	}

	if err := s.store.Delete(ip); err != nil {
		log.Printf("Error deleting host: %s", err)
		http.Error(w, "Failed to delete host", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSetPrimaryHost sets the selected host as primary and removes duplicates
func (s *Server) handleSetPrimaryHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID parameter required", http.StatusBadRequest)
		return
	}

	// Get the target host
	target, err := s.store.GetByID(id)
	if err != nil {
		http.Error(w, "Host not found", http.StatusNotFound)
		return
	}

	if target.Hostname == "" || target.Hostname == "localhost" || target.Hostname == "unknown" {
		http.Error(w, "Cannot set primary for host with invalid hostname", http.StatusBadRequest)
		return
	}

	// Find all hosts with same hostname
	allHosts := s.store.GetAll()
	deletedCount := 0
	for _, h := range allHosts {
		if h.Hostname == target.Hostname && h.ID != target.ID {
			if err := s.store.Delete(h.IPAddress); err != nil {
				log.Printf("Failed to delete duplicate host %s: %v", h.IPAddress, err)
			} else {
				deletedCount++
			}
		}
	}

	log.Printf("Set %s (%s) as primary for hostname %s. Deleted %d duplicates.", target.IPAddress, target.ID, target.Hostname, deletedCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleCheckHosts triggers health check on all hosts
func (s *Server) handleCheckHosts(w http.ResponseWriter, r *http.Request) {
	// Datastar action can be a GET or POST.
	// If triggered via @post('/api/hosts/check'), it expects a response.
	// We can just return 204 No Content or empty body, as the update will come via SSE.
	go s.store.CheckAllHosts()
	w.WriteHeader(http.StatusNoContent)
}

// handleHostsStream streams host list updates via Server-Sent Events for Datastar
func (s *Server) handleHostsStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Helper to render and send the host list
	sendUpdate := func() error {
		// Get current host IP based on persistent ID
		currentIP := ""
		if localHost, err := s.anthias.GetMetadata(); err == nil {
			// Try to find this host in the store to get its user-preferred IP
			if storedHost, err := s.store.GetByID(localHost.ID); err == nil {
				currentIP = storedHost.IPAddress
			} else {
				// Fallback to detected IP
				currentIP = localHost.IPAddress
			}
		}

		// Identify duplicate hostnames
		allHosts := s.store.GetAll()
		hostnameCounts := make(map[string]int)
		for _, h := range allHosts {
			if h.Hostname != "" && h.Hostname != "localhost" && h.Hostname != "unknown" {
				hostnameCounts[h.Hostname]++
			}
		}
		duplicateHostnames := make(map[string]bool)
		for name, count := range hostnameCounts {
			if count > 1 {
				duplicateHostnames[name] = true
			}
		}

		data := TemplateData{
			Hosts:              allHosts,
			CurrentHostIP:      currentIP,
			CurrentVersion:     types.Version,
			DuplicateHostnames: duplicateHostnames,
		}

		var buf bytes.Buffer
		if err := s.templates.ExecuteTemplate(&buf, "host-rows", data); err != nil {
			return err
		}

		// Send datastar-merge-fragments event
		// Datastar expects:
		// event: datastar-merge-fragments
		// data: fragments <fragment>...
		
		fmt.Fprintf(w, "event: datastar-merge-fragments\n")
		
		lines := strings.Split(buf.String(), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(w, "data: fragments %s\n", line)
		}
		fmt.Fprintf(w, "\n")
		flusher.Flush()
		return nil
	}

	// Send initial state
	if err := sendUpdate(); err != nil {
		log.Printf("Error sending initial SSE update: %v", err)
		return
	}

	updates := s.store.Updates()
	ticker := time.NewTicker(30 * time.Second) // Keep-alive / periodic refresh
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			if err := sendUpdate(); err != nil {
				log.Printf("Error sending SSE update: %v", err)
				return
			}
		case <-ticker.C:
			// Optional: send a comment or empty event to keep connection alive
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

// handlePushHosts pushes the current host list to all other hosts
func (s *Server) handlePushHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Optional list of specific target IPs to push to
	var req struct {
		Targets []string `json:"targets"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	targetSet := map[string]struct{}{}
	for _, t := range req.Targets {
		t = strings.TrimSpace(t)
		if t != "" {
			targetSet[t] = struct{}{}
		}
	}

	allHosts := s.store.GetAll()
	hostListJSON, err := json.Marshal(allHosts)
	if err != nil {
		http.Error(w, "Failed to marshal host list", http.StatusInternalServerError)
		return
	}

	results := make(map[string]string)

	// Push to each host (except localhost)
	for _, host := range allHosts {
		if host.IPAddress == "127.0.0.1" {
			continue
		}

		// If a target list is provided, only include those
		if len(targetSet) > 0 {
			if _, ok := targetSet[host.IPAddress]; !ok {
				continue
			}
		}

		go func(h types.Host) {
			url := fmt.Sprintf("http://%s:8080/api/hosts/receive", h.IPAddress)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(hostListJSON))
			if err != nil {
				log.Printf("Failed to push to %s: %v", h.IPAddress, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				log.Printf("Successfully pushed host list to %s", h.IPAddress)
			} else {
				log.Printf("Failed to push to %s: HTTP %d", h.IPAddress, resp.StatusCode)
			}
		}(host)

		results[host.IPAddress] = "pushed"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"results": results,
	})
}

// handleReceiveHosts receives pushed host list from another host
func (s *Server) handleReceiveHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var hosts []types.Host
	if err := json.NewDecoder(r.Body).Decode(&hosts); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	merge := r.URL.Query().Get("merge") == "true"

	if merge {
		// Merge mode: Upsert received hosts, do not delete anything
		count := 0
		for _, h := range hosts {
			if err := s.store.Upsert(h); err != nil {
				log.Printf("Error merging host %s: %v", h.ID, err)
			} else {
				count++
			}
		}
		log.Printf("Merged %d hosts from remote announcement", count)
	} else {
		// Replace mode: Overwrite entire host list (default for "Push to fleet")
		backupPath, err := s.store.BackupCurrent(20)
		if err != nil {
			log.Printf("Error creating host backup: %v", err)
			http.Error(w, "Failed to backup existing host list", http.StatusInternalServerError)
			return
		}

		if err := s.store.ReplaceAll(hosts); err != nil {
			log.Printf("Error replacing host list: %s", err)
			http.Error(w, "Failed to update host list", http.StatusInternalServerError)
			return
		}

		if backupPath != "" {
			log.Printf("Received host list; previous copy moved to %s (count: %d)", backupPath, len(hosts))
		} else {
			log.Printf("Received host list from remote host (count: %d)", len(hosts))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRebootHost handles reboot requests for hosts
func (s *Server) handleRebootHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the target IP and origin from request
	var req struct {
		TargetIP string `json:"target_ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetIP == "" {
		http.Error(w, "target_ip is required", http.StatusBadRequest)
		return
	}

	// Get the origin IP from the request
	originIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(originIP); err == nil {
		originIP = host
	}

	// If the target is localhost, execute the reboot
	localHost, err := s.anthias.GetMetadata()
	if err != nil {
		log.Printf("Error getting local metadata: %v", err)
		http.Error(w, "Failed to determine local host", http.StatusInternalServerError)
		return
	}

	if req.TargetIP == localHost.IPAddress || req.TargetIP == "127.0.0.1" {
		// This is a reboot request for THIS host
		log.Printf("REBOOT REQUEST received from %s for local host %s", originIP, req.TargetIP)

		// Execute the reboot sequence
		go func() {
			log.Println("Initiating safe reboot sequence...")
			log.Println("Step 1: Stopping Docker engine...")

			// Stop docker engine
			if err := exec.Command("systemctl", "stop", "docker").Run(); err != nil {
				log.Printf("Warning: Failed to stop docker: %v", err)
			}

			time.Sleep(2 * time.Second)

			log.Println("Step 2: Requesting system reboot...")

			// Reboot the system
			if err := exec.Command("systemctl", "reboot").Run(); err != nil {
				log.Printf("Error: Failed to initiate reboot: %v", err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "rebooting",
			"message": "Reboot sequence initiated",
		})
		return
	}

	// Forward the reboot request to the target host
	targetURL := fmt.Sprintf("http://%s:8080/api/hosts/reboot", req.TargetIP)
	reqBody, _ := json.Marshal(req)

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to forward reboot request to %s: %v", req.TargetIP, err)
		http.Error(w, "Failed to reach target host", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Forward the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleUpgradeHost handles package upgrade requests for hosts
func (s *Server) handleUpgradeHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the target IP and origin from request
	var req struct {
		TargetIP string `json:"target_ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetIP == "" {
		http.Error(w, "target_ip is required", http.StatusBadRequest)
		return
	}

	// Get the origin IP from the request
	originIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(originIP); err == nil {
		originIP = host
	}

	// If the target is localhost, execute the upgrade
	localHost, err := s.anthias.GetMetadata()
	if err != nil {
		log.Printf("Error getting local metadata: %v", err)
		http.Error(w, "Failed to determine local host", http.StatusInternalServerError)
		return
	}

	if req.TargetIP == localHost.IPAddress || req.TargetIP == "127.0.0.1" {
		// This is an upgrade request for THIS host
		log.Printf("UPGRADE REQUEST received from %s for local host %s", originIP, req.TargetIP)

		// Execute the upgrade sequence
		go func() {
			log.Println("Initiating package upgrade sequence...")
			log.Println("Step 1: Running apt update...")

			// Run apt update
			if err := exec.Command("apt", "update").Run(); err != nil {
				log.Printf("Warning: Failed to run apt update: %v", err)
			}

			log.Println("Step 2: Running apt upgrade...")

			// Run apt upgrade -y
			if err := exec.Command("apt", "upgrade", "-y").Run(); err != nil {
				log.Printf("Error: Failed to run apt upgrade: %v", err)
			}

			log.Println("Package upgrade complete. Service will restart if nsm was updated.")
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "upgrading",
			"message": "Package upgrade initiated",
		})
		return
	}

	// Forward the upgrade request to the target host
	targetURL := fmt.Sprintf("http://%s:8080/api/hosts/upgrade", req.TargetIP)
	reqBody, _ := json.Marshal(req)

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to forward upgrade request to %s: %v", req.TargetIP, err)
		http.Error(w, "Failed to reach target host", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Forward the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleDiscoveryScan initiates a network scan for other NSM instances
func (s *Server) handleDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for interface override from query param
	overrideIP := r.URL.Query().Get("interface_ip")
	if overrideIP == "" {
		overrideIP = os.Getenv("NSM_HOST_IP")
	}

	go func() {
		log.Println("Starting network discovery scan...")
		scanner := discovery.NewScanner(s.port, overrideIP)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		results, err := scanner.Scan(ctx)
		if err != nil {
			log.Printf("Discovery scan failed: %v", err)
			return
		}

		count := 0
		for host := range results {
			// Try to get remote details
			var remoteHost types.Host
			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s:%d/api/host/local", host.IP, host.Port))
			if err == nil {
				if json.NewDecoder(resp.Body).Decode(&remoteHost) == nil {
					// We have full details!
					// Ensure IP is correct (trust the discovery IP for reachability)
					remoteHost.IPAddress = host.IP
					remoteHost.DashboardURL = fmt.Sprintf("http://%s:%d", host.IP, host.Port)
					
					// Check for stale entries with same IP but different ID
					if oldHost, err := s.store.GetByIP(host.IP); err == nil && oldHost.ID != remoteHost.ID {
						log.Printf("Replacing stale host %s (ID: %s) with discovered ID %s", oldHost.IPAddress, oldHost.ID, remoteHost.ID)
						s.store.Delete(oldHost.IPAddress)
					}

					if err := s.store.Upsert(remoteHost); err != nil {
						log.Printf("Failed to upsert discovered host: %v", err)
					} else {
						log.Printf("Discovered and updated host: %s (ID: %s)", host.IP, remoteHost.ID)
					}
                    
                    // Mutual discovery: Push ourselves to them
                    go func(targetIP string) {
                        if local, err := s.anthias.GetMetadata(); err == nil {
                            // Get stored version for full details
                            if stored, err := s.store.GetByID(local.ID); err == nil {
                                local = stored
                            }
                            // Wrap in list
                            hosts := []types.Host{*local}
                            body, _ := json.Marshal(hosts)
                            http.Post(fmt.Sprintf("http://%s:8080/api/hosts/receive?merge=true", targetIP), "application/json", bytes.NewBuffer(body))
                        }
                    }(host.IP)
				}
				resp.Body.Close()
                continue
			}

			// Fallback for older versions (try /api/version)
			// ... (keep existing logic or just skip?)
			// If /api/host/local fails, maybe it's an old version.
			// Let's keep the old logic as fallback?
			// The user said "regular checking is not required".
			// If we fail to get details, we can fall back to "Discovered Host".
			
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

			// ... (rest of old logic)

			// If we have an ID, check by ID
			if remoteID != "" {
				if existing, err := s.store.GetByID(remoteID); err == nil {
					// Host exists, update IP if changed
					if existing.IPAddress != host.IP {
						log.Printf("Host %s moved from %s to %s", remoteID, existing.IPAddress, host.IP)
						existing.IPAddress = host.IP
						existing.DashboardURL = fmt.Sprintf("http://%s:%d", host.IP, host.Port)
						existing.Status = types.StatusUnreachable // Reset status
						s.store.Upsert(*existing)
						
						// Trigger health check
						go func(h types.Host) {
							updated := h
							hosts.CheckHealth(&updated)
							s.store.Upsert(updated)
						}(*existing)
					}
					continue // Already exists and updated if needed
				} else {
					// ID not found in DB. Check if we have a stale host with this IP.
					if oldHost, err := s.store.GetByIP(host.IP); err == nil {
						// We have a host at this IP, but with a different ID (since GetByID failed).
						log.Printf("Replacing stale host %s (ID: %s) with discovered ID %s", oldHost.IPAddress, oldHost.ID, remoteID)
						if err := s.store.Delete(oldHost.IPAddress); err != nil {
							log.Printf("Failed to delete stale host: %v", err)
						}
					}
				}
			} else {
				// Fallback to IP check
				if _, err := s.store.GetByIP(host.IP); err == nil {
					continue // Already exists by IP
				}
			}

			// Add new host
			newHost := types.Host{
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

			if err := s.store.Upsert(newHost); err != nil {
				log.Printf("Failed to add discovered host %s: %v", host.IP, err)
				continue
			}
			count++
			log.Printf("Discovered and added new host: %s (ID: %s)", host.IP, remoteID)

			// Check health of new host
			go func(base types.Host) {
				updated := base
				hosts.CheckHealth(&updated)
				if err := s.store.Upsert(updated); err != nil {
					log.Printf("Error refreshing host %s after update: %v", updated.IPAddress, err)
				}
			}(newHost)
		}
		log.Printf("Discovery scan complete. Added %d new hosts.", count)
	}()

	w.WriteHeader(http.StatusNoContent)
}

// handleAnthiasProxy proxies requests to Anthias devices to avoid CORS issues.
func (s *Server) handleAnthiasProxy(w http.ResponseWriter, r *http.Request) {
	// Get IP and path from query parameters
	ip := r.URL.Query().Get("ip")
	path := r.URL.Query().Get("path")

	if ip == "" || path == "" {
		http.Error(w, "Missing 'ip' or 'path' parameter", http.StatusBadRequest)
		return
	}

	// Build the target URL
	targetURL := fmt.Sprintf("http://%s%s", ip, path)

	// Create the proxy request
	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to reach Anthias device: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Copy status code and body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// setCacheHeaders sets cache-busting headers to prevent browser caching.
// These headers ensure fresh content in development and production.
func (s *Server) setCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func isValidIPv4(ip string) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

func copyNetworkState(dst, src *types.Host) {
	dst.Status = src.Status
	dst.CMSStatus = src.CMSStatus
	dst.AssetCount = src.AssetCount
	dst.NSMStatus = src.NSMStatus
	dst.NSMVersion = src.NSMVersion
	dst.DashboardURL = src.DashboardURL
	dst.LastChecked = src.LastChecked

	dst.StatusVPN = src.StatusVPN
	dst.CMSStatusVPN = src.CMSStatusVPN
	dst.AssetCountVPN = src.AssetCountVPN
	dst.NSMStatusVPN = src.NSMStatusVPN
	dst.NSMVersionVPN = src.NSMVersionVPN
	dst.DashboardURLVPN = src.DashboardURLVPN
	dst.LastCheckedVPN = src.LastCheckedVPN
}

// tryGorillaUpgrade attempts to upgrade the connection using gorilla/websocket
// if it is linked into the binary. This avoids a hard dependency in case the
// module isn't available during certain builds.
// tryGorillaUpgrade is implemented in websocket_shim.go (using gorilla/websocket)
