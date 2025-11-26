package api

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"nexsign.mini/nsm/internal/types"
)

// @Title: Get Health
// @Route: GET /api/health
// @Description: Returns server health status
// @Response: {"status": "ok"}
func (s *Service) HandleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// @Title: Get Version
// @Route: GET /api/version
// @Description: Returns NSM version and node ID
// @Response: {"version": "...", "status": "ok", "id": "..."}
func (s *Service) HandleVersion(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	
	response := map[string]string{
		"version":  types.Version,
		"status":   "ok",
		"hostname": hostname,
		"go_ver":   runtime.Version(),
		"os_arch":  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if meta, err := s.anthias.GetMetadata(); err == nil {
		response["id"] = meta.ID
	}

	s.writeJSON(w, http.StatusOK, response)
}

// @Title: Get Local Host
// @Route: GET /api/host/local
// @Description: Returns metadata for this specific host
// @Response: Host object with full details
func (s *Service) HandleHostLocal(w http.ResponseWriter, r *http.Request) {
	meta, err := s.anthias.GetMetadata()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get local metadata")
		return
	}

	// Try to get full details from store if available
	if stored, err := s.store.GetByID(meta.ID); err == nil {
		s.writeJSON(w, http.StatusOK, stored)
		return
	}

	// Fallback to basic metadata
	host := types.Host{
		ID:        meta.ID,
		Nickname:  "Local Host",
		IPAddress: os.Getenv("NSM_HOST_IP"),
		Status:    types.StatusHealthy,
		LastChecked: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, host)
}
