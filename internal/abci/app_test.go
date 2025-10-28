package abci

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	tmabci "github.com/tendermint/tendermint/abci/types"

	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/types"
)

// Test that a correctly signed AddHost transaction passes CheckTx and
// is applied by DeliverTx.
func TestAddHostCheckAndDeliver(t *testing.T) {
	// create identity
	idPath := "test_abci_key.pem"
	defer os.Remove(idPath)
	id, err := identity.LoadOrCreateIdentity(idPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity error: %v", err)
	}

	// build host payload
	host := types.Host{
		Hostname:       "test-host",
		IPAddress:      "192.0.2.5",
		AnthiasVersion: "v0.0-test",
		AnthiasStatus:  "Online",
		DashboardURL:   "http://192.0.2.5:8080",
		PublicKey:      id.PublicKeyHex(),
	}
	payload, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("marshal host: %v", err)
	}

	tx := &types.Transaction{
		Type:      types.TxAddHost,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	stx, err := tx.Sign(id)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}

	txBytes, err := json.Marshal(stx)
	if err != nil {
		t.Fatalf("marshal signed tx: %v", err)
	}

	app := NewABCIApplication(nil, id.PrivateKey())

	// CheckTx should accept the transaction
	resp := app.CheckTx(tmabci.RequestCheckTx{Tx: txBytes})
	if resp.Code != CodeTypeOK {
		t.Fatalf("CheckTx failed: code=%d log=%s", resp.Code, resp.Log)
	}

	// DeliverTx should apply the state
	dresp := app.DeliverTx(tmabci.RequestDeliverTx{Tx: txBytes})
	if dresp.Code != CodeTypeOK {
		t.Fatalf("DeliverTx failed: code=%d log=%s", dresp.Code, dresp.Log)
	}

	// Verify state contains host
	state := app.GetState()
	if _, ok := state[host.PublicKey]; !ok {
		t.Fatalf("host not found in state after DeliverTx")
	}
}

// Test that a tampered or mismatched signature is rejected by CheckTx.
func TestCheckTxRejectsInvalidSignature(t *testing.T) {
	// create two identities
	aPath := "test_abci_key_a.pem"
	bPath := "test_abci_key_b.pem"
	defer os.Remove(aPath)
	defer os.Remove(bPath)
	ida, err := identity.LoadOrCreateIdentity(aPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity a: %v", err)
	}
	idb, err := identity.LoadOrCreateIdentity(bPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity b: %v", err)
	}

	// create a transaction that claims to be from A but signed by B
	host := types.Host{PublicKey: ida.PublicKeyHex()}
	payload, _ := json.Marshal(host)
	tx := &types.Transaction{Type: types.TxAddHost, Timestamp: time.Now(), Payload: payload}
	stx, err := tx.Sign(idb) // signed by B
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	// set PublicKey field to A's public key but leave signature from B -> invalid
	stx.PublicKey = []byte(ida.PublicKey())
	txBytes, _ := json.Marshal(stx)

	app := NewABCIApplication(nil, ida.PrivateKey())

	resp := app.CheckTx(tmabci.RequestCheckTx{Tx: txBytes})
	if resp.Code == CodeTypeOK {
		t.Fatalf("CheckTx unexpectedly accepted invalid signature")
	}
}

// Test that UpdateStatus transactions update the host AnthiasStatus in state.
func TestUpdateStatusDeliver(t *testing.T) {
	// create identity
	idPath := "test_abci_key_upd.pem"
	defer os.Remove(idPath)
	id, err := identity.LoadOrCreateIdentity(idPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity error: %v", err)
	}

	// add initial host via AddHost tx
	host := types.Host{
		Hostname:      "update-host",
		IPAddress:     "192.0.2.6",
		AnthiasStatus: "Offline",
		PublicKey:     id.PublicKeyHex(),
	}
	payload, _ := json.Marshal(host)
	addTx := &types.Transaction{Type: types.TxAddHost, Timestamp: time.Now(), Payload: payload}
	stxAdd, err := addTx.Sign(id)
	if err != nil {
		t.Fatalf("sign add tx: %v", err)
	}
	addBytes, _ := json.Marshal(stxAdd)

	app := NewABCIApplication(nil, id.PrivateKey())
	dresp := app.DeliverTx(tmabci.RequestDeliverTx{Tx: addBytes})
	if dresp.Code != CodeTypeOK {
		t.Fatalf("DeliverTx add failed: code=%d log=%s", dresp.Code, dresp.Log)
	}

	// now send UpdateStatus
	upd := types.UpdateStatusPayload{Status: "Online", LastSeen: time.Now()}
	updPayload, _ := json.Marshal(upd)
	tx := &types.Transaction{Type: types.TxUpdateStatus, Timestamp: time.Now(), Payload: updPayload}
	stx, err := tx.Sign(id)
	if err != nil {
		t.Fatalf("sign update tx: %v", err)
	}
	txBytes, _ := json.Marshal(stx)

	resp := app.DeliverTx(tmabci.RequestDeliverTx{Tx: txBytes})
	if resp.Code != CodeTypeOK {
		t.Fatalf("DeliverTx update failed: code=%d log=%s", resp.Code, resp.Log)
	}

	state := app.GetState()
	h, ok := state[id.PublicKeyHex()]
	if !ok {
		t.Fatalf("host not found after update")
	}
	if h.AnthiasStatus != "Online" {
		t.Fatalf("expected AnthiasStatus Online, got %s", h.AnthiasStatus)
	}
}

// Test that RestartHost transactions targeting the local node invoke the
// configured ActionHandler.
func TestRestartHostActionHandler(t *testing.T) {
	idPath := "test_abci_key_restart.pem"
	defer os.Remove(idPath)
	id, err := identity.LoadOrCreateIdentity(idPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity error: %v", err)
	}

	app := NewABCIApplication(nil, id.PrivateKey())

	called := false
	var gotPayload []byte
	app.ActionHandler = func(action string, payload []byte) error {
		if action != "restart" {
			t.Fatalf("unexpected action: %s", action)
		}
		called = true
		gotPayload = payload
		return nil
	}

	// craft RestartHost tx targeting this node
	payload := types.RestartHostPayload{TargetPublicKey: id.PublicKeyHex()}
	pbytes, _ := json.Marshal(payload)
	tx := &types.Transaction{Type: types.TxRestartHost, Timestamp: time.Now(), Payload: pbytes}
	stx, err := tx.Sign(id)
	if err != nil {
		t.Fatalf("sign restart tx: %v", err)
	}
	txBytes, _ := json.Marshal(stx)

	resp := app.DeliverTx(tmabci.RequestDeliverTx{Tx: txBytes})
	if resp.Code != CodeTypeOK {
		t.Fatalf("DeliverTx restart failed: code=%d log=%s", resp.Code, resp.Log)
	}

	if !called {
		t.Fatalf("expected ActionHandler to be called for restart targeting local node")
	}
	if len(gotPayload) == 0 {
		t.Fatalf("handler received empty payload")
	}
}
