package api

import (
	"encoding/json"
	"net/http"

	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/logger"
	"nexsign.mini/nsm/internal/types"
)

// AnthiasProvider defines the interface for interacting with Anthias
type AnthiasProvider interface {
	GetMetadata() (*types.Host, error)
}

// Service handles API requests
type Service struct {
	store   *hosts.Store
	anthias AnthiasProvider
	logger  *logger.Logger
}

// NewService creates a new API service
func NewService(store *hosts.Store, anthias AnthiasProvider, logger *logger.Logger) *Service {
	return &Service{
		store:   store,
		anthias: anthias,
		logger:  logger,
	}
}

// writeJSON writes a JSON response
func (s *Service) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response
func (s *Service) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}
