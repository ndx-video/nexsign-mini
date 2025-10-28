# internal/config

Centralized runtime configuration for nexSign mini.

## Purpose

This package provides a single source of truth for all configurable runtime settings in the `nsm` service. It loads configuration from a JSON file with sensible defaults, allowing the application to run out-of-the-box in development while supporting production customization.

## Design Philosophy

### Configuration Precedence

1. **JSON Config File**: Primary source of configuration (default: `/etc/nsm/config.json`)
2. **Environment Variables**: Can override individual config values for quick tweaks
3. **Defaults**: Built-in defaults ensure the service runs without any configuration

### Graceful Degradation

If the config file is missing or malformed, the service uses defaults and logs a warning rather than failing to start. This makes development and testing frictionless.

## Usage

### Loading Configuration

```go
import "nexsign.mini/nsm/internal/config"

// Load config from default path or use defaults
cfg, _ := config.LoadConfig("/etc/nsm/config.json")

// Access configuration values
keyFile := cfg.KeyFile
port := cfg.Port
```

### Getting the Global Config

```go
// After LoadConfig is called once, use Get() anywhere
cfg := config.Get()
if cfg.EnableActions {
    // Execute privileged actions
}
```

## Configuration File Format

Create a JSON file (e.g., `/etc/nsm/config.json`):

```json
{
  "key_file": "nsm_key.pem",
  "host_data_file": "test-hosts.json",
  "port": 8080,
  "mdns_service_name": "_nsm._tcp",
  "tendermint_peers_file": "tendermint_persistent_peers",
  "log_file": "nsm.log",
  "restart_command": "systemctl restart nsm",
  "enable_actions": false
}
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `key_file` | string | `"nsm_key.pem"` | Path to the node's ED25519 private key (identity) |
| `host_data_file` | string | `"test-hosts.json"` | Path to initial host state JSON file |
| `port` | int | `8080` | HTTP port for web server and mDNS announcement |
| `mdns_service_name` | string | `"_nsm._tcp"` | mDNS service name for local discovery |
| `tendermint_peers_file` | string | `"tendermint_persistent_peers"` | File to write discovered peer addresses |
| `log_file` | string | `"nsm.log"` | Path to log file (rotated via lumberjack) |
| `restart_command` | string | `"systemctl restart nsm"` | Command or agent URL for restart actions |
| `enable_actions` | bool | `false` | Enable execution of ActionTransactions |

### Field Merging

If a field is omitted or empty in the JSON file, the default value is used. This allows partial configuration files:

```json
{
  "port": 9000,
  "enable_actions": true
}
```

All other fields will use defaults.

## Environment Variable Overrides

The main application (`cmd/nsm/main.go`) reads environment variables that override config values:

```bash
# Override the config file path
export CONFIG_FILE=/opt/nsm/custom-config.json

# Override individual settings
export KEY_FILE=/secure/nsm_key.pem
export PORT=9000
export MDNS_SERVICE_NAME=_custom._tcp
```

## Development vs Production

### Development Setup

No configuration file needed! Just run:

```bash
go run cmd/nsm/main.go
```

Defaults are optimized for local development:
- Listens on port 8080
- Uses `nsm_key.pem` in current directory
- Actions disabled by default (safe)
- Logs to `nsm.log`

### Production Setup

1. Create `/etc/nsm/config.json` with production values:

```json
{
  "key_file": "/var/lib/nsm/nsm_key.pem",
  "host_data_file": "/var/lib/nsm/hosts.json",
  "port": 8080,
  "log_file": "/var/log/nsm/nsm.log",
  "restart_command": "http://localhost:9001/action",
  "enable_actions": true
}
```

2. Ensure proper permissions:

```bash
sudo chmod 600 /etc/nsm/config.json
sudo chown nsm:nsm /etc/nsm/config.json
```

3. Start the service:

```bash
systemctl start nsm
```

## Sample Configuration

A sample configuration file is provided at `deploy/config.json.sample`. Copy and customize it:

```bash
sudo cp deploy/config.json.sample /etc/nsm/config.json
sudo nano /etc/nsm/config.json
```

## Security Considerations

### File Permissions

The config file may contain sensitive paths and settings. Protect it:

```bash
sudo chmod 600 /etc/nsm/config.json
sudo chown nsm:nsm /etc/nsm/config.json
```

### Sensitive Fields

- `restart_command`: If using direct shell execution, ensure the command is static and trusted
- `key_file`: Path to the node's identity; protect this file with 0600 permissions

### Enable Actions Carefully

Setting `enable_actions: true` allows the node to execute system commands when it receives ActionTransactions. Only enable this in production after:
- Verifying the consensus network is trusted
- Ensuring proper authorization is in place at the ABCI layer
- Testing the restart mechanism in a staging environment

## Testing

In tests, config loading falls back to defaults, so no config file is required. To test with a specific config:

```go
func TestWithConfig(t *testing.T) {
    cfg, _ := config.LoadConfig("testdata/test-config.json")
    if cfg.Port != 9999 {
        t.Fatalf("expected port 9999, got %d", cfg.Port)
    }
}
```

## Extending the Configuration

To add a new configuration field:

1. Add the field to the `Config` struct:

```go
type Config struct {
    // ...existing fields...
    MyNewField string `json:"my_new_field"`
}
```

2. Add a default value in `LoadConfig`:

```go
def := &Config{
    // ...existing defaults...
    MyNewField: "default-value",
}
```

3. Add the field merging logic:

```go
if c.MyNewField == "" {
    c.MyNewField = def.MyNewField
}
```

4. Document the new field in this README and `deploy/config.json.sample`.

## Troubleshooting

### Config file not found

If `/etc/nsm/config.json` doesn't exist, the service will start with defaults and log:

```
No config file found at /etc/nsm/config.json, using defaults
```

This is normal for development. In production, create the file if you need custom settings.

### Config parse error

If the JSON is malformed, the service will fall back to defaults and log a warning. Check your JSON syntax:

```bash
cat /etc/nsm/config.json | jq .
```

### Environment variables not working

Environment variable overrides are implemented in `cmd/nsm/main.go`, not in the config package itself. Check that the main application is calling `getEnv()` with the config value as the fallback.
