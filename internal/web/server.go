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
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"nexsign.mini/nsm/internal/api"
	"nexsign.mini/nsm/internal/anthias"
	"nexsign.mini/nsm/internal/docs"
	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/logger"
	"nexsign.mini/nsm/internal/types"
)

// TemplateData holds the data to be passed to the HTML template.
type TemplateData struct {
	Hosts              []types.Host
	SelectedHost       *types.Host
	CurrentHostIP      string
	CurrentVersion     string
	BuildTime          string
	Interfaces         []string
	EnvVarSet          bool
	DuplicateHostnames map[string]bool
	EditLocks          map[string]string // hostID -> editorID
	DocList            []string
	DocContent         template.HTML
	CurrentDoc         string
}

// sseBroker manages SSE connections for broadcasting host updates
type sseBroker struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func newSSEBroker() *sseBroker {
	return &sseBroker{
		clients: make(map[chan []byte]struct{}),
	}
}

func (b *sseBroker) register(client chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[client] = struct{}{}
}

func (b *sseBroker) unregister(client chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, client)
	close(client)
}

func (b *sseBroker) broadcast(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for client := range b.clients {
		select {
		case client <- data:
		default:
			// Client is slow/blocked, skip
		}
	}
}

// Server is the web server for the dashboard and API.
type Server struct {
	store      *hosts.Store
	anthias    *anthias.Client
	port       int
	templates  *template.Template
	logger     *logger.Logger
	sseBroker  *sseBroker
	editLocks  map[string]string // hostID -> editorID
	editMu     sync.RWMutex
	apiService *api.Service
	docService *docs.Service
}

// NewServer creates a new web server.
func NewServer(store *hosts.Store, anthiasClient *anthias.Client, port int) (*Server, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	logger := logger.New(200) // Keep last 200 messages
	apiService := api.NewService(store, anthiasClient, logger)
	docService := docs.NewService("internal/docs")

	s := &Server{
		store:      store,
		anthias:    anthiasClient,
		port:       port,
		templates:  templates,
		logger:     logger,
		sseBroker:  newSSEBroker(),
		editLocks:  make(map[string]string),
		apiService: apiService,
		docService: docService,
	}
	
	// Log server initialization
	s.logger.Info("NSM server initialized")
	
	// Start listening for host updates and broadcast them via SSE
	go s.watchHostUpdates()
	
	return s, nil
}

