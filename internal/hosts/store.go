// Package hosts provides host list management backed by SQLite.
package hosts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexsign.mini/nsm/internal/types"

	_ "modernc.org/sqlite"
)

const (
	defaultDBFile        = "hosts.db"
	legacyJSONName       = "hosts.json"
	defaultBackupDirName = "backups"
	maxBusyTimeoutMs     = 5000
	defaultMaxBackups    = 20
)

var errNoBackups = errors.New("no host backups available")

// Store manages the host list and persistence to a SQLite database file.
type Store struct {
	mu        sync.RWMutex
	db        *sql.DB
	file      string
	backupDir string
	updates   chan struct{}
}

type backupInfo struct {
	path      string
	timestamp int64
}

// NewStore creates a new host store backed by SQLite.
func NewStore(filePath string) (*Store, error) {
	if filePath == "" {
		filePath = defaultDBFile
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}

	s := &Store{
		file:      absPath,
		backupDir: filepath.Join(filepath.Dir(absPath), defaultBackupDirName),
		updates:   make(chan struct{}, 1),
	}

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	if err := s.tryOpenOrRecover(); err != nil {
		return nil, err
	}

	if err := s.ensureSchema(); err != nil {
		_ = s.closeDB()
		return nil, err
	}

	if err := s.migrateLegacyJSON(); err != nil {
		_ = s.closeDB()
		return nil, err
	}

	return s, nil
}

// Updates returns a channel that receives a value whenever the host list changes.
func (s *Store) Updates() <-chan struct{} {
	return s.updates
}

func (s *Store) notify() {
	select {
	case s.updates <- struct{}{}:
	default:
	}
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeDB()
}

func (s *Store) tryOpenOrRecover() error {
	if err := s.openDB(); err != nil {
		if recErr := s.recoverDatabase(err); recErr != nil {
			return recErr
		}
	}
	return nil
}

func (s *Store) openDB() error {
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	connStr := fmt.Sprintf("file:%s", filepath.Clean(s.file))

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping sqlite: %w", err)
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", maxBusyTimeoutMs)); err != nil {
		db.Close()
		return fmt.Errorf("set busy timeout: %w", err)
	}

	s.db = db
	return nil
}

func (s *Store) recoverDatabase(openErr error) error {
	if err := s.restoreLatestBackup(); err != nil {
		if errors.Is(err, errNoBackups) {
			if cleanErr := s.resetDatabaseFiles(); cleanErr != nil {
				return fmt.Errorf("reset database after %v: %w", openErr, cleanErr)
			}
			if err := s.openDB(); err != nil {
				return fmt.Errorf("create fresh database after %v: %w", openErr, err)
			}
			return nil
		}
		return fmt.Errorf("restore database after %v: %w", openErr, err)
	}
	return nil
}

func (s *Store) closeDB() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *Store) resetDatabaseFiles() error {
	_ = s.closeDB()

	var firstErr error
	for _, path := range []string{s.file, s.file + "-wal", s.file + "-shm"} {
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("remove %s: %w", filepath.Base(path), err)
			}
		}
	}
	return firstErr
}

func (s *Store) removeSidecarFilesLocked() {
	for _, path := range []string{s.file + "-wal", s.file + "-shm"} {
		_ = os.Remove(path)
	}
}

func (s *Store) restoreLatestBackup() error {
	base := filepath.Base(s.file)
	prefix := strings.TrimSuffix(base, filepath.Ext(base))
	backups, err := s.listBackups(prefix, filepath.Ext(base))
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return errNoBackups
	}

	latest := backups[len(backups)-1]
	if err := s.resetDatabaseFiles(); err != nil {
		return err
	}
	if err := copyFile(latest.path, s.file); err != nil {
		return fmt.Errorf("copy backup %s: %w", filepath.Base(latest.path), err)
	}
	return s.openDB()
}

