package ledger

// Host represents the state of a single Anthias host on the network.
type Host struct {
	Hostname       string `json:"hostname"`
	IPAddress      string `json:"ip_address"`
	AnthiasVersion string `json:"anthias_version"`
	AnthiasStatus  string `json:"anthias_status"`
	DashboardURL   string `json:"dashboard_url"`
}

// State represents the full collection of hosts known on the network.
// The map key is the unique identifier for the host (e.g., its IP address or a node ID).
type State struct {
	Hosts map[string]Host `json:"hosts"`
}

func NewState() *State {
	return &State{
		Hosts: make(map[string]Host),
	}
}