// Logger returns the server's logger instance
func (s *Server) Logger() *logger.Logger {
	return s.logger
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
	mux.HandleFunc("/views/api", s.handleAPIView)
	mux.HandleFunc("/views/docs", s.handleDocsView)

	// API routes (delegated to apiService)
	mux.HandleFunc("/api/health", s.apiService.HandleHealth)
	mux.HandleFunc("/api/version", s.apiService.HandleVersion)
	mux.HandleFunc("/api/host/local", s.apiService.HandleHostLocal)
	mux.HandleFunc("/api/hosts", s.apiService.HandleHosts)
	mux.HandleFunc("/api/hosts/add", s.handleAddHost) // Kept local for pushToOnlinePeers
	mux.HandleFunc("/api/hosts/update", s.handleUpdateHost) // Kept local for pushToOnlinePeers
	mux.HandleFunc("/api/hosts/delete", s.apiService.HandleDeleteHost)
	mux.HandleFunc("/api/hosts/set-primary", s.apiService.HandleSetPrimaryHost)
	mux.HandleFunc("/api/hosts/check", s.apiService.HandleCheckHosts)
	mux.HandleFunc("/api/hosts/check-one", s.apiService.HandleCheckHost)
	mux.HandleFunc("/api/hosts/stream", s.handleHostsStream) // Kept in web for SSE logic
	mux.HandleFunc("/api/hosts/announce", s.apiService.HandleAnnounceHost)
	mux.HandleFunc("/api/hosts/lock", s.handleLockHost) // Kept local for editLocks
	mux.HandleFunc("/api/hosts/unlock", s.handleUnlockHost) // Kept local for editLocks
	mux.HandleFunc("/api/hosts/push", s.apiService.HandlePushHosts)
	mux.HandleFunc("/api/hosts/receive", s.apiService.HandleReceiveHosts)
	mux.HandleFunc("/api/hosts/reboot", s.apiService.HandleRebootHost)
	mux.HandleFunc("/api/hosts/upgrade", s.apiService.HandleUpgradeHost)
	mux.HandleFunc("/api/hosts/export/internal", s.apiService.HandleExportInternal)
	mux.HandleFunc("/api/hosts/export/download", s.apiService.HandleExportDownload)
	mux.HandleFunc("/api/hosts/import/internal", s.apiService.HandleImportInternal)
	mux.HandleFunc("/api/hosts/import/upload", s.apiService.HandleImportUpload)
	mux.HandleFunc("/api/backups/list", s.apiService.HandleBackupsList)
	mux.HandleFunc("/api/backups/restore", s.apiService.HandleRestoreBackup)
	mux.HandleFunc("/api/discovery/scan", s.apiService.HandleDiscoveryScan)
	mux.HandleFunc("/api/proxy/anthias", s.apiService.HandleProxyAnthias)
	
	// WebSocket routes
	mux.HandleFunc("/ws/diagnostics", s.handleDiagnosticsWS)
	mux.HandleFunc("/ws/status", s.handleStatusWS)

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
	err := s.templates.ExecuteTemplate(w, "layout.html", TemplateData{
		CurrentVersion: types.Version,
		BuildTime:      types.BuildTime,
	})
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

	s.editMu.RLock()
	editLocks := make(map[string]string)
	for k, v := range s.editLocks {
		editLocks[k] = v
	}
	s.editMu.RUnlock()

	data := TemplateData{
		Hosts:              allHosts,
		CurrentHostIP:      currentIP,
		CurrentVersion:     types.Version,
		Interfaces:         interfaces,
		EnvVarSet:          os.Getenv("NSM_HOST_IP") != "",
		DuplicateHostnames: duplicateHostnames,
		EditLocks:          editLocks,
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "home-view.html", data); err != nil {
		log.Printf("Error executing home-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
		return
	}

	s.setCacheHeaders(w)
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
	s.setCacheHeaders(w)

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "advanced-view.html", TemplateData{
		CurrentVersion: types.Version,
		BuildTime:      types.BuildTime,
	}); err != nil {
		log.Printf("Error executing advanced-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
		return
	}

	s.setCacheHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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

func (s *Server) handleAPIView(w http.ResponseWriter, r *http.Request) {
	s.setCacheHeaders(w)

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "api-view.html", TemplateData{
		CurrentVersion: types.Version,
		BuildTime:      types.BuildTime,
	}); err != nil {
		log.Printf("Error executing api-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
		return
	}

	s.setCacheHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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

func (s *Server) handleDocsView(w http.ResponseWriter, r *http.Request) {
	s.setCacheHeaders(w)

	docName := r.URL.Query().Get("doc")
	docList, _ := s.docService.ListDocs()

	var docContent string
	if docName != "" {
		content, err := s.docService.GetDoc(r.Context(), docName)
		if err == nil {
			docContent = content
		} else {
			s.logger.Error(fmt.Sprintf("Failed to load doc %s: %v", docName, err))
		}
	} else if len(docList) > 0 {
		// Default to first doc if none selected
		// docName = docList[0]
		// content, _ := s.docService.GetDoc(r.Context(), docName)
		// docContent = content
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "docs-view.html", TemplateData{
		CurrentVersion: types.Version,
		BuildTime:      types.BuildTime,
		DocList:        docList,
		DocContent:     template.HTML(docContent),
		CurrentDoc:     docName,
	}); err != nil {
		log.Printf("Error executing docs-view template: %s", err)
		http.Error(w, "Failed to render view", http.StatusInternalServerError)
		return
	}

	s.setCacheHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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
		s.logger.Error(fmt.Sprintf("Failed to add host %s: %v", ip, err))
		http.Error(w, "Failed to add host", http.StatusInternalServerError)
		return
	}

	s.logger.Info(fmt.Sprintf("API: Added new host: %s (%s)", ip, nickname))
	log.Printf("Added new host: %s (%s)", ip, nickname)

	// Auto-push to online peers
	go s.pushToOnlinePeers(host)

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
		s.logger.Error(fmt.Sprintf("Failed to update host %s: %v", updateReq.OldIP, err))
		http.Error(w, "Failed to update host", http.StatusInternalServerError)
		return
	}

	s.logger.Info(fmt.Sprintf("API: Updated host: %s -> %s", updateReq.OldIP, newIP))

	if updatedHost, getErr := s.store.GetByIP(newIP); getErr == nil {
		// Auto-push to online peers
		go s.pushToOnlinePeers(*updatedHost)
		
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

// watchHostUpdates listens for host changes and broadcasts them to all SSE clients
func (s *Server) watchHostUpdates() {
	updates := s.store.Updates()
	for range updates {
		// Render the updated host list
		data := s.renderHostListFragment()
		if data != nil {
			s.sseBroker.broadcast(data)
		}
	}
}

// bufferResponseWriter is a mock http.ResponseWriter that captures output to a buffer
// It implements http.ResponseWriter and http.Flusher for compatibility with the datastar SDK
type bufferResponseWriter struct {
	header http.Header
	buf    bytes.Buffer
	status int
}

func newBufferResponseWriter() *bufferResponseWriter {
	return &bufferResponseWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (b *bufferResponseWriter) Header() http.Header {
	return b.header
}

func (b *bufferResponseWriter) Write(data []byte) (int, error) {
	return b.buf.Write(data)
}

func (b *bufferResponseWriter) WriteHeader(statusCode int) {
	b.status = statusCode
}

func (b *bufferResponseWriter) Flush() {
	// No-op for buffer writer
}

// formatSSEEvent formats an HTML element as a datastar SSE event
// Uses datastar-merge-fragments format (client expects this format)
func formatSSEEvent(htmlContent string, selectorID string) ([]byte, error) {

	// Create a buffer-based response writer
	bufWriter := newBufferResponseWriter()

	// Use the SDK to send merge-fragments event
	bufWriter.Header().Set("Content-Type", "text/event-stream")
	bufWriter.Header().Set("Cache-Control", "no-cache")
	bufWriter.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(bufWriter, "event: datastar-merge-fragments\n")

	lines := strings.Split(htmlContent, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Fprintf(bufWriter, "data: fragments %s\n", line)
	}
	fmt.Fprintf(bufWriter, "\n")

	return bufWriter.buf.Bytes(), nil
}

// renderHostListFragment creates the SSE-formatted fragment for host list updates
func (s *Server) renderHostListFragment() []byte {
	// Get current host IP based on persistent ID
	currentIP := ""
	if localHost, err := s.anthias.GetMetadata(); err == nil {
		if storedHost, err := s.store.GetByID(localHost.ID); err == nil {
			currentIP = storedHost.IPAddress
		} else {
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

	s.editMu.RLock()
	editLocks := make(map[string]string)
	for k, v := range s.editLocks {
		editLocks[k] = v
	}
	s.editMu.RUnlock()

	templateData := TemplateData{
		Hosts:              allHosts,
		CurrentHostIP:      currentIP,
		CurrentVersion:     types.Version,
		DuplicateHostnames: duplicateHostnames,
		EditLocks:          editLocks,
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "host-rows-content", templateData); err != nil {
		log.Printf("Error rendering host-rows-content template: %v", err)
		return nil
	}

	// Wrap content in tbody with matching ID for datastar to target
	content := "<tbody id=\"host_table_body\" class=\"divide-y divide-desert-gray\">" + buf.String() + "</tbody>"
	
	// Use SDK to format the SSE event
	eventBytes, err := formatSSEEvent(content, "host_table_body")
	if err != nil {
		log.Printf("Error formatting SSE event: %v", err)
		return nil
	}
	
	return eventBytes
}

// handleHostsStream establishes an SSE connection and streams host list updates
func (s *Server) handleHostsStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create a channel for this client
	clientChan := make(chan []byte, 10)
	s.sseBroker.register(clientChan)
	defer s.sseBroker.unregister(clientChan)

	s.logger.Info("SSE client connected for host updates")
	defer s.logger.Info("SSE client disconnected")

	// Send initial state immediately
	initialData := s.renderHostListFragment()
	if initialData != nil {
		w.Write(initialData)
		flusher.Flush()
	}

	// Set up keep-alive ticker
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-clientChan:
			// Broadcast update received
			w.Write(data)
			flusher.Flush()
		case <-keepAlive.C:
			// Send keep-alive comment to prevent timeout
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

// handleAnnounceHost receives a single host announcement and upserts it
func (s *Server) handleAnnounceHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var host types.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate that we have at least an ID and IP
	if host.ID == "" || host.IPAddress == "" {
		http.Error(w, "Host ID and IP address are required", http.StatusBadRequest)
		return
	}

	if err := s.store.Upsert(host); err != nil {
		log.Printf("Failed to upsert announced host: %v", err)
		http.Error(w, "Failed to upsert host", http.StatusInternalServerError)
		return
	}

	s.logger.Info(fmt.Sprintf("Received host announcement: %s (ID: %s)", host.IPAddress, host.ID))
	w.WriteHeader(http.StatusNoContent)
}

// handleLockHost attempts to acquire an edit lock on a host
func (s *Server) handleLockHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		HostID   string `json:"host_id"`
		EditorID string `json:"editor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.HostID == "" || req.EditorID == "" {
		http.Error(w, "host_id and editor_id are required", http.StatusBadRequest)
		return
	}

	s.editMu.Lock()
	existingEditor, locked := s.editLocks[req.HostID]
	if locked && existingEditor != req.EditorID {
		s.editMu.Unlock()
		resp := map[string]interface{}{
			"success": false,
			"locked_by": existingEditor,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	s.editLocks[req.HostID] = req.EditorID
	s.editMu.Unlock()

	s.logger.Info(fmt.Sprintf("Lock acquired: host %s by %s", req.HostID, req.EditorID))
	
	// Broadcast lock state via SSE
	s.broadcastLockState()
	
	// Announce lock to peers
	go s.announceLockToPeers(req.HostID, req.EditorID, true)

	resp := map[string]interface{}{
		"success": true,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleUnlockHost releases an edit lock on a host
func (s *Server) handleUnlockHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		HostID   string `json:"host_id"`
		EditorID string `json:"editor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.HostID == "" {
		http.Error(w, "host_id is required", http.StatusBadRequest)
		return
	}

	s.editMu.Lock()
	existingEditor, locked := s.editLocks[req.HostID]
	// Only allow unlock if the editor matches or if no editor specified (force unlock)
	if locked && req.EditorID != "" && existingEditor != req.EditorID {
		s.editMu.Unlock()
		http.Error(w, "Cannot unlock: locked by different editor", http.StatusForbidden)
		return
	}
	delete(s.editLocks, req.HostID)
	s.editMu.Unlock()

	s.logger.Info(fmt.Sprintf("Lock released: host %s", req.HostID))
	
	// Broadcast lock state via SSE
	s.broadcastLockState()
	
	// Announce unlock to peers
	go s.announceLockToPeers(req.HostID, req.EditorID, false)

	w.WriteHeader(http.StatusNoContent)
}

// broadcastLockState sends current lock state to all SSE clients
func (s *Server) broadcastLockState() {
	s.editMu.RLock()
	locks := make(map[string]string)
	for k, v := range s.editLocks {
		locks[k] = v
	}
	s.editMu.RUnlock()

	data, err := json.Marshal(map[string]interface{}{
		"locks": locks,
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to marshal lock state: %v", err))
		return
	}

	msg := fmt.Sprintf("event: lock-state\ndata: %s\n\n", string(data))
	s.sseBroker.broadcast([]byte(msg))
}

// pushToOnlinePeers pushes a single host to all online peers on the same subnet
func (s *Server) pushToOnlinePeers(host types.Host) {
	allHosts := s.store.GetAll()
	localSubnet := getSubnet(host.IPAddress)

	if localSubnet == "" {
		s.logger.Warning(fmt.Sprintf("Cannot determine subnet for %s, skipping peer push", host.IPAddress))
		return
	}

	peerCount := 0
	for _, peer := range allHosts {
		// Skip self
		if peer.ID == host.ID {
			continue
		}

		// Only push to healthy/online hosts
		if peer.Status != types.StatusHealthy {
			continue
		}

		// Only push to hosts on the same subnet
		peerSubnet := getSubnet(peer.IPAddress)
		if peerSubnet != localSubnet {
			continue
		}

		peerCount++
		go func(targetIP, targetID string) {
			body, err := json.Marshal(host)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Failed to marshal host for peer push: %v", err))
				return
			}

			url := fmt.Sprintf("http://%s:8080/api/hosts/announce", targetIP)
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				s.logger.Warning(fmt.Sprintf("Failed to announce to peer %s: %v", targetIP, err))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				s.logger.Info(fmt.Sprintf("Announced host %s to peer %s", host.IPAddress, targetIP))
			} else {
				s.logger.Warning(fmt.Sprintf("Peer %s returned status %d for announcement", targetIP, resp.StatusCode))
			}
		}(peer.IPAddress, peer.ID)
	}

	if peerCount > 0 {
		s.logger.Info(fmt.Sprintf("Announcing host %s to %d online peers on subnet %s.0/24", host.IPAddress, peerCount, localSubnet))
	} else {
		s.logger.Info(fmt.Sprintf("No online peers on subnet %s.0/24 to announce to", localSubnet))
	}
}

// getSubnet extracts the first three octets of an IP address (e.g., "192.168.10" from "192.168.10.5")
func getSubnet(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	return strings.Join(parts[:3], ".")
}

// announceLockToPeers announces a lock/unlock operation to online peers on the same subnet
func (s *Server) announceLockToPeers(hostID, editorID string, isLock bool) {
	allHosts := s.store.GetAll()
	
	// Get the host being locked to determine its subnet
	var targetHost *types.Host
	for _, h := range allHosts {
		if h.ID == hostID {
			targetHost = &h
			break
		}
	}
	
	if targetHost == nil {
		s.logger.Warning(fmt.Sprintf("Cannot find host %s for lock announcement", hostID))
		return
	}
	
	localSubnet := getSubnet(targetHost.IPAddress)
	if localSubnet == "" {
		s.logger.Warning(fmt.Sprintf("Cannot determine subnet for %s, skipping lock announcement", targetHost.IPAddress))
		return
	}

	endpoint := "/api/hosts/lock"
	if !isLock {
		endpoint = "/api/hosts/unlock"
	}

	peerCount := 0
	for _, peer := range allHosts {
		// Skip self
		if peer.ID == targetHost.ID {
			continue
		}

		// Only announce to healthy/online hosts
		if peer.Status != types.StatusHealthy {
			continue
		}

		// Only announce to hosts on the same subnet
		peerSubnet := getSubnet(peer.IPAddress)
		if peerSubnet != localSubnet {
			continue
		}

		peerCount++
		go func(targetIP string) {
			reqBody := map[string]string{
				"host_id":   hostID,
				"editor_id": editorID,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Failed to marshal lock request: %v", err))
				return
			}

			url := fmt.Sprintf("http://%s:8080%s", targetIP, endpoint)
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				// Silently ignore peer announcement failures
				return
			}
			defer resp.Body.Close()
		}(peer.IPAddress)
	}

	if peerCount > 0 {
		action := "locked"
		if !isLock {
			action = "unlocked"
		}
		s.logger.Info(fmt.Sprintf("Announcing %s state for host %s to %d peer(s)", action, hostID, peerCount))
	}
}



// handleDiagnosticsWS handles WebSocket connections for diagnostics data
func (s *Server) handleDiagnosticsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Get current time
			currentTime := time.Now().Format("2006-01-02 15:04:05")

			// Get node ID
			var nodeID string
			if meta, err := s.anthias.GetMetadata(); err == nil {
				nodeID = meta.ID
			}

			// Get host count
			hosts := s.store.GetAll()
			hostCount := len(hosts)

			// Get most recent backup timestamp
			lastBackup := "none"
			if backupDir := "backups"; true {
				if entries, err := os.ReadDir(backupDir); err == nil {
					var latestTime time.Time
					for _, entry := range entries {
						if entry.IsDir() || !strings.HasPrefix(entry.Name(), "hosts.") {
							continue
						}
						if info, err := entry.Info(); err == nil {
							if info.ModTime().After(latestTime) {
								latestTime = info.ModTime()
							}
						}
					}
					if !latestTime.IsZero() {
						lastBackup = latestTime.Format("2006-01-02 15:04:05")
					}
				}
			}

			msg := map[string]interface{}{
				"time":        currentTime,
				"node_id":     nodeID,
				"hosts_count": hostCount,
				"last_backup": lastBackup,
			}

			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

// handleStatusWS handles WebSocket connections for status bar messages and console logs
func (s *Server) handleStatusWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Send initial history (last 50 logs)
	initialLogs := s.logger.GetRecent(50)
	// Reverse to send oldest first if needed, but client usually appends.
	// GetRecent returns newest first.
	for i := len(initialLogs) - 1; i >= 0; i-- {
		if err := conn.WriteJSON(initialLogs[i]); err != nil {
			return
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond) // Poll faster for responsiveness
	defer ticker.Stop()

	var lastLogTime time.Time
	if len(initialLogs) > 0 {
		lastLogTime = initialLogs[0].Timestamp
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Get recent logs
			recent := s.logger.GetRecent(20) // Check last 20
			
			// Filter for new logs
			var newLogs []logger.Message
			for _, msg := range recent {
				if msg.Timestamp.After(lastLogTime) {
					newLogs = append(newLogs, msg)
				}
			}

			// Send new logs (oldest first)
			for i := len(newLogs) - 1; i >= 0; i-- {
				msg := newLogs[i]
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
				// Update tracker
				if msg.Timestamp.After(lastLogTime) {
					lastLogTime = msg.Timestamp
				}
			}
		}
	}
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
