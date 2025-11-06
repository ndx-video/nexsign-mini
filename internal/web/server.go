// Package web implements the HTTP server and HTMX-backed dashboard for
// nexSign mini. It serves templates and API endpoints for managing the
// host list manually via a web UI.
package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"nexsign.mini/nsm/internal/anthias"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/types"
)

// TemplateData holds the data to be passed to the HTML template.
type TemplateData struct {
	Hosts        []types.Host
	SelectedHost *types.Host
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
func (s *Server) Start() {
	log.Printf("Web UI: Starting dashboard and API server on http://localhost:%d", s.port)

	fs := http.FileServer(http.Dir("internal/web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Page routes
	http.HandleFunc("/", s.handlePageLoad)
	http.HandleFunc("/views/home", s.handleHomeView)

	// API routes
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/hosts", s.handleGetHosts)
	http.HandleFunc("/api/hosts/add", s.handleAddHost)
	http.HandleFunc("/api/hosts/update", s.handleUpdateHost)
	http.HandleFunc("/api/hosts/delete", s.handleDeleteHost)
	http.HandleFunc("/api/hosts/check", s.handleCheckHosts)
	http.HandleFunc("/api/hosts/push", s.handlePushHosts)
	http.HandleFunc("/api/hosts/receive", s.handleReceiveHosts)

	addr := fmt.Sprintf(":%d", s.port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Error starting web server: %s", err)
		}
	}()
}

func (s *Server) handlePageLoad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)
	err := s.templates.ExecuteTemplate(w, "layout.html", nil)
	if err != nil {
		log.Printf("Error executing layout template: %s", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *Server) handleHomeView(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{Hosts: s.store.GetAll()}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)

	err := s.templates.ExecuteTemplate(w, "home-view.html", data)
	if err != nil {
		log.Printf("Error executing home-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
	}
}

// Health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	var host types.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if host.IPAddress == "" {
		http.Error(w, "IP address is required", http.StatusBadRequest)
		return
	}

	// Set initial status
	host.Status = types.StatusUnreachable
	host.LastChecked = time.Now()
	if host.DashboardURL == "" {
		host.DashboardURL = fmt.Sprintf("http://%s:8080", host.IPAddress)
	}

	if err := s.store.Add(host); err != nil {
		log.Printf("Error adding host: %s", err)
		http.Error(w, "Failed to add host", http.StatusInternalServerError)
		return
	}

	// Check health of new host
	go func() {
		host.Status = hosts.CheckHealth(&host)
		host.LastChecked = time.Now()
		s.store.Update(host.IPAddress, func(h *types.Host) {
			h.Status = host.Status
			h.LastChecked = host.LastChecked
		})
	}()

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
		OldIP    string `json:"old_ip"`
		IPAddress string `json:"ip_address"`
		Hostname  string `json:"hostname"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := s.store.Update(updateReq.OldIP, func(h *types.Host) {
		if updateReq.IPAddress != "" {
			h.IPAddress = updateReq.IPAddress
			h.DashboardURL = fmt.Sprintf("http://%s:8080", updateReq.IPAddress)
		}
		if updateReq.Hostname != "" {
			h.Hostname = updateReq.Hostname
		}
	})

	if err != nil {
		log.Printf("Error updating host: %s", err)
		http.Error(w, "Failed to update host", http.StatusInternalServerError)
		return
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

// handleCheckHosts triggers health check on all hosts
func (s *Server) handleCheckHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go s.store.CheckAllHosts()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "checking"})
}

// handlePushHosts pushes the current host list to all other hosts
func (s *Server) handlePushHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
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

	if err := s.store.ReplaceAll(hosts); err != nil {
		log.Printf("Error replacing host list: %s", err)
		http.Error(w, "Failed to update host list", http.StatusInternalServerError)
		return
	}

	log.Printf("Received and updated host list from remote host (count: %d)", len(hosts))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// setCacheHeaders sets cache-busting headers to prevent browser caching.
// These headers ensure fresh content in development and production.
func (s *Server) setCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// tryGorillaUpgrade attempts to upgrade the connection using gorilla/websocket
// if it is linked into the binary. This avoids a hard dependency in case the
// module isn't available during certain builds.
// tryGorillaUpgrade is implemented in websocket_shim.go (using gorilla/websocket)
