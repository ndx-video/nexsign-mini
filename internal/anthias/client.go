package anthias

import (
	"nexsign.mini/nsm/internal/types"
)

// Client is responsible for communicating with the local Anthias instance.
type Client struct {
	// We can add configuration here later, like the API endpoint for Anthias.
}

// NewClient creates a new Anthias client.
func NewClient() *Client {
	return &Client{}
}

// GetMetadata fetches the metadata from the local Anthias instance.
// For now, it returns mock data for development purposes.
func (c *Client) GetMetadata() (*types.Host, error) {
	// TODO: Replace this with actual API calls to a local Anthias service.
	mockHost := &types.Host{
		Hostname:       "anthias-dev-box",
		IPAddress:      "127.0.0.1",
		AnthiasVersion: "v2.0.0-mock",
		AnthiasStatus:  "Running",
		DashboardURL:   "http://localhost:8080", // Assuming a default dev port
	}

	return mockHost, nil
}
