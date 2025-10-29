// Package config centralizes runtime configuration for nsm. It loads a
// JSON configuration file and exposes a process-wide configuration with
// sensible defaults. Tests and development builds will use defaults when the
// file is not present. Production operators should place a JSON file at
// /etc/nsm/config.json or specify a different path via the CONFIG_FILE env var.
package config

import (
	"encoding/json"
	"os"
)

// Config holds configurable options for the nsm service.
type Config struct {
	KeyFile             string `json:"key_file"`
	HostDataFile        string `json:"host_data_file"`
	Port                int    `json:"port"`
	MDNSServiceName     string `json:"mdns_service_name"`
	TendermintPeersFile string `json:"tendermint_peers_file"`
	LogFile             string `json:"log_file"`
	RestartCommand      string `json:"restart_command"`
	EnableActions       bool   `json:"enable_actions"`
}

var cfg *Config

// LoadConfig reads a JSON file at path. If the file does not exist or
// cannot be parsed, LoadConfig returns defaults (and no error) so that the
// application can run in development with minimal friction.
func LoadConfig(path string) (*Config, error) {
	// sensible defaults
	def := &Config{
		KeyFile:             "nsm_key.pem",
		HostDataFile:        "",
		Port:                8080,
		MDNSServiceName:     "_nsm._tcp",
		TendermintPeersFile: "tendermint_persistent_peers",
		LogFile:             "nsm.log",
		RestartCommand:      "systemctl restart nsm",
		EnableActions:       false,
	}

	// if no file path provided, return defaults
	if path == "" {
		cfg = def
		return cfg, nil
	}

	// read file
	b, err := os.ReadFile(path)
	if err != nil {
		// file missing or unreadable -> use defaults
		cfg = def
		return cfg, nil
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		// parse error -> use defaults
		cfg = def
		return cfg, nil
	}

	// merge defaults for any zero-value fields
	if c.KeyFile == "" {
		c.KeyFile = def.KeyFile
	}
	if c.HostDataFile == "" {
		c.HostDataFile = def.HostDataFile
	}
	if c.Port == 0 {
		c.Port = def.Port
	}
	if c.MDNSServiceName == "" {
		c.MDNSServiceName = def.MDNSServiceName
	}
	if c.TendermintPeersFile == "" {
		c.TendermintPeersFile = def.TendermintPeersFile
	}
	if c.LogFile == "" {
		c.LogFile = def.LogFile
	}
	if c.RestartCommand == "" {
		c.RestartCommand = def.RestartCommand
	}

	cfg = &c
	return cfg, nil
}

// Get returns the loaded configuration. If LoadConfig hasn't been called
// yet, it returns defaults.
func Get() *Config {
	if cfg == nil {
		// initialize with defaults
		LoadConfig("")
	}
	return cfg
}
