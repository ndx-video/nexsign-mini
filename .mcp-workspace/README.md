MCP Server (minimal)

This lightweight server implements a minimal JSON-RPC interface to satisfy MCP-style "initialize" and "ping" requests for local development.

Run (Linux / bash):

```bash
cd .mcp-workspace
go build -o mcpserver
./mcpserver
```

The server listens on `:4000` and exposes:
- `POST /rpc` - JSON-RPC endpoint
- `GET /ping` - quick health check (returns `pong`)

Initialize example (curl):

```bash
curl -sS -X POST http://localhost:4000/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

Troubleshooting initialize timeout in VS Code Agentic frameworks:
- Ensure the server is reachable on `localhost:4000` from the extension host.
- If the extension reports repeated `Waiting for server to respond to initialize`, the agent host may be invoking a different port or expecting a different protocol. Verify the extension config.
- Use `curl http://localhost:4000/ping` to confirm the server is up.
- Increase any client timeout settings if applicable.
