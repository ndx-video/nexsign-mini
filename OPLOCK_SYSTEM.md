# Optimistic Locking System (Oplock)

## Overview

The oplock system prevents concurrent edits across multiple dashboards by implementing a distributed locking mechanism with real-time state synchronization via SSE.

## Architecture

### Server Components

#### 1. Lock Storage (`/internal/web/server.go`)

```go
type Server struct {
    // ... other fields
    editLocks  map[string]string // hostID -> editorID
    editMu     sync.RWMutex
}
```

- **editLocks**: Maps host IDs to editor IDs (browser session identifiers)
- **editMu**: RWMutex for thread-safe access to the lock map

#### 2. Lock/Unlock Endpoints

**POST `/api/hosts/lock`**
- Acquires an edit lock for a specific host
- Request body: `{ "host_id": "...", "editor_id": "..." }`
- Returns: `{ "success": true }` or `{ "success": false, "locked_by": "..." }`
- On success:
  - Adds lock to `editLocks` map
  - Broadcasts lock state via SSE to all connected dashboards
  - Announces lock to online peers on same subnet

**POST `/api/hosts/unlock`**
- Releases an edit lock
- Request body: `{ "host_id": "...", "editor_id": "..." }`
- Validates that the requester owns the lock (or allows force unlock if no editor_id)
- On success:
  - Removes lock from `editLocks` map
  - Broadcasts updated lock state via SSE
  - Announces unlock to online peers

#### 3. SSE Broadcasting

**Lock State Event**
```
event: lock-state
data: { "locks": { "host_id_1": "editor_abc", "host_id_2": "editor_xyz" } }
```

- Sent whenever locks are acquired or released
- Received by all connected dashboard clients
- Triggers UI updates to show/hide lock overlays

#### 4. Peer Synchronization

`announceLockToPeers(hostID, editorID, isLock bool)`
- Announces lock/unlock operations to healthy peers on same subnet
- Uses POST to peer's `/api/hosts/lock` or `/api/hosts/unlock` endpoints
- Ensures all dashboards across the fleet see the same lock state

### Client Components

#### 1. Editor ID Generation (`/internal/web/static/app.js`)

```javascript
let EDITOR_ID = sessionStorage.getItem('nsm_editor_id');
if (!EDITOR_ID) {
    EDITOR_ID = 'editor_' + Math.random().toString(36).substring(2, 15);
    sessionStorage.setItem('nsm_editor_id', EDITOR_ID);
}
```

- Unique identifier persisted per browser tab session
- Used to track which dashboard is editing which host

#### 2. Edit Mode with Lock Acquisition

`enterEditMode(button)`:
1. Extracts `data-host-id` from table row
2. Sends POST to `/api/hosts/lock` with `host_id` and `editor_id`
3. If lock acquired:
   - Show edit fields
   - Hide display fields
   - Enable save/cancel buttons
   - Add ESC key handler for cancel
4. If lock denied:
   - Show alert with current editor's ID
   - Prevent edit mode activation

#### 3. Lock Release

**On Save** (`saveEdit`):
- After successful `/api/hosts/update`
- Sends POST to `/api/hosts/unlock`
- Lock released automatically

**On Cancel** (`cancelEdit`):
- Reverts field values to original
- Sends POST to `/api/hosts/unlock`
- Lock released immediately

**On ESC Key**:
- Triggers `cancelEdit` function
- Lock released via unlock endpoint

### Template Components

#### Lock State Rendering (`/internal/web/host-rows.html`)

```html
{{$isLocked := index $.EditLocks .ID}}
<tr ... {{if $isLocked}}data-locked-by="{{$isLocked}}" class="opacity-60 pointer-events-none relative"{{end}}>
    {{if $isLocked}}
    <td colspan="8" class="absolute inset-0 flex items-center justify-center bg-black bg-opacity-50 text-desert-yellow font-bold text-lg z-10 pointer-events-none">
        ⚠️ INFORMATION IS BEING EDITED BY {{$isLocked}}
    </td>
    {{end}}
    ...
    <button class="edit-btn" ... {{if $isLocked}}disabled{{end}}>✏️</button>
    ...
</tr>
```

