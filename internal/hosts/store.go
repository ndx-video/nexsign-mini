// Package hosts provides host list management including loading, saving,
// and synchronizing the hosts.json file.
package hosts

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"nexsign.mini/nsm/internal/types"
)

const defaultHostsFile = "hosts.json"

// Store manages the host list and persistence to hosts.json
type Store struct {
	mu    sync.RWMutex
	hosts []types.Host
	file  string
}

// NewStore creates a new host store. If hosts.json doesn't exist, creates it.
func NewStore(filePath string) (*Store, error) {
	if filePath == "" {
		filePath = defaultHostsFile
	}

	s := &Store{
		hosts: []types.Host{},
		file:  filePath,
	}

	// Try to load existing hosts.json
	if err := s.Load(); err != nil {
		// If file doesn't exist, create empty file
		if os.IsNotExist(err) {
			if err := s.Save(); err != nil {
				return nil, fmt.Errorf("failed to create hosts.json: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load hosts.json: %w", err)
		}
	}

	return s, nil
}

// Load reads the hosts from the JSON file
func (s *Store) Load() error {
	data, err := os.ReadFile(s.file)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle empty file or empty JSON object/array
	if len(data) == 0 || string(data) == "{}" || string(data) == "[]" {
		s.hosts = []types.Host{}
		return nil
	}

	return json.Unmarshal(data, &s.hosts)
}

// Save writes the current host list to the JSON file
func (s *Store) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.hosts, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal hosts: %w", err)
	}

	return os.WriteFile(s.file, data, 0644)
}

// GetAll returns all hosts (thread-safe copy)
func (s *Store) GetAll() []types.Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	hosts := make([]types.Host, len(s.hosts))
	copy(hosts, s.hosts)
	return hosts
}

// Add adds a new host to the list and saves
func (s *Store) Add(host types.Host) error {
	s.mu.Lock()
	s.hosts = append(s.hosts, host)
	s.mu.Unlock()

	return s.Save()
}

// Update updates an existing host by IP address
func (s *Store) Update(ip string, updater func(*types.Host)) error {
	s.mu.Lock()
	found := false
	for i := range s.hosts {
		if s.hosts[i].IPAddress == ip {
			updater(&s.hosts[i])
			found = true
			break
		}
	}
	s.mu.Unlock()

	if !found {
		return fmt.Errorf("host not found: %s", ip)
	}

	return s.Save()
}

// Delete removes a host by IP address
func (s *Store) Delete(ip string) error {
	s.mu.Lock()
	found := false
	for i, host := range s.hosts {
		if host.IPAddress == ip {
			s.hosts = append(s.hosts[:i], s.hosts[i+1:]...)
			found = true
			break
		}
	}
	s.mu.Unlock()

	if !found {
		return fmt.Errorf("host not found: %s", ip)
	}

	return s.Save()
}

// ReplaceAll replaces the entire host list (used when receiving pushed updates)
func (s *Store) ReplaceAll(hosts []types.Host) error {
	s.mu.Lock()
	s.hosts = hosts
	s.mu.Unlock()

	return s.Save()
}

// GetByIP returns a specific host by IP address
func (s *Store) GetByIP(ip string) (*types.Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, host := range s.hosts {
		if host.IPAddress == ip {
			// Return a copy
			hostCopy := host
			return &hostCopy, nil
		}
	}

	return nil, fmt.Errorf("host not found: %s", ip)
}
