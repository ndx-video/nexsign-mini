package agent
// Package agent provides a privileged helper process for executing sensitive
// actions (such as restart) on behalf of the nsm node. This agent should run
// with the necessary privileges and expose a local socket or HTTP endpoint.
// The main nsm process can be configured to call the agent instead of running
// system commands directly, improving security and isolation.
package agent

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
)

// ActionRequest is the payload sent to the agent for execution.
type ActionRequest struct {
	Action  string `json:"action"`
	Payload []byte `json:"payload"`
}

// Serve starts the agent HTTP server on the given address (e.g., "localhost:9001").
func Serve(addr string) error {
	http.HandleFunc("/action", handleAction)
	log.Printf("agent: listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
		return
	}
	if req.Action == "restart" {
		cmd := exec.Command("/bin/sh", "-c", "systemctl restart nsm")
		if err := cmd.Run(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("restart failed"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("restarted"))
		return
	}
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("unknown action"))
}
