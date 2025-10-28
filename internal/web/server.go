// Package web implements the HTTP server and HTMX-backed dashboard for
// nexSign mini. It serves templates and API endpoints that present the
// distributed ledger's host list and provides UI-driven actions (e.g.
// restart host) which are translated into signed transactions and
// broadcast to the network.
package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"nexsign.mini/nsm/internal/abci"
	"nexsign.mini/nsm/internal/types"
)

// TemplateData holds the data to be passed to the HTML template.
type TemplateData struct {
	Hosts        map[string]types.Host
	SelectedHost *types.Host
}

// Server is the web server for the dashboard and API.
type Server struct {
	app       *abci.ABCIApplication
	port      int
	templates *template.Template
}

// NewServer creates a new web server.
func NewServer(app *abci.ABCIApplication, port int) (*Server, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		app:       app,
		port:      port,
		templates: templates,
	}, nil
}

// Start initializes and runs the web server.
func (s *Server) Start() {
	log.Printf("Web UI: Starting dashboard and API server on http://localhost:%d", s.port)

	fs := http.FileServer(http.Dir("internal/web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", s.handlePageLoad)
	http.HandleFunc("/views/home", s.handleHomeView)
	http.HandleFunc("/views/host", s.handleHostView)
	http.HandleFunc("/api/hosts", s.handleGetHosts)

	addr := fmt.Sprintf(":%d", s.port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Error starting web server: %s", err)
		}
	}()
}

func (s *Server) handlePageLoad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "layout.html", nil)
	if err != nil {
		log.Printf("Error executing layout template: %s", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *Server) handleHomeView(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{Hosts: s.app.GetState()}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "home-view.html", data)
	if err != nil {
		log.Printf("Error executing home-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
	}
}

func (s *Server) handleHostView(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host")
	data := TemplateData{}

	currentState := s.app.GetState()
	if host, ok := currentState[hostID]; ok {
		data.SelectedHost = &host
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.templates.ExecuteTemplate(w, "host-view.html", data)
	if err != nil {
		log.Printf("Error executing host-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
	}
}

func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.app.GetState()); err != nil {
		log.Printf("Error encoding hosts to JSON: %s", err)
		http.Error(w, "Failed to retrieve host list", http.StatusInternalServerError)
	}
}
