// Package abci contains the ABCI application that connects our business logic
// to the Tendermint consensus engine. It implements transaction validation
// (CheckTx) and execution (DeliverTx) and maintains the in-memory ledger
// state of known hosts. This component is the critical bridge between the
// distributed consensus layer and the nsm domain logic: signatures are
// validated here and state transitions are applied here.
package abci

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"log"

	abci "github.com/tendermint/tendermint/abci/types"
	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/types"
)

const (
	CodeTypeOK            uint32 = 0
	CodeTypeEncodingError uint32 = 1
	CodeTypeAuthError     uint32 = 2
	CodeTypeInvalidTx     uint32 = 3
)

// ABCIApplication implements the ABCI interface.
type ABCIApplication struct {
	abci.BaseApplication
	state       map[string]types.Host
	nodePrivKey ed25519.PrivateKey
	localPubKey string
	// ActionHandler is an optional callback invoked when this node receives
	// an ActionTransaction targeting the local node. Tests can inject a
	// handler to observe or execute privileged actions instead of running
	// system commands directly in library code.
	ActionHandler func(action string, payload []byte) error
}

// NewABCIApplication creates a new ABCI application with a given initial state.
func NewABCIApplication(initialState map[string]types.Host, privKey ed25519.PrivateKey) *ABCIApplication {
	if initialState == nil {
		initialState = make(map[string]types.Host)
	}
	return &ABCIApplication{
		state:         initialState,
		nodePrivKey:   privKey,
		localPubKey:   identity.GetPublicKeyHex(privKey),
		ActionHandler: nil,
	}
}

// GetState returns a copy of the current application state.
func (app *ABCIApplication) GetState() map[string]types.Host {
	// Return a copy to prevent race conditions or unintended modifications
	stateCopy := make(map[string]types.Host)
	for k, v := range app.state {
		stateCopy[k] = v
	}
	return stateCopy
}

func (app *ABCIApplication) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{}
}

func (app *ABCIApplication) Query(req abci.RequestQuery) abci.ResponseQuery {
	return abci.ResponseQuery{Code: CodeTypeOK}
}

func (app *ABCIApplication) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	var signedTx types.SignedTransaction
	if err := json.Unmarshal(req.Tx, &signedTx); err != nil {
		return abci.ResponseCheckTx{Code: CodeTypeEncodingError, Log: "failed to decode signed tx"}
	}

	if !ed25519.Verify(signedTx.PublicKey, signedTx.Tx, signedTx.Signature) {
		return abci.ResponseCheckTx{Code: CodeTypeAuthError, Log: "invalid signature"}
	}

	var tx types.Transaction
	if err := json.Unmarshal(signedTx.Tx, &tx); err != nil {
		return abci.ResponseCheckTx{Code: CodeTypeEncodingError, Log: "failed to decode inner tx"}
	}

	if tx.Type != types.TxAddHost {
		signerPubKeyHex := hex.EncodeToString(signedTx.PublicKey)
		if _, ok := app.state[signerPubKeyHex]; !ok {
			return abci.ResponseCheckTx{Code: CodeTypeAuthError, Log: "signer public key not found in state"}
		}
	}

	return abci.ResponseCheckTx{Code: CodeTypeOK}
}

func (app *ABCIApplication) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	var signedTx types.SignedTransaction
	if err := json.Unmarshal(req.Tx, &signedTx); err != nil {
		return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "failed to decode signed tx"}
	}

	if !ed25519.Verify(signedTx.PublicKey, signedTx.Tx, signedTx.Signature) {
		return abci.ResponseDeliverTx{Code: CodeTypeAuthError, Log: "invalid signature"}
	}

	var tx types.Transaction
	if err := json.Unmarshal(signedTx.Tx, &tx); err != nil {
		return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "failed to decode inner tx"}
	}

	signerPubKeyHex := hex.EncodeToString(signedTx.PublicKey)

	switch tx.Type {
	case types.TxAddHost:
		var host types.Host
		if err := json.Unmarshal(tx.Payload, &host); err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "failed to decode AddHost payload"}
		}
		if host.PublicKey != signerPubKeyHex {
			return abci.ResponseDeliverTx{Code: CodeTypeAuthError, Log: "payload public key does not match signer"}
		}
		app.state[host.PublicKey] = host
		log.Printf("INFO: Added host to state: %s (%s)", host.Hostname, host.PublicKey)

	case types.TxUpdateStatus:
		var payload types.UpdateStatusPayload
		if err := json.Unmarshal(tx.Payload, &payload); err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "failed to decode UpdateStatus payload"}
		}
		if host, ok := app.state[signerPubKeyHex]; ok {
			host.AnthiasStatus = payload.Status
			host.LastSeen = payload.LastSeen
			app.state[signerPubKeyHex] = host
			log.Printf("INFO: Updated status for host %s to %s", host.Hostname, payload.Status)
		} else {
			return abci.ResponseDeliverTx{Code: CodeTypeInvalidTx, Log: "host not found for status update"}
		}

	case types.TxRestartHost:
		var payload types.RestartHostPayload
		if err := json.Unmarshal(tx.Payload, &payload); err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "failed to decode RestartHost payload"}
		}

		if payload.TargetPublicKey == app.localPubKey {
			log.Printf("INFO: Received restart command for this node.")
			// If an ActionHandler is provided (useful for tests), call it instead
			// of executing system-level restart commands here.
			if app.ActionHandler != nil {
				if err := app.ActionHandler("restart", tx.Payload); err != nil {
					return abci.ResponseDeliverTx{Code: CodeTypeInvalidTx, Log: "action handler failed"}
				}
			}
		} else {
			log.Printf("INFO: Received restart command for different node (%s), ignoring.", payload.TargetPublicKey)
		}

	default:
		return abci.ResponseDeliverTx{Code: CodeTypeInvalidTx, Log: "unknown transaction type"}
	}

	return abci.ResponseDeliverTx{Code: CodeTypeOK}
}
