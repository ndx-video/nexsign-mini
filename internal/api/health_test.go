package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexsign.mini/nsm/internal/types"
)

func TestHandleHealth(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	svc.HandleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}

func TestHandleCheckHosts(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/check", nil)
	w := httptest.NewRecorder()

	svc.HandleCheckHosts(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status No Content, got %v", resp.Status)
	}
	
	// Wait a bit for the goroutine to run (though we can't easily verify the side effect without more complex mocking)
	time.Sleep(10 * time.Millisecond)
}

func TestHandleCheckHost(t *testing.T) {
	svc, store, cleanup := setupTest(t)
	defer cleanup()

	// Add a host to check
	store.Add(types.Host{ID: "1", IPAddress: "127.0.0.1", Nickname: "Localhost"})

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/check-one?ip=127.0.0.1", nil)
	w := httptest.NewRecorder()

	svc.HandleCheckHost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status No Content, got %v", resp.Status)
	}
}

func TestHandleCheckHost_NotFound(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/check-one?ip=1.2.3.4", nil)
	w := httptest.NewRecorder()

	svc.HandleCheckHost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status NotFound, got %v", resp.Status)
	}
}
