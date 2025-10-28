# internal/web

Web server and dashboard UI for nexSign mini.

## Purpose

This package provides the HTTP server and HTMX-based dashboard interface for monitoring and managing nexSign nodes. It serves both the web UI and a simple REST API.

## Components

### Server

The main web server struct:

```go
type Server struct {
    app    *abci.ABCIApplication  // Access to ABCI state
    port   int                     // HTTP port
    server *http.Server
}
```

### Templates

The UI is built using Go templates with HTMX for dynamic updates:

- `layout.html`: Base layout with navigation
- `home-view.html`: Dashboard home page with host list
- `host-view.html`: Detailed view of a single host

### Static Assets

- `static/htmx.min.js`: HTMX library for dynamic updates

## Usage

### Starting the Server

```go
import "nexsign.mini/nsm/internal/web"

// Create server with ABCI app and port
server, err := web.NewServer(abciApp, 8080)
if err != nil {
    log.Fatal(err)
}

// Start in background (non-blocking)
server.Start()

// Server is now running on http://localhost:8080
```

### Accessing the Dashboard

Open a web browser to `http://localhost:8080` to view:
- List of all known hosts
- Host status and version information
- IP addresses and dashboard links
- Last seen timestamps

## Endpoints

### Web UI Endpoints

#### GET /

Dashboard home page showing all hosts in a table.

**Response**: HTML page with host list

#### GET /host/{publicKey}

Detailed view of a specific host.

**Response**: HTML page with host details

### API Endpoints

**Note**: API endpoints are planned but not yet fully implemented.

#### GET /api/hosts

List all hosts in JSON format.

**Response**:
```json
{
  "abc123...": {
    "hostname": "signage-1",
    "ip_address": "192.168.1.10",
    "anthias_version": "0.18.2",
    "anthias_status": "Online",
    "dashboard_url": "http://192.168.1.10:80",
    "public_key": "abc123..."
  }
}
```

#### GET /api/host/{publicKey}

Get details of a specific host.

**Response**:
```json
{
  "hostname": "signage-1",
  "ip_address": "192.168.1.10",
  "anthias_version": "0.18.2",
  "anthias_status": "Online",
  "dashboard_url": "http://192.168.1.10:80",
  "public_key": "abc123..."
}
```

#### POST /api/action

Submit an action (e.g., restart) for a host.

**Request Body**:
```json
{
  "action": "restart",
  "target_public_key": "abc123..."
}
```

**Response**:
```json
{
  "status": "submitted",
  "transaction_hash": "def456..."
}
```

## HTMX Integration

The dashboard uses HTMX for dynamic, server-driven updates without writing JavaScript:

### Polling for Updates

The host list automatically refreshes every 5 seconds:

```html
<div hx-get="/hosts-partial" hx-trigger="every 5s" hx-swap="outerHTML">
  <!-- Host list rendered here -->
</div>
```

### Action Buttons

Host action buttons trigger POST requests:

```html
<button hx-post="/action/restart" 
        hx-vals='{"target": "abc123..."}'
        hx-swap="none">
  Restart
</button>
```

### Status Indicators

Status badges update dynamically based on host state:

```html
<span class="badge badge-{{ .Status }}">
  {{ .AnthiasStatus }}
</span>
```

## Templates

### Layout Structure

All pages use a common layout defined in `layout.html`:

```html
<!DOCTYPE html>
<html>
<head>
  <title>nexSign mini</title>
  <script src="/static/htmx.min.js"></script>
</head>
<body>
  {{ template "content" . }}
</body>
</html>
```

### Rendering Templates

Templates are loaded and parsed at server startup:

```go
templates := template.Must(template.ParseFS(templateFS, "*.html"))
```

### Adding a New Page

1. Create a new template file in `internal/web/`
2. Define the template with `{{ define "page-name" }}`
3. Add a handler in `server.go`:

```go
func (s *Server) handleNewPage(w http.ResponseWriter, r *http.Request) {
    data := map[string]interface{}{
        "Title": "New Page",
    }
    templates.ExecuteTemplate(w, "page-name.html", data)
}
```

4. Register the route in `NewServer()`

## Styling

The UI uses minimal inline CSS for styling. For production, consider:
- Adding a CSS framework (Bootstrap, Tailwind, etc.)
- Using custom stylesheets
- Implementing dark mode
- Adding responsive design for mobile

## Configuration

The web server port is configurable:

```json
{
  "port": 8080
}
```

Or via environment variable:

```bash
export PORT=9000
```

## Security Considerations

### Access Control

**Current State**: No authentication or authorization

**Production TODO**:
- Add basic auth for sensitive operations
- Implement API keys for programmatic access
- Add HTTPS/TLS support
- Rate limit action submissions

### CSRF Protection

Action submissions should include CSRF tokens to prevent cross-site request forgery.

### Input Validation

All user inputs (public keys, action parameters) should be validated to prevent injection attacks.

## Development

### Hot Reload

For development with template hot-reload:

```bash
# Use a tool like Air or CompileDaemon
go install github.com/cosmtrek/air@latest
air
```

### Testing Templates

To test template rendering:

```go
func TestTemplateRender(t *testing.T) {
    tmpl := template.Must(template.ParseFiles("home-view.html", "layout.html"))
    data := map[string]interface{}{"Hosts": []types.Host{}}
    err := tmpl.ExecuteTemplate(os.Stdout, "home-view.html", data)
    if err != nil {
        t.Fatal(err)
    }
}
```

## Future Enhancements

### Real-Time Updates

Replace polling with WebSocket or Server-Sent Events:

```html
<div hx-ws="connect:/ws">
  <!-- Real-time updates via WebSocket -->
</div>
```

### Charts and Graphs

Add visualizations:
- Host status over time
- Network topology
- Action history

### Advanced Filtering

Add client-side filtering and sorting:
- Filter by status
- Search by hostname/IP
- Sort by any column

### Mobile App

Consider building a native mobile app or PWA for remote monitoring.

## Related Packages

- **`internal/abci`**: Provides state data for the UI
- **`internal/types`**: Defines the Host model displayed in the UI
- **`internal/actions`**: Executes actions submitted via the UI

## Static Files

Static files are embedded using `go:embed`:

```go
//go:embed static/*
var staticFS embed.FS
```

This ensures the binary is self-contained with no external file dependencies.

## Example Dashboard View

```
+------------------------------------------+
| nexSign mini Dashboard                   |
+------------------------------------------+
| Hostname   | IP Address    | Status      |
+------------+---------------+-------------+
| signage-1  | 192.168.1.10  | [Online]   |
| signage-2  | 192.168.1.20  | [Offline]  |
| signage-3  | 192.168.1.30  | [Online]   |
+------------+---------------+-------------+
```

## Troubleshooting

### Templates not loading

Check that template files are embedded correctly:
```go
//go:embed *.html
var templateFS embed.FS
```

### Static files 404

Ensure static files are served correctly:
```go
http.Handle("/static/", http.FileServer(http.FS(staticFS)))
```

### Port already in use

Change the port in config or via environment variable:
```bash
PORT=9000 go run cmd/nsm/main.go
```
