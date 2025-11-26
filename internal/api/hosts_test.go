package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nexsign.mini/nsm/internal/types"
)

func TestHandleHosts(t *testing.T) {
	svc, store, cleanup := setupTest(t)
	defer cleanup()

	// Add some dummy hosts
	store.Add(types.Host{ID: "1", IPAddress: "192.168.1.1", Nickname: "Host 1"})
	store.Add(types.Host{ID: "2", IPAddress: "192.168.1.2", Nickname: "Host 2"})

	req := httptest.NewRequest(http.MethodGet, "/api/hosts", nil)
	w := httptest.NewRecorder()

	svc.HandleHosts(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var hosts []types.Host
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(hosts))
	}
}

func TestHandleAddHost(t *testing.T) {
	svc, store, cleanup := setupTest(t)
	defer cleanup()

	payload := map[string]string{
		"nickname":   "New Host",
		"ip_address": "192.168.1.100",
		"notes":      "Test note",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/add", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	svc.HandleAddHost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status No Content, got %v", resp.Status)
	}

	// Verify host was added
	host, err := store.GetByIP("192.168.1.100")
	if err != nil {
		t.Fatalf("Host not found in store: %v", err)
	}

	if host.Nickname != "New Host" {
		t.Errorf("Expected nickname 'New Host', got '%s'", host.Nickname)
	}
}

func TestHandleAddHost_InvalidInput(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	// Missing IP
	payload := map[string]string{
		"nickname": "Bad Host",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/add", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	svc.HandleAddHost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", resp.Status)
	}
}
