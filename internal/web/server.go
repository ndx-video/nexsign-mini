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

	fs := http.FileServer(http.Dir("internal/web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", s.handlePageLoad) // Main page load
	http.HandleFunc("/views/home", s.handleHomeView) // HTMX home view
	http.HandleFunc("/views/host", s.handleHostView) // HTMX host view

	http.HandleFunc("/api/hosts", s.handleGetHosts)

	addr := fmt.Sprintf(":%d", s.port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Error starting web server: %s", err)
		}
	}()
}

// handlePageLoad serves the main layout which then loads content via HTMX.
func (s *Server) handlePageLoad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "layout.html", nil)
	if err != nil {
		log.Printf("Error executing layout template: %s", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// handleHomeView serves the list of hosts for HTMX.
func (s *Server) handleHomeView(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{Hosts: s.state.Hosts}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "home-view.html", data)
	if err != nil {
		log.Printf("Error executing home-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
	}
}

// handleHostView serves the iframe for a selected host for HTMX.
func (s *Server) handleHostView(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host")
	data := TemplateData{}
	if host, ok := s.state.Hosts[hostID]; ok {
		data.SelectedHost = &host
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "host-view.html", data)
	if err != nil {
		log.Printf("Error executing host-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
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
