// Package tendermint provides socket-based ABCI server for Tendermint integration.
//
// This package implements Option B (socket-based) architecture where:
// - nsm runs an ABCI server listening on a Unix socket
// - Tendermint runs as a separate process and connects via the socket
// - Communication happens over the ABCI protocol
//
// This is the standard, battle-tested approach used by most Tendermint applications.
package tendermint

import (
	"fmt"
	"os"

	abciserver "github.com/tendermint/tendermint/abci/server"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
)

// Config holds configuration for the ABCI server and Tendermint connection.
type Config struct {
	// TendermintHome is the directory for Tendermint data and config
	TendermintHome string

	// SocketAddress is the Unix socket address (e.g., "unix://nsm.sock")
	SocketAddress string
}

// ABCIServer wraps an ABCI server for socket-based Tendermint connection.
type ABCIServer struct {
	server  service.Service
	abciApp abci.Application
	config  *Config
	socket  string
}

// NewABCIServer creates a new socket-based ABCI server.
//
// Parameters:
// - app: the ABCI application implementing CheckTx, DeliverTx, etc.
// - config: server configuration including socket path
//
// The server is created but not started. Call Start() to begin listening.
func NewABCIServer(app abci.Application, config *Config) (*ABCIServer, error) {
	if app == nil {
		return nil, fmt.Errorf("ABCI application cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.SocketAddress == "" {
		return nil, fmt.Errorf("socket address cannot be empty")
	}

	// Create socket server using Tendermint's ABCI server package
	server := abciserver.NewSocketServer(config.SocketAddress, app)

	return &ABCIServer{
		server:  server,
		abciApp: app,
		config:  config,
		socket:  config.SocketAddress,
	}, nil
}

// Start begins listening on the Unix socket for Tendermint connections.
func (s *ABCIServer) Start() error {
	if err := s.server.Start(); err != nil {
		return fmt.Errorf("failed to start ABCI server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the ABCI server and cleans up the socket file.
func (s *ABCIServer) Stop() error {
	if s.server.IsRunning() {
		if err := s.server.Stop(); err != nil {
			return fmt.Errorf("failed to stop ABCI server: %w", err)
		}
	}

	// Clean up socket file if it exists
	// Extract path from "unix://path"
	socketPath := s.socket
	if len(socketPath) > 7 && socketPath[:7] == "unix://" {
		socketPath = socketPath[7:]
	}
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	return nil
}

// IsRunning returns true if the ABCI server is currently running.
func (s *ABCIServer) IsRunning() bool {
	return s.server.IsRunning()
}

// SocketPath returns the socket address the server is listening on.
func (s *ABCIServer) SocketPath() string {
	return s.socket
}
