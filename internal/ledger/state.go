// Package ledger provides a lightweight in-memory representation of the
// replicated ledger state used by the nsm application. It defines the
// State container that holds known hosts. The ABCI application will persist
// and mutate this state in response to confirmed transactions.
package ledger

import "nexsign.mini/nsm/internal/types"

// State represents the full collection of hosts known on the network.
// The map key is the unique identifier for the host (e.g., its public key).
type State struct {
	Hosts map[string]types.Host `json:"hosts"`
}

func NewState() *State {
	return &State{
		Hosts: make(map[string]types.Host),
	}
}
