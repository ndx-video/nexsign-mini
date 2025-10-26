package consensus

import (
	"nexsign.mini/nsm/internal/ledger"

	abci "github.com/tendermint/tendermint/abci/types"
)

// ABCIApplication is our custom application that implements the ABCI interface.
type ABCIApplication struct {
	abci.BaseApplication
	state *ledger.State
}

// NewABCIApplication creates a new instance of our ABCI application.
func NewABCIApplication(state *ledger.State) *ABCIApplication {
	return &ABCIApplication{state: state}
}

// Info is called by Tendermint to get information about the application.
func (app *ABCIApplication) Info(req abci.RequestInfo) abci.ResponseInfo {
	// For now, we'll return a simple response.
	// Later, we can use this to handle app versioning and state recovery.
	return abci.ResponseInfo{}
}

// Query is used to query the application state.
// This is how our web dashboard and API will get the list of hosts.
func (app *ABCIApplication) Query(req abci.RequestQuery) abci.ResponseQuery {
	// TODO: Implement query logic to return the ledger state (e.g., as JSON).
	return abci.ResponseQuery{Code: 0}
}

// CheckTx is used to validate transactions before they are committed.
func (app *ABCIApplication) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	// TODO: Implement transaction validation logic.
	return abci.ResponseCheckTx{Code: 0} // 0 means the transaction is valid for now.
}

// DeliverTx is where we process transactions and update the application state.
func (app *ABCIApplication) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	// TODO: Implement transaction processing logic to update the ledger state.
	return abci.ResponseDeliverTx{Code: 0}
}

// Commit is called after a block of transactions has been processed.
func (app *ABCIApplication) Commit() abci.ResponseCommit {
	// TODO: Persist the application state to disk.
	return abci.ResponseCommit{Data: []byte{}}
}