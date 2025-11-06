package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nexsign.mini/nsm/internal/types"
)

func TestBackupCurrentCreatesAndPrunesBackups(t *testing.T) {
	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts.db")

	store, err := NewStore(hostsFile)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.ReplaceAll([]types.Host{{IPAddress: "192.168.0.1"}}); err != nil {
		t.Fatalf("initial ReplaceAll: %v", err)
	}

	backupPath, err := store.BackupCurrent(10)
	if err != nil {
		t.Fatalf("BackupCurrent: %v", err)
	}
	if backupPath == "" {
		t.Fatalf("expected backup path, got empty string")
	}
	if filepath.Ext(backupPath) != ".db" {
		t.Fatalf("expected .db extension, got %q", filepath.Ext(backupPath))
	}
	if filepath.Dir(backupPath) != filepath.Join(dir, "backups") {
		t.Fatalf("expected backup in backups directory, got %q", filepath.Dir(backupPath))
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file should exist: %v", err)
	}
	if _, err := os.Stat(hostsFile); err != nil {
		t.Fatalf("hosts.db should still exist: %v", err)
	}

	// Calling backup again with no primary file should be a no-op.
	if err := os.Remove(hostsFile); err != nil {
		t.Fatalf("remove hosts.db: %v", err)
	}
	emptyPath, err := store.BackupCurrent(10)
	if err != nil {
		t.Fatalf("BackupCurrent without primary file: %v", err)
	}
	if emptyPath != "" {
		t.Fatalf("expected empty path when no hosts.db, got %q", emptyPath)
	}
	// Recreate store after removing underlying file so future operations succeed.
	if err := store.Close(); err != nil {
		t.Fatalf("close db after removal: %v", err)
	}
	if err := store.tryOpenOrRecover(); err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	if err := store.ensureSchema(); err != nil {
		t.Fatalf("ensure schema after reopen: %v", err)
	}

	if err := store.ReplaceAll([]types.Host{{IPAddress: "192.168.0.2"}}); err != nil {
		t.Fatalf("ReplaceAll after backup: %v", err)
	}

	// Generate more than maxBackups backups to ensure pruning occurs.
	for i := 0; i < 12; i++ {
		if _, err := store.BackupCurrent(10); err != nil {
			t.Fatalf("backup iteration %d: %v", i, err)
		}
		if err := store.ReplaceAll([]types.Host{{IPAddress: fmt.Sprintf("192.168.0.%d", i+3)}}); err != nil {
			t.Fatalf("ReplaceAll iteration %d: %v", i, err)
		}
	}

	backupDir := filepath.Join(dir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var backupCount int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "hosts-") && strings.HasSuffix(name, ".db") {
			backupCount++
		}
	}

	if backupCount > 10 {
		t.Fatalf("expected at most 10 backup files, found %d", backupCount)
	}
}

func TestNewStoreRecoversFromCorruptDBWithoutBackups(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hosts.db")

	if err := os.WriteFile(dbPath, []byte("this is not sqlite"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if hosts := store.GetAll(); len(hosts) != 0 {
		t.Fatalf("expected empty host list after recovery, got %d", len(hosts))
	}

	if err := store.Add(types.Host{IPAddress: "10.0.0.1", Nickname: "demo"}); err != nil {
		t.Fatalf("Add after recovery: %v", err)
	}
}

func TestNewStoreRestoresLatestBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hosts.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	host := types.Host{IPAddress: "10.0.0.5", Nickname: "primary"}
	if err := store.Add(host); err != nil {
		store.Close()
		t.Fatalf("Add: %v", err)
	}

	if _, err := store.BackupCurrent(20); err != nil {
		store.Close()
		t.Fatalf("BackupCurrent: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := os.WriteFile(dbPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	store, err = NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore after corruption: %v", err)
	}
	defer store.Close()

	restored, err := store.GetByIP(host.IPAddress)
	if err != nil {
		t.Fatalf("GetByIP after restore: %v", err)
	}
	if restored.Nickname != host.Nickname {
		t.Fatalf("expected nickname %q, got %q", host.Nickname, restored.Nickname)
	}
}

func TestStoreAddAndUpdate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hosts.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	original := types.Host{IPAddress: "10.0.0.10", Nickname: "initial"}
	if err := store.Add(original); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := store.Update(original.IPAddress, func(h *types.Host) {
		h.Nickname = "updated"
		h.Notes = "inline edit"
	}); err != nil {
		t.Fatalf("Update nickname: %v", err)
	}

	updated, err := store.GetByIP(original.IPAddress)
	if err != nil {
		t.Fatalf("GetByIP: %v", err)
	}
	if updated.Nickname != "updated" || updated.Notes != "inline edit" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	if err := store.Update(original.IPAddress, func(h *types.Host) {
		h.IPAddress = "10.0.0.11"
		h.Nickname = "moved"
	}); err != nil {
		t.Fatalf("Update IP: %v", err)
	}

	if _, err := store.GetByIP(original.IPAddress); err == nil {
		t.Fatalf("expected original IP lookup to fail")
	}

	moved, err := store.GetByIP("10.0.0.11")
	if err != nil {
		t.Fatalf("GetByIP new address: %v", err)
	}
	if moved.Nickname != "moved" {
		t.Fatalf("expected nickname 'moved', got %q", moved.Nickname)
	}
}
