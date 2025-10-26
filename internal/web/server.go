package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"nexsign.mini/nsm/internal/ledger"
)

// TemplateData holds the data to be passed to the HTML template.
type TemplateData struct {
	Hosts        map[string]ledger.Host
	SelectedHost *ledger.Host
}

// Server is the web server for the dashboard and API.
type Server struct {
	state     *ledger.State
	port      int
	templates *template.Template
}

// NewServer creates a new web server.
func NewServer(state *ledger.State, port int) (*Server, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		state:     state,
		port:      port,
		templates: templates,
	}, nil
}

// Start initializes and runs the web server.
func (s *Server) Start() {
	log.Printf("Web UI: Starting dashboard and API server on http://localhost:%d", s.port)

	http.HandleFunc("/", s.handleDashboard)
	http.HandleFunc("/api/hosts", s.handleGetHosts)

	addr := fmt.Sprintf(":%d", s.port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Error starting web server: %s", err)
		}
	}()
}

// handleDashboard renders the main dashboard page.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host")

	data := TemplateData{
		Hosts: s.state.Hosts,
	}

	if host, ok := s.state.Hosts[hostID]; ok {
		data.SelectedHost = &host
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %s", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// handleGetHosts provides the list of all known hosts as JSON.
func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.state.Hosts); err != nil {
		log.Printf("Error encoding hosts to JSON: %s", err)
		http.Error(w, "Failed to retrieve host list", http.StatusInternalServerError)
	}
}