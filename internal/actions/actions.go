// Package actions provides default action handlers invoked by the ABCI
// application when it receives ActionTransactions targeting the local node.
//
// Handlers should be safe and side-effect-minimal by default. Production
// deployments can replace the handler with a function that performs real
// system-level operations (restart, service control, etc.) if desired.
package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"nexsign.mini/nsm/internal/config"
	"nexsign.mini/nsm/internal/types"
)

// ExecuteAction routes the named action to the appropriate handler.
// It returns an error if the action failed or if the payload is invalid.
func ExecuteAction(action string, payload []byte) error {
	switch action {
	case "restart":
		return handleRestart(payload)
	default:
		// Unknown actions are logged and ignored by default.
		log.Printf("actions: unknown action '%s' received; ignoring", action)
		return nil
	}
}

// handleRestart decodes the RestartHostPayload and performs a default
// safe behavior: log the intent. Production code can replace ExecuteAction
// with a handler that actually restarts services or signals an orchestrator.
func handleRestart(payload []byte) error {
	var p types.RestartHostPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid restart payload: %w", err)
	}
	cfg := config.Get()
	if cfg != nil && cfg.EnableActions {
		cmdStr := cfg.RestartCommand
		if strings.HasPrefix(cmdStr, "http://") || strings.HasPrefix(cmdStr, "https://") {
			// POST to agent
			reqBody, _ := json.Marshal(map[string]interface{}{
				"action":  "restart",
				"payload": payload,
			})
			resp, err := http.Post(cmdStr, "application/json", bytes.NewReader(reqBody))
			if err != nil {
				return fmt.Errorf("agent POST failed: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("agent returned status %d", resp.StatusCode)
			}
			log.Printf("actions: restart POST to agent succeeded for target %s", p.TargetPublicKey)
			return nil
		}
		// Otherwise, execute as shell command
		log.Printf("actions: executing restart command for target %s: %s", p.TargetPublicKey, cmdStr)
		cmd := exec.Command("/bin/sh", "-c", cmdStr)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restart command failed: %v stdout=%s stderr=%s", err, out.String(), stderr.String())
		}
		log.Printf("actions: restart command output: %s", out.String())
		return nil
	}
	// Default safe behavior: log and return success.
	log.Printf("actions: restart requested for target %s (actions disabled; not executing)", p.TargetPublicKey)
	return nil
}
