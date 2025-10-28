# internal/actions

Action execution router for handling ActionTransactions that target the local node.

## Purpose

This package provides a safe, configurable execution layer for actions received via consensus (e.g., restart commands). It acts as a router that dispatches actions to appropriate handlers, with built-in safety defaults and production-ready execution paths.

## Design Philosophy

### Safety First

- **Default Behavior**: Log actions without executing them (safe for development/tests)
- **Opt-In Execution**: Production execution must be explicitly enabled via configuration
- **Flexible Backends**: Support both direct command execution and agent-based delegation

### Configuration-Driven

All action execution is controlled by the central config file (`/etc/nsm/config.json`):

```json
{
  "enable_actions": true,
  "restart_command": "systemctl restart nsm"
}
```

## Usage

### Basic Usage

The main `nsm` process wires the action handler to the ABCI app:

```go
import "nexsign.mini/nsm/internal/actions"

// In cmd/nsm/main.go
abciApp.ActionHandler = func(action string, payload []byte) error {
    return actions.ExecuteAction(action, payload)
}
```

### Execution Modes

The `ExecuteAction` router supports three execution modes based on configuration:

#### 1. Safe Mode (Default)

```json
{
  "enable_actions": false
}
```

Actions are logged but not executed. Ideal for development and testing.

#### 2. Direct Execution Mode

```json
{
  "enable_actions": true,
  "restart_command": "systemctl restart nsm"
}
```

The configured shell command is executed directly via `/bin/sh -c "<command>"`.

**Use Case**: Simple deployments where the `nsm` process runs with sufficient privileges.

**Security Note**: Ensure the process has permissions to execute the command.

#### 3. Agent Mode (Recommended for Production)

```json
{
  "enable_actions": true,
  "restart_command": "http://localhost:9001/action"
}
```

Actions are POSTed to a privileged agent process via HTTP. The agent executes the system command with elevated permissions.

**Use Case**: Production deployments with privilege separation.

**Benefits**:
- Main process runs with minimal privileges
- Agent handles all privileged operations
- Clear separation of concerns
- Easier to audit and secure

## Supported Actions

### restart

Executes a restart action for the local node.

**Payload**: `types.RestartHostPayload`

```go
type RestartHostPayload struct {
    TargetPublicKey string `json:"target_public_key"`
}
```

**Behavior**:
- If `enable_actions` is false: Log only
- If `restart_command` is HTTP URL: POST to agent
- Otherwise: Execute as shell command

## Adding New Actions

To add a new action type:

1. Add a new case to the `ExecuteAction` switch:

```go
func ExecuteAction(action string, payload []byte) error {
    switch action {
    case "restart":
        return handleRestart(payload)
    case "update_config":  // New action
        return handleUpdateConfig(payload)
    default:
        log.Printf("actions: unknown action '%s' received; ignoring", action)
        return nil
    }
}
```

2. Implement the handler function:

```go
func handleUpdateConfig(payload []byte) error {
    // Decode payload
    // Check config.Get().EnableActions
    // Execute or POST to agent
    return nil
}
```

3. Update the agent (if using agent mode) to handle the new action.

## Error Handling

Handlers return errors when:
- Payload cannot be decoded
- Command execution fails
- Agent returns non-200 status

The ABCI app translates these errors to appropriate response codes for the blockchain.

## Testing

Action handlers are designed to be testable:

```go
// In tests, inject a mock handler
app.ActionHandler = func(action string, payload []byte) error {
    if action != "restart" {
        t.Fatalf("unexpected action: %s", action)
    }
    // Assert expectations
    return nil
}
```

See `internal/abci/app_test.go` for examples.

## Security Considerations

### Command Injection

When using direct execution mode, the `restart_command` is executed via shell. Ensure:
- The config file is protected (read-only, owned by trusted user)
- The command string is static and not constructed from user input
- Consider using agent mode for better isolation

### Agent Authentication

When using agent mode, consider:
- Restricting agent to localhost only
- Adding mutual TLS for agent communication
- Using a shared secret or token for authentication

### Authorization

The action execution layer does NOT perform authorization checks. Authorization should be handled at the consensus layer (ABCI CheckTx) before actions reach this point.

## Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_actions` | bool | `false` | Enable action execution |
| `restart_command` | string | `"systemctl restart nsm"` | Command or agent URL |

## Examples

### Example 1: Safe Development Mode

```json
{
  "enable_actions": false
}
```

All actions are logged but not executed.

### Example 2: Direct Systemctl

```json
{
  "enable_actions": true,
  "restart_command": "systemctl restart nsm"
}
```

Requires the process to run with systemctl permissions.

### Example 3: Custom Script

```json
{
  "enable_actions": true,
  "restart_command": "/usr/local/bin/nsm-restart.sh"
}
```

Delegates to a custom script that handles the restart logic.

### Example 4: Agent-Based (Recommended)

```json
{
  "enable_actions": true,
  "restart_command": "http://localhost:9001/action"
}
```

POSTs to a privileged agent. See `internal/agent/README.md` for agent setup.