**Locked Row Styling**:
- `opacity-60`: Greyed-out appearance
- `pointer-events-none`: Disables all interactions
- Overlay message with editor ID
- Disabled edit/delete/info buttons
- Disabled row checkbox

## Data Flow

### Lock Acquisition Flow

1. User clicks edit button on Dashboard A
2. Browser sends POST `/api/hosts/lock` with `host_id` and `editor_id`
3. Server checks `editLocks` map:
   - If unlocked: Add to map, return success
   - If locked by different editor: Return failure with `locked_by`
4. Server broadcasts `lock-state` event via SSE to all connected browsers
5. Server sends POST to peers' `/api/hosts/lock` endpoints
6. All dashboards receive SSE update and re-render host table with lock overlay
7. Dashboard A enters edit mode

### Lock Release Flow

1. User clicks save/cancel or presses ESC on Dashboard A
2. Browser sends POST `/api/hosts/unlock` with `host_id` and `editor_id`
3. Server validates ownership and removes from `editLocks` map
4. Server broadcasts updated `lock-state` via SSE
5. Server sends POST to peers' `/api/hosts/unlock` endpoints
6. All dashboards re-render host table, removing lock overlay

### SSE Update Flow

```
Server Store Change
  ↓
watchHostUpdates() goroutine detects change
  ↓
renderHostListFragment() includes current EditLocks
  ↓
SSE broadcast to all connected dashboards
  ↓
Datastar merges fragment (updates host table with lock state)
```

## Race Condition Handling

### Concurrent Lock Attempts
- First request to acquire lock wins
- Subsequent requests receive `locked_by` response
- User sees friendly alert message

### Network Partitions
- Locks are server-authoritative (not distributed consensus)
- If peer unreachable, local lock still prevents edits
- When peer recovers, next SSE/peer-sync updates lock state

### Stale Locks
- No automatic expiration currently implemented
- User can force unlock by:
  - Closing browser tab (session ends, but lock persists)
  - **Future**: Add timeout-based lock expiration
  - **Future**: Add admin "force unlock" button in Advanced view

## Edge Cases

### Browser Tab Closed During Edit
- Lock remains in `editLocks` map
- **Current behavior**: Lock persists until server restart
- **Future improvement**: Add heartbeat/timeout mechanism

### Server Restart
- `editLocks` map is in-memory (not persisted)
- All locks cleared on restart
- Normal operation resumes

### Network Failures During Lock
- If lock request fails: User sees error, edit mode not entered
- If unlock request fails: Lock eventually times out (future improvement)
- SSE reconnection handles dashboard state refresh

## Configuration

No configuration required. Lock system is enabled by default.

## Testing

### Single Dashboard Test
1. Open dashboard, click edit on a host
2. Verify edit mode activates
3. Press ESC or cancel
4. Verify edit mode exits and lock released

### Multi-Dashboard Test
1. Open dashboard in two browser tabs (Tab A and Tab B)
2. On Tab A: Click edit on host X
3. Verify Tab A enters edit mode
4. On Tab B: Verify host X shows lock overlay with editor ID
5. On Tab B: Try to click edit on host X
6. Verify Tab B shows "already being edited" alert
7. On Tab A: Save or cancel
8. Verify Tab B lock overlay disappears

### Multi-Server Test
1. Access Dashboard A on Server 1
2. Access Dashboard B on Server 2 (same subnet)
3. On Dashboard A: Edit host X
4. Verify Dashboard B shows lock overlay
5. On Dashboard A: Cancel edit
6. Verify Dashboard B lock overlay clears

## Future Enhancements

- [ ] Lock expiration/timeout (e.g., 5 minutes)
- [ ] Heartbeat to keep lock alive during long edits
- [ ] Admin "force unlock" in Advanced view
- [ ] Lock history/audit log
- [ ] Visual indication of "self" vs "other" locks
- [ ] Lock queue (notify when lock becomes available)
