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
	"time"

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
	nodeID    string
}

// NewServer creates a new web server.
func NewServer(app *abci.ABCIApplication, port int, opts ...func(*Server)) (*Server, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	s := &Server{
		app:       app,
		port:      port,
		templates: templates,
	}
	// apply optional configurators
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Start initializes and runs the web server.
func (s *Server) Start() {
	log.Printf("Web UI: Starting dashboard and API server on http://localhost:%d", s.port)

	fs := http.FileServer(http.Dir("internal/web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", s.handlePageLoad)
	http.HandleFunc("/views/home", s.handleHomeView)
	http.HandleFunc("/views/host", s.handleHostView)
	http.HandleFunc("/views/advanced", s.handleAdvancedView)
	http.HandleFunc("/api/hosts", s.handleGetHosts)
	http.HandleFunc("/diagnostics", s.handleDiagnosticsPage)
	http.HandleFunc("/ws/diagnostics", s.handleDiagnosticsWS)

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
	data := TemplateData{Hosts: s.app.GetState()}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)

	// Add current timestamp for client-side health calculation if needed
	w.Header().Set("X-Server-Time", time.Now().Format(time.RFC3339))

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
	s.setCacheHeaders(w)
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

func (s *Server) handleAdvancedView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)
	if err := s.templates.ExecuteTemplate(w, "advanced-view.html", nil); err != nil {
		log.Printf("Error executing advanced-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
	}
}

// handleDiagnosticsPage serves a simple HTML page embedded via iframe that
// demonstrates realtime updates over a websocket connection.
func (s *Server) handleDiagnosticsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.setCacheHeaders(w)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <script src="https://cdn.tailwindcss.com"></script>
  <title>Diagnostics</title>
</head>
<body class="p-4 text-sm text-gray-800">
  <div class="mb-2">Diagnostics stream:</div>
  <pre id="out" class="bg-gray-100 rounded p-2 h-64 overflow-auto"></pre>
  <script>
	(function(){
	  const out = document.getElementById('out');
	  const proto = (location.protocol === 'https:') ? 'wss' : 'ws';
	  const url = proto + '://' + location.host + '/ws/diagnostics';
	  function log(line){ out.textContent += line + "\n"; out.scrollTop = out.scrollHeight; }
	  function connect(){
		const ws = new WebSocket(url);
		ws.onopen = () => log('[ws] connected');
		ws.onmessage = (e) => log(e.data);
		ws.onclose = () => { log('[ws] disconnected'); setTimeout(connect, 1500); };
		ws.onerror = () => { try { ws.close(); } catch(e){} };
	  }
	  connect();
	})();
  </script>
 </body></html>`)
}

// WebSocket endpoint with SSE fallback
func (s *Server) handleDiagnosticsWS(w http.ResponseWriter, r *http.Request) {
	// Use gorilla/websocket upgrader (provided by websocket_shim.go)
	if wsUpgrade, writer, ok := tryGorillaUpgrade(w, r); ok {
		defer writer.Close()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for t := range ticker.C {
			payload := map[string]interface{}{
				"time":        t.Format(time.RFC3339),
				"node_id":     s.nodeID,
				"hosts_count": len(s.app.GetState()),
			}
			b, _ := json.Marshal(payload)
			_ = wsUpgrade.WriteMessage(1 /* TextMessage */, b)
		}
		return
	}
	// Fallback to SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for t := range ticker.C {
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"time":"%s","node_id":"%s","hosts_count":%d}`, t.Format(time.RFC3339), s.nodeID, len(s.app.GetState())))
		flusher.Flush()
	}
}

// Option helpers
func WithNodeID(id string) func(*Server) {
	return func(s *Server) { s.nodeID = id }
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
