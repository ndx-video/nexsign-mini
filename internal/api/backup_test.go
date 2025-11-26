package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleBackupsList(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	// Create a dummy backup file
	backupDir := "backups"
	os.MkdirAll(backupDir, 0755)
	defer os.RemoveAll(backupDir)

	os.WriteFile(filepath.Join(backupDir, "hosts-test.db"), []byte("dummy data"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/api/backups/list", nil)
	w := httptest.NewRecorder()

	svc.HandleBackupsList(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var backups []struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&backups); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	found := false
	for _, b := range backups {
		if b.Filename == "hosts-test.db" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find hosts-test.db in backup list")
	}
}

func TestHandleExportInternal(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/hosts/export/internal", nil)
	w := httptest.NewRecorder()

	svc.HandleExportInternal(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	// The response is JSON with path, not the file content itself
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}
}

func TestHandleExportDownload(t *testing.T) {
	svc, _, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/hosts/export/download", nil)
	w := httptest.NewRecorder()

	svc.HandleExportDownload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}
}
