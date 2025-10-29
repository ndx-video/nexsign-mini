// Package tendermint - Transaction broadcasting via Tendermint RPC
//
// This file provides functions to broadcast transactions to Tendermint consensus.
// It uses the Tendermint RPC HTTP client to submit transactions.
package tendermint

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nexsign.mini/nsm/internal/types"
)

// BroadcastClient wraps a Tendermint RPC client for broadcasting transactions.
type BroadcastClient struct {
	rpcAddr string
	client  *http.Client
}

// NewBroadcastClient creates a new Tendermint RPC client for transaction broadcasting.
//
// Parameters:
//   - rpcAddr: Tendermint RPC address (e.g., "http://localhost:26657")
//
// Returns a client ready to broadcast transactions.
func NewBroadcastClient(rpcAddr string) *BroadcastClient {
	if rpcAddr == "" {
		rpcAddr = "http://localhost:26657"
	}

	return &BroadcastClient{
		rpcAddr: rpcAddr,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BroadcastTxSync broadcasts a transaction and returns immediately after CheckTx passes.
// This is fast but doesn't wait for the transaction to be committed.
//
// Use this for:
//   - Dashboard actions where user doesn't need to wait
//   - High-throughput scenarios
//   - When you'll poll for confirmation later
//
// Returns:
//   - txHash: The transaction hash (for later lookup)
//   - error: Any error during broadcast
func (bc *BroadcastClient) BroadcastTxSync(tx []byte) (string, error) {
	return bc.broadcast("broadcast_tx_sync", tx)
}

// BroadcastTxCommit broadcasts a transaction and waits for it to be committed to a block.
// This is slower but guarantees the transaction is finalized.
//
// Use this for:
//   - Critical operations (e.g., adding/removing validators)
//   - Anthias polling where you need confirmation
//   - When the caller needs proof of execution
//
// Returns:
//   - txHash: The transaction hash
//   - error: Any error during broadcast or commit
func (bc *BroadcastClient) BroadcastTxCommit(tx []byte) (string, error) {
	return bc.broadcast("broadcast_tx_commit", tx)
}

// BroadcastSignedTransaction is a high-level helper that broadcasts a SignedTransaction.
// It handles JSON marshaling and uses BroadcastTxSync for speed.
//
// Parameters:
//   - signedTx: The signed transaction to broadcast
//
// Returns:
//   - txHash: The transaction hash
//   - error: Any error during marshaling or broadcast
func (bc *BroadcastClient) BroadcastSignedTransaction(signedTx *types.SignedTransaction) (string, error) {
	// Marshal the signed transaction to JSON
	txBytes, err := json.Marshal(signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transaction: %w", err)
	}

	return bc.BroadcastTxSync(txBytes)
}

// BroadcastSignedTransactionCommit is like BroadcastSignedTransaction but waits for commit.
//
// Parameters:
//   - signedTx: The signed transaction to broadcast
//
// Returns:
//   - txHash: The transaction hash
//   - error: Any error during marshaling or broadcast
func (bc *BroadcastClient) BroadcastSignedTransactionCommit(signedTx *types.SignedTransaction) (string, error) {
	// Marshal the signed transaction to JSON
	txBytes, err := json.Marshal(signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transaction: %w", err)
	}

	return bc.BroadcastTxCommit(txBytes)
}

// broadcast is the internal helper that performs the actual RPC call.
func (bc *BroadcastClient) broadcast(method string, tx []byte) (string, error) {
	// Tendermint RPC expects tx as a base64-encoded string for JSON-RPC
	// See Tendermint v0.35 RPC docs: broadcast_tx_* requires base64 string
	txB64 := base64.StdEncoding.EncodeToString(tx)

	// Build JSON-RPC request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params": map[string]string{
			"tx": txB64,
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	// Send HTTP POST request
	resp, err := bc.client.Post(bc.rpcAddr, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send RPC request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read RPC response: %w", err)
	}

	// Parse response
	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Code uint32 `json:"code"`
			Data string `json:"data"`
			Log  string `json:"log"`
			Hash string `json:"hash"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    string `json:"data"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return "", fmt.Errorf("failed to parse RPC response: %w (body: %s)", err, string(respBytes))
	}

	// Check for RPC-level error
	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error %d: %s (%s)", rpcResp.Error.Code, rpcResp.Error.Message, rpcResp.Error.Data)
	}

	// Check for application-level error (non-zero code from CheckTx/DeliverTx)
	if rpcResp.Result.Code != 0 {
		return "", fmt.Errorf("transaction failed with code %d: %s", rpcResp.Result.Code, rpcResp.Result.Log)
	}

	// Return transaction hash
	return rpcResp.Result.Hash, nil
}

// QueryTx queries a transaction by hash.
//
// Parameters:
//   - txHash: The transaction hash (hex-encoded)
//
// Returns:
//   - Transaction details or error
func (bc *BroadcastClient) QueryTx(txHash string) (map[string]interface{}, error) {
	// Build JSON-RPC request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tx",
		"params": map[string]interface{}{
			"hash": txHash,
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	// Send HTTP POST request
	resp, err := bc.client.Post(bc.rpcAddr, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to send RPC request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	// Parse as generic map
	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	return result, nil
}
