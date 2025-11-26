package api

import (
	"os"
	"testing"

	"nexsign.mini/nsm/internal/hosts"
	"nexsign.mini/nsm/internal/logger"
	"nexsign.mini/nsm/internal/types"
)

// MockAnthias implements AnthiasProvider for testing
type MockAnthias struct {
	Metadata *types.Host
	Err      error
}

func (m *MockAnthias) GetMetadata() (*types.Host, error) {
	return m.Metadata, m.Err
}

// setupTest creates a temporary store and service for testing
func setupTest(t *testing.T) (*Service, *hosts.Store, func()) {
	// Create a temporary file for the database
	tmpDB, err := os.CreateTemp("", "hosts-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	tmpDB.Close() // Close it, NewStore will open it

	store, err := hosts.NewStore(tmpDB.Name())
	if err != nil {
		os.Remove(tmpDB.Name())
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create a mock Anthias client
	mockAnthias := &MockAnthias{
		Metadata: &types.Host{
			ID:        "test-id",
			Hostname:  "test-host",
			IPAddress: "127.0.0.1",
		},
	}

	// Create a logger that discards output
	// For now, we'll just use the standard logger but we could mock it if needed
	l := logger.New(100)

	svc := NewService(store, mockAnthias, l)

	cleanup := func() {
		os.Remove(tmpDB.Name())
	}

	return svc, store, cleanup
}