func (s *Store) listBackups(prefix, ext string) ([]backupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup directory: %w", err)
	}

	var backups []backupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, prefix+"-") {
			continue
		}
		if ext != "" && !strings.HasSuffix(name, ext) {
			continue
		}

		stem := name
		if ext != "" {
			stem = strings.TrimSuffix(stem, ext)
		}
		tsPart := strings.TrimPrefix(stem, prefix+"-")
		ts, parseErr := strconv.ParseInt(tsPart, 10, 64)
		if parseErr != nil {
			info, statErr := entry.Info()
			if statErr != nil {
				continue
			}
			ts = info.ModTime().Unix()
		}

		backups = append(backups, backupInfo{
			path:      filepath.Join(s.backupDir, name),
			timestamp: ts,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		if backups[i].timestamp == backups[j].timestamp {
			return backups[i].path < backups[j].path
		}
		return backups[i].timestamp < backups[j].timestamp
	})

	return backups, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (s *Store) ensureSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS hosts (
		ip_address TEXT PRIMARY KEY,
		nickname TEXT,
		vpn_ip_address TEXT,
		hostname TEXT,
		notes TEXT,
		status TEXT,
		status_vpn TEXT,
		nsm_status TEXT,
		nsm_status_vpn TEXT,
		nsm_version TEXT,
		nsm_version_vpn TEXT,
		anthias_version TEXT,
		anthias_version_vpn TEXT,
		anthias_status TEXT,
		anthias_status_vpn TEXT,
		cms_status TEXT,
		cms_status_vpn TEXT,
		asset_count INTEGER,
		asset_count_vpn INTEGER,
		dashboard_url TEXT,
		dashboard_url_vpn TEXT,
		last_checked TEXT,
		last_checked_vpn TEXT
	)`)
	if err != nil {
		return fmt.Errorf("create hosts table: %w", err)
	}

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}

	return nil
}

func (s *Store) migrateLegacyJSON() error {
	legacyPath := filepath.Join(filepath.Dir(s.file), legacyJSONName)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read legacy hosts.json: %w", err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "[]" || trimmed == "{}" {
		_ = os.Remove(legacyPath)
		return nil
	}

	var hosts []types.Host
	if err := json.Unmarshal(data, &hosts); err != nil {
		return fmt.Errorf("decode legacy hosts.json: %w", err)
	}

	if len(hosts) == 0 {
		_ = os.Remove(legacyPath)
		return nil
	}

	if err := s.ReplaceAll(hosts); err != nil {
		return fmt.Errorf("migrate legacy hosts: %w", err)
	}

	migratedName := legacyPath + ".migrated"
	if err := os.Rename(legacyPath, migratedName); err != nil {
		return fmt.Errorf("rename legacy hosts file: %w", err)
	}

	return nil
}

// GetAll returns all hosts ordered by IP address.
func (s *Store) GetAll() []types.Host {
	s.mu.RLock()
	rows, err := s.db.Query(`SELECT ip_address, nickname, vpn_ip_address, hostname, notes,
		status, status_vpn, nsm_status, nsm_status_vpn, nsm_version, nsm_version_vpn,
		anthias_version, anthias_version_vpn, anthias_status, anthias_status_vpn,
		cms_status, cms_status_vpn, asset_count, asset_count_vpn, dashboard_url,
		dashboard_url_vpn, last_checked, last_checked_vpn FROM hosts ORDER BY ip_address`)
	s.mu.RUnlock()
	if err != nil {
		return []types.Host{}
	}
	defer rows.Close()

	var hosts []types.Host
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			continue
		}
		hosts = append(hosts, host)
	}

	return hosts
}

// Add inserts a new host.
func (s *Store) Add(host types.Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO hosts (
		ip_address, nickname, vpn_ip_address, hostname, notes, status, status_vpn,
		nsm_status, nsm_status_vpn, nsm_version, nsm_version_vpn, anthias_version,
		anthias_version_vpn, anthias_status, anthias_status_vpn, cms_status,
		cms_status_vpn, asset_count, asset_count_vpn, dashboard_url,
		dashboard_url_vpn, last_checked, last_checked_vpn)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, hostToArgs(host)...)
	if err != nil {
		return fmt.Errorf("insert host: %w", err)
	}
	s.notify()
	return nil
}

// Update applies a mutation to an existing host identified by IP address.
func (s *Store) Update(ip string, updater func(*types.Host)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	host, err := s.getHostLocked(ip)
	if err != nil {
		return err
	}

	originalIP := host.IPAddress
	updater(&host)

	if host.IPAddress != originalIP {
		if _, err := s.db.Exec(`DELETE FROM hosts WHERE ip_address = ?`, originalIP); err != nil {
			return fmt.Errorf("delete original host: %w", err)
		}
	}

	_, err = s.db.Exec(`INSERT INTO hosts (
		ip_address, nickname, vpn_ip_address, hostname, notes, status, status_vpn,
		nsm_status, nsm_status_vpn, nsm_version, nsm_version_vpn, anthias_version,
		anthias_version_vpn, anthias_status, anthias_status_vpn, cms_status,
		cms_status_vpn, asset_count, asset_count_vpn, dashboard_url,
		dashboard_url_vpn, last_checked, last_checked_vpn)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip_address) DO UPDATE SET
			nickname = excluded.nickname,
			vpn_ip_address = excluded.vpn_ip_address,
			hostname = excluded.hostname,
			notes = excluded.notes,
			status = excluded.status,
			status_vpn = excluded.status_vpn,
			nsm_status = excluded.nsm_status,
			nsm_status_vpn = excluded.nsm_status_vpn,
			nsm_version = excluded.nsm_version,
			nsm_version_vpn = excluded.nsm_version_vpn,
			anthias_version = excluded.anthias_version,
			anthias_version_vpn = excluded.anthias_version_vpn,
			anthias_status = excluded.anthias_status,
			anthias_status_vpn = excluded.anthias_status_vpn,
			cms_status = excluded.cms_status,
			cms_status_vpn = excluded.cms_status_vpn,
			asset_count = excluded.asset_count,
			asset_count_vpn = excluded.asset_count_vpn,
			dashboard_url = excluded.dashboard_url,
			dashboard_url_vpn = excluded.dashboard_url_vpn,
			last_checked = excluded.last_checked,
			last_checked_vpn = excluded.last_checked_vpn`, hostToArgs(host)...)
	if err != nil {
		return fmt.Errorf("update host: %w", err)
	}
	s.notify()
	return nil
}

// Delete removes a host by IP address.
func (s *Store) Delete(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM hosts WHERE ip_address = ?`, ip)
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("host not found: %s", ip)
	}

	s.notify()
	return nil
}

