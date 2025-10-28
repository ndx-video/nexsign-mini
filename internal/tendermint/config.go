// Package tendermint - Tendermint configuration helpers
//
// This file provides helper functions for initializing and configuring
// a Tendermint node that connects to our ABCI server via socket.
package tendermint

import (
"fmt"
"os"
"os/exec"
"path/filepath"
)

// InitTendermint initializes a Tendermint home directory with config and genesis files.
// This should be run once before starting Tendermint for the first time.
//
// It runs: `tendermint init --home <tmHome>`
func InitTendermint(tmHome string) error {
if tmHome == "" {
tmHome = filepath.Join(os.Getenv("HOME"), ".tendermint")
}

// Check if already initialized
configFile := filepath.Join(tmHome, "config", "config.toml")
if _, err := os.Stat(configFile); err == nil {
return nil // Already initialized
}

cmd := exec.Command("tendermint", "init", "--home", tmHome)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr

if err := cmd.Run(); err != nil {
return fmt.Errorf("failed to initialize Tendermint: %w", err)
}

return nil
}

// GetTendermintCommand returns the command to start Tendermint node.
// The returned command can be run with cmd.Start() or exec.Command().Run().
//
// Example:
//   cmd := tendermint.GetTendermintCommand("/path/to/.tendermint", "unix://nsm.sock")
//   cmd.Start()
func GetTendermintCommand(tmHome, socketAddr string) *exec.Cmd {
if tmHome == "" {
tmHome = filepath.Join(os.Getenv("HOME"), ".tendermint")
}

if socketAddr == "" {
socketAddr = "unix://nsm.sock"
}

cmd := exec.Command("tendermint", "node",
"--home", tmHome,
"--proxy_app", socketAddr,
)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr

return cmd
}

// TendermintHome returns the default Tendermint home directory.
func TendermintHome() string {
if home := os.Getenv("TMHOME"); home != "" {
return home
}
return filepath.Join(os.Getenv("HOME"), ".tendermint")
}
