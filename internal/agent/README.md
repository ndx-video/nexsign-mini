# internal/agent

Privileged helper agent for executing sensitive actions (such as service restarts) on behalf of the main `nsm` process.

## Purpose

The agent provides a secure privilege separation model where the main `nsm` process can run with minimal privileges, and sensitive system operations are delegated to a separate agent process that runs with the necessary elevated permissions.

## Usage

### Starting the Agent

Run the agent as a privileged process (e.g., with `sudo` or as a systemd service with elevated permissions):

```bash
# Run directly with sudo
sudo go run internal/agent/agent.go

# Or build and run
go build -o nsm-agent internal/agent/agent.go
sudo ./nsm-agent
```

By default, the agent listens on `localhost:9001`.

### Configuring the Main Process

Configure the main `nsm` process to use the agent by setting the `restart_command` in your config file to point to the agent HTTP endpoint:

```json
{
  "restart_command": "http://localhost:9001/action",
  "enable_actions": true
}
```

### How It Works

1. The main `nsm` process receives an `ActionTransaction` (e.g., restart command) via ABCI consensus.
2. If `enable_actions` is true and `restart_command` is an HTTP URL, the action handler POSTs to the agent.
3. The agent receives the request, validates it, and executes the configured system command (e.g., `systemctl restart nsm`).
4. The agent returns success/failure to the main process.

## API

### POST /action

Execute an action via the agent.

**Request Body:**
```json
{
  "action": "restart",
  "payload": "<base64-encoded-payload>"
}
```

**Response:**
- `200 OK` - Action executed successfully
- `400 Bad Request` - Invalid request format
- `405 Method Not Allowed` - Wrong HTTP method
- `500 Internal Server Error` - Action execution failed
- `501 Not Implemented` - Unknown action type

## Security Considerations

- **Access Control**: The agent should only be accessible from `localhost` or trusted sources. Do not expose it to external networks.
- **Privilege Separation**: Run the main `nsm` process with minimal privileges and only run the agent with elevated permissions.
- **Authentication**: In production, consider adding authentication/authorization between the main process and agent (e.g., shared secret, mutual TLS).
- **Audit Logging**: All actions executed by the agent are logged. Review logs regularly.

## Production Deployment

For production, run the agent as a systemd service:

1. Create `/etc/systemd/system/nsm-agent.service`:

```ini
[Unit]
Description=nexSign mini privileged agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/nsm-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

2. Enable and start:

```bash
sudo systemctl enable nsm-agent
sudo systemctl start nsm-agent
```

## Alternative: Direct Execution

If you don't need privilege separation, you can configure `restart_command` to execute commands directly:

```json
{
  "restart_command": "systemctl restart nsm",
  "enable_actions": true
}
```

The main process will execute the command via shell. Ensure the process has the necessary permissions.

## Development

During development and testing, keep `enable_actions` set to `false` to prevent accidental system restarts.