// ReplaceAll atomically replaces the entire host list.
func (s *Store) ReplaceAll(hosts []types.Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin replace: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM hosts`); err != nil {
		tx.Rollback()
		return fmt.Errorf("truncate hosts: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO hosts (
		ip_address, nickname, vpn_ip_address, hostname, notes, status, status_vpn,
		nsm_status, nsm_status_vpn, nsm_version, nsm_version_vpn, anthias_version,
		anthias_version_vpn, anthias_status, anthias_status_vpn, cms_status,
		cms_status_vpn, asset_count, asset_count_vpn, dashboard_url,
		dashboard_url_vpn, last_checked, last_checked_vpn)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare replace insert: %w", err)
	}
	defer stmt.Close()

	for _, host := range hosts {
		if _, err := stmt.Exec(hostToArgs(host)...); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert host during replace: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace: %w", err)
	}

	s.notify()
	return nil
}

// BackupCurrent writes a snapshot of the database to a timestamped file and
// prunes old backups beyond maxBackups. Returns the backup path when created.
func (s *Store) BackupCurrent(maxBackups int) (string, error) {
	snapshot, err := s.ExportSnapshot()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	if maxBackups <= 0 {
		maxBackups = defaultMaxBackups
	}

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return "", fmt.Errorf("ensure backup directory: %w", err)
	}

	dir := s.backupDir
	base := filepath.Base(s.file)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext)
	if prefix == "" {
		prefix = base
	}

	timestamp := time.Now().Unix()
	var backupPath string
	for {
		name := fmt.Sprintf("%s-%d%s", prefix, timestamp, ext)
		backupPath = filepath.Join(dir, name)
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			break
		}
		timestamp++
	}

	if err := os.WriteFile(backupPath, snapshot, 0o600); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	pruneBackups(dir, prefix, ext, maxBackups)

	return backupPath, nil
}

// ExportSnapshot returns a consistent copy of the current database contents.
func (s *Store) ExportSnapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.file); errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}

	tempFile, err := os.CreateTemp(filepath.Dir(s.file), "hosts-export-*.db")
	if err != nil {
		return nil, fmt.Errorf("create temp export file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	escaped := strings.ReplaceAll(tempPath, "'", "''")
	if _, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("vacuum into temp file: %w", err)
	}

	data, err := os.ReadFile(tempPath)
	os.Remove(tempPath)
	if err != nil {
		return nil, fmt.Errorf("read export file: %w", err)
	}

	return data, nil
}

// ImportSnapshot replaces the current database contents with the provided
// SQLite database bytes. Returns the backup path if the existing database was
// moved aside.
func (s *Store) ImportSnapshot(data []byte, maxBackups int) (string, error) {
	if len(data) == 0 {
		return "", errors.New("snapshot data is empty")
	}

	if maxBackups <= 0 {
		maxBackups = defaultMaxBackups
	}

	dir := filepath.Dir(s.file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("prepare db directory: %w", err)
	}
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return "", fmt.Errorf("prepare backup directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, "hosts-import-*.db")
	if err != nil {
		return "", fmt.Errorf("create temp import file: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return "", fmt.Errorf("write temp import file: %w", err)
	}
	tempFile.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.closeDB()

	var backupPath string
	if _, err := os.Stat(s.file); err == nil {
		backupPath = uniqueBackupPath(s.backupDir, filepath.Base(s.file))
		if err := os.Rename(s.file, backupPath); err != nil {
			_ = s.openDB()
			os.Remove(tempPath)
			return "", fmt.Errorf("rename existing db: %w", err)
		}
		s.removeSidecarFilesLocked()
	}

	if err := os.Rename(tempPath, s.file); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, s.file)
		}
		os.Remove(tempPath)
		_ = s.openDB()
		return "", fmt.Errorf("activate imported db: %w", err)
	}

	if err := s.openDB(); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, s.file)
			_ = s.openDB()
		}
		return "", fmt.Errorf("reopen db after import: %w", err)
	}

	if err := s.ensureSchema(); err != nil {
		return backupPath, err
	}

	base := filepath.Base(s.file)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext)
	if prefix == "" {
		prefix = base
	}
	pruneBackups(s.backupDir, prefix, ext, maxBackups)

	return backupPath, nil
}

// GetByIP returns a specific host by IP address.
func (s *Store) GetByIP(ip string) (*types.Host, error) {
	s.mu.RLock()
	host, err := s.getHostLocked(ip)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	hostCopy := host
	return &hostCopy, nil
}

func (s *Store) getHostLocked(ip string) (types.Host, error) {
	row := s.db.QueryRow(`SELECT ip_address, nickname, vpn_ip_address, hostname, notes,
		status, status_vpn, nsm_status, nsm_status_vpn, nsm_version, nsm_version_vpn,
		anthias_version, anthias_version_vpn, anthias_status, anthias_status_vpn,
		cms_status, cms_status_vpn, asset_count, asset_count_vpn, dashboard_url,
		dashboard_url_vpn, last_checked, last_checked_vpn FROM hosts WHERE ip_address = ?`, ip)

	host, err := scanHost(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.Host{}, fmt.Errorf("host not found: %s", ip)
		}
		return types.Host{}, err
	}
	return host, nil
}

func hostToArgs(host types.Host) []any {
	return []any{
		host.IPAddress,
		host.Nickname,
		host.VPNIPAddress,
		host.Hostname,
		host.Notes,
		string(host.Status),
		string(host.StatusVPN),
		host.NSMStatus,
		host.NSMStatusVPN,
		host.NSMVersion,
		host.NSMVersionVPN,
		host.AnthiasVersion,
		host.AnthiasVersionVPN,
		host.AnthiasStatus,
		host.AnthiasStatusVPN,
		string(host.CMSStatus),
		string(host.CMSStatusVPN),
		host.AssetCount,
		host.AssetCountVPN,
		host.DashboardURL,
		host.DashboardURLVPN,
		formatTime(host.LastChecked),
		formatTime(host.LastCheckedVPN),
	}
}

func scanHost(scanner interface{ Scan(dest ...any) error }) (types.Host, error) {
	var (
		ip, nickname, vpnIP, hostname, notes sql.NullString
		status, statusVPN                    sql.NullString
		nsmStatus, nsmStatusVPN              sql.NullString
		nsmVersion, nsmVersionVPN            sql.NullString
		anthiasVersion, anthiasVersionVPN    sql.NullString
		anthiasStatus, anthiasStatusVPN      sql.NullString
		cmsStatus, cmsStatusVPN              sql.NullString
		assetCount, assetCountVPN            sql.NullInt64
		dashboard, dashboardVPN              sql.NullString
		lastChecked, lastCheckedVPN          sql.NullString
	)

	if err := scanner.Scan(
		&ip, &nickname, &vpnIP, &hostname, &notes,
		&status, &statusVPN, &nsmStatus, &nsmStatusVPN,
		&nsmVersion, &nsmVersionVPN, &anthiasVersion, &anthiasVersionVPN,
		&anthiasStatus, &anthiasStatusVPN, &cmsStatus, &cmsStatusVPN,
		&assetCount, &assetCountVPN, &dashboard, &dashboardVPN,
		&lastChecked, &lastCheckedVPN,
	); err != nil {
		return types.Host{}, err
	}

	host := types.Host{
		IPAddress:         ip.String,
		Nickname:          nickname.String,
		VPNIPAddress:      vpnIP.String,
		Hostname:          hostname.String,
		Notes:             notes.String,
		Status:            types.HostStatus(status.String),
		StatusVPN:         types.HostStatus(statusVPN.String),
		NSMStatus:         nsmStatus.String,
		NSMStatusVPN:      nsmStatusVPN.String,
		NSMVersion:        nsmVersion.String,
		NSMVersionVPN:     nsmVersionVPN.String,
		AnthiasVersion:    anthiasVersion.String,
		AnthiasVersionVPN: anthiasVersionVPN.String,
		AnthiasStatus:     anthiasStatus.String,
		AnthiasStatusVPN:  anthiasStatusVPN.String,
		CMSStatus:         types.AnthiasCMSStatus(cmsStatus.String),
		CMSStatusVPN:      types.AnthiasCMSStatus(cmsStatusVPN.String),
		AssetCount:        int(assetCount.Int64),
		AssetCountVPN:     int(assetCountVPN.Int64),
		DashboardURL:      dashboard.String,
		DashboardURLVPN:   dashboardVPN.String,
		LastChecked:       parseTime(lastChecked.String),
		LastCheckedVPN:    parseTime(lastCheckedVPN.String),
	}

	return host, nil
}

func formatTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts
	}
	return time.Time{}
}

func uniqueBackupPath(dir, base string) string {
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext)
	if prefix == "" {
		prefix = base
	}

	timestamp := time.Now().Unix()
	for {
		name := fmt.Sprintf("%s-%d%s", prefix, timestamp, ext)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path
		}
		timestamp++
	}
}

func pruneBackups(dir, prefix, ext string, maxBackups int) {
	if maxBackups <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []backupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, prefix+"-") {
			continue
		}
		if ext != "" && !strings.HasSuffix(name, ext) {
			continue
		}

		stem := name
		if ext != "" {
			stem = strings.TrimSuffix(stem, ext)
		}
		tsPart := strings.TrimPrefix(stem, prefix+"-")
		ts, err := strconv.ParseInt(tsPart, 10, 64)
		if err != nil {
			info, statErr := entry.Info()
			if statErr != nil {
				continue
			}
			ts = info.ModTime().Unix()
		}

		backups = append(backups, backupInfo{
			path:      filepath.Join(dir, name),
			timestamp: ts,
		})
	}

	if len(backups) <= maxBackups {
		return
	}

	sort.Slice(backups, func(i, j int) bool {
		if backups[i].timestamp == backups[j].timestamp {
			return backups[i].path < backups[j].path
		}
		return backups[i].timestamp < backups[j].timestamp
	})

	for i := 0; i < len(backups)-maxBackups; i++ {
		_ = os.Remove(backups[i].path)
	}
}
