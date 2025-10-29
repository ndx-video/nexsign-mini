// Package anthias provides a background poller that periodically queries the
// local Anthias instance and broadcasts signed transactions to the network
// when state changes are detected.
package anthias

import (
	"encoding/json"
	"log"
	"time"

	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/tendermint"
	"nexsign.mini/nsm/internal/types"
)

// Poller periodically polls Anthias and broadcasts signed transactions when
// changes occur. It sends an add_host at startup if the host is not present
// and then update_status on changes.
type Poller struct {
	interval time.Duration
	client   *Client
	id       *identity.Identity
	tmClient *tendermint.BroadcastClient

	// cached state for change detection
	lastStatus string
}

// NewPoller constructs a Poller.
func NewPoller(interval time.Duration, client *Client, id *identity.Identity, tmRPC string) *Poller {
	if tmRPC == "" {
		tmRPC = "http://localhost:26657"
	}
	return &Poller{
		interval:   interval,
		client:     client,
		id:         id,
		tmClient:   tendermint.NewBroadcastClient(tmRPC),
		lastStatus: "",
	}
}

// Start begins the polling loop in a goroutine.
func (p *Poller) Start(getState func() map[string]types.Host) {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		// initial tick immediately
		p.pollOnce(getState)

		for range ticker.C {
			p.pollOnce(getState)
		}
	}()
}

func (p *Poller) pollOnce(getState func() map[string]types.Host) {
	host, err := p.client.GetMetadata()
	if err != nil {
		log.Printf("Anthias poller: metadata error: %v", err)
		return
	}

	// Ensure host.PublicKey is set to our node's identity
	host.PublicKey = p.id.PublicKeyHex()

	// If host not in state, broadcast add_host and return
	state := getState()
	if _, ok := state[host.PublicKey]; !ok {
		if err := p.broadcastAddHost(host); err != nil {
			log.Printf("Anthias poller: add_host broadcast failed: %v", err)
			return
		}
		// After add_host, also broadcast current status
		if err := p.broadcastStatus(host.AnthiasStatus); err != nil {
			log.Printf("Anthias poller: initial update_status failed: %v", err)
		}
		p.lastStatus = host.AnthiasStatus
		return
	}

	// If status changed, broadcast update_status
	if host.AnthiasStatus != p.lastStatus {
		if err := p.broadcastStatus(host.AnthiasStatus); err != nil {
			log.Printf("Anthias poller: update_status broadcast failed: %v", err)
			return
		}
		p.lastStatus = host.AnthiasStatus
	}
}

func (p *Poller) broadcastAddHost(h *types.Host) error {
	payload, err := json.Marshal(h)
	if err != nil {
		return err
	}
	tx := types.Transaction{
		Type:      types.TxAddHost,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	signed, err := tx.Sign(p.id)
	if err != nil {
		return err
	}
	// commit to ensure state contains signer before further updates
	_, err = p.tmClient.BroadcastSignedTransactionCommit(signed)
	return err
}

func (p *Poller) broadcastStatus(status string) error {
	payload, err := json.Marshal(types.UpdateStatusPayload{
		Status:   status,
		LastSeen: time.Now(),
	})
	if err != nil {
		return err
	}
	tx := types.Transaction{
		Type:      types.TxUpdateStatus,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	signed, err := tx.Sign(p.id)
	if err != nil {
		return err
	}
	// sync is fine for periodic updates
	_, err = p.tmClient.BroadcastSignedTransaction(signed)
	return err
}
