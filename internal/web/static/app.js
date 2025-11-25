// Global application JavaScript for NSM Dashboard

const HOST_TABLE_COLUMN_COUNT = 8;
const ipv4Pattern = /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/;

// Generate unique editor ID for this browser session
let EDITOR_ID = sessionStorage.getItem('nsm_editor_id');
if (!EDITOR_ID) {
    EDITOR_ID = 'editor_' + Math.random().toString(36).substring(2, 15);
    sessionStorage.setItem('nsm_editor_id', EDITOR_ID);
}

function validateIPv4(value) {
    if (!value) {
        return false;
    }
    return ipv4Pattern.test(value);
}

window.toggleSelectAll = function (cb) {
    document.querySelectorAll('.row-select').forEach(el => el.checked = cb.checked);
}

function getSelectedIPs() {
    const ips = [];
    document.querySelectorAll('tbody tr').forEach(tr => {
        const checkbox = tr.querySelector('.row-select');
        if (checkbox && checkbox.checked) {
            const ip = tr.getAttribute('data-ip');
            if (ip) ips.push(ip);
        }
    });
    return ips;
}

window.pushSelected = function () {
    const targets = getSelectedIPs();
    if (targets.length === 0) {
        alert('Select one or more hosts to push to.');
        return;
    }
    fetch('/api/hosts/push', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ targets })
    }).then(() => {
        // No need to trigger refresh, SSE will handle it if state changes
    }).catch(() => { });
}

window.pushAll = function () {
    fetch('/api/hosts/push', { method: 'POST' }).then(() => {
        // No need to trigger refresh
    }).catch(() => { });
}

window.enterEditMode = function (button) {
    const row = button.closest('tr');
    const hostID = row.getAttribute('data-host-id');
    
    if (!hostID) {
        alert('Unable to edit: host ID not found');
        return;
    }

    // Attempt to acquire lock
    fetch('/api/hosts/lock', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            host_id: hostID,
            editor_id: EDITOR_ID
        })
    }).then(resp => resp.json()).then(data => {
        if (!data.success) {
            alert(`This host is currently being edited by ${data.locked_by}. Please try again later.`);
            return;
        }

        // Lock acquired successfully, enter edit mode
        const nicknameDisplay = row.querySelector('.nickname-display');
        const nicknameInput = row.querySelector('.nickname-edit');
        const lanDisplay = row.querySelector('.ip-lan-display');
        const lanInput = row.querySelector('.lan-ip-edit');
        const vpnDisplay = row.querySelector('.vpn-ip-display');
        const vpnInput = row.querySelector('.vpn-ip-edit');
        const notesDisplay = row.querySelector('.notes-display');
        const notesInput = row.querySelector('.notes-edit');

        if (nicknameDisplay && nicknameInput) {
            nicknameDisplay.classList.add('hidden');
            nicknameInput.classList.remove('hidden');
        }

        if (lanDisplay && lanInput) {
            lanDisplay.classList.add('hidden');
            lanInput.classList.remove('hidden');
        }

        if (vpnDisplay && vpnInput) {
            vpnDisplay.classList.add('hidden');
            vpnInput.classList.remove('hidden');
        }

        if (notesDisplay && notesInput) {
            notesDisplay.classList.add('hidden');
            notesInput.classList.remove('hidden');
        }

        [nicknameInput, lanInput, vpnInput, notesInput].forEach(el => {
            if (el) {
                el.dataset.originalValue = el.value;
            }
        });

        row.querySelector('.edit-btn').classList.add('hidden');
        row.querySelector('.save-btn').classList.remove('hidden');
        row.querySelector('.cancel-btn').classList.remove('hidden');

        const enterHandler = (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                window.saveEdit(row.querySelector('.save-btn'));
            } else if (e.key === 'Escape') {
                e.preventDefault();
                window.cancelEdit(row.querySelector('.cancel-btn'));
            }
        };

        [nicknameInput, lanInput, vpnInput].forEach(el => el && el.addEventListener('keydown', enterHandler));
        notesInput && notesInput.addEventListener('keydown', enterHandler);

        if (lanInput) {
            lanInput.focus();
        }
    }).catch(err => {
        alert('Failed to acquire edit lock. Please try again.');
        console.error('Lock acquisition error:', err);
    });
};

window.cancelEdit = function (button) {
    const row = button.closest('tr');
    const hostID = row.getAttribute('data-host-id');

    const nicknameDisplay = row.querySelector('.nickname-display');
    const nicknameInput = row.querySelector('.nickname-edit');
    const lanDisplay = row.querySelector('.ip-lan-display');
    const lanInput = row.querySelector('.lan-ip-edit');
    const vpnDisplay = row.querySelector('.vpn-ip-display');
    const vpnInput = row.querySelector('.vpn-ip-edit');
    const notesDisplay = row.querySelector('.notes-display');
    const notesInput = row.querySelector('.notes-edit');

    [nicknameInput, lanInput, vpnInput, notesInput].forEach(el => {
        if (el && typeof el.dataset.originalValue !== 'undefined') {
            el.value = el.dataset.originalValue;
        }
    });

    if (nicknameDisplay && nicknameInput) {
        nicknameDisplay.classList.remove('hidden');
        nicknameInput.classList.add('hidden');
    }
    if (lanDisplay && lanInput) {
        lanDisplay.classList.remove('hidden');
        lanInput.classList.add('hidden');
    }
    if (vpnDisplay && vpnInput) {
        vpnDisplay.classList.remove('hidden');
        vpnInput.classList.add('hidden');
    }
    if (notesDisplay && notesInput) {
        notesDisplay.classList.remove('hidden');
        notesInput.classList.add('hidden');
    }

    row.querySelector('.edit-btn').classList.remove('hidden');
    row.querySelector('.save-btn').classList.add('hidden');
    row.querySelector('.cancel-btn').classList.add('hidden');

    // Release lock
    if (hostID) {
        fetch('/api/hosts/unlock', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                host_id: hostID,
                editor_id: EDITOR_ID
            })
        }).catch(err => console.error('Failed to release lock:', err));
    }
};

window.saveEdit = function (button) {
    const row = button.closest('tr');
    const oldIP = row.dataset.ip;
    const hostID = row.getAttribute('data-host-id');

    const nicknameInput = row.querySelector('.nickname-edit');
    const lanInput = row.querySelector('.lan-ip-edit');
    const vpnInput = row.querySelector('.vpn-ip-edit');
    const notesInput = row.querySelector('.notes-edit');

    const nickname = nicknameInput ? nicknameInput.value.trim() : '';
    const newIP = lanInput ? lanInput.value.trim() : '';
    const newVPN = vpnInput ? vpnInput.value.trim() : '';
    const notes = notesInput ? notesInput.value.trim() : '';

    if (!validateIPv4(newIP)) {
        alert('Please enter a valid LAN IPv4 address.');
        return;
    }

    if (newVPN && !validateIPv4(newVPN)) {
        alert('VPN IP address must be a valid IPv4 format.');
        return;
    }

    fetch('/api/hosts/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            old_ip: oldIP,
            ip_address: newIP,
            vpn_ip_address: newVPN,
            nickname: nickname,
            notes: notes
        })
    }).then(resp => {
        if (!resp.ok) {
            throw new Error('update failed');
        }
        
        // Release lock after successful save
        if (hostID) {
            fetch('/api/hosts/unlock', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    host_id: hostID,
                    editor_id: EDITOR_ID
                })
            }).catch(err => console.error('Failed to release lock:', err));
        }
        
        // SSE will update the row
    }).catch(() => {
        alert('Failed to update host. Please try again.');
    });
};

window.addHost = function (event) {
    event.preventDefault();

    const nicknameInput = document.getElementById('new-nickname');
    const lanInput = document.getElementById('new-lan-ip');
    const vpnInput = document.getElementById('new-vpn-ip');
    const notesInput = document.getElementById('new-notes');

    const nickname = nicknameInput.value.trim();
    const lanIP = lanInput.value.trim();
    const vpnIP = vpnInput.value.trim();
    const notes = notesInput.value.trim();

    if (!validateIPv4(lanIP)) {
        alert('Please enter a valid LAN IPv4 address.');
        return;
    }

    if (vpnIP && !validateIPv4(vpnIP)) {
        alert('VPN IP address must be a valid IPv4 format.');
        return;
    }

    fetch('/api/hosts/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            nickname: nickname,
            ip_address: lanIP,
            vpn_ip_address: vpnIP,
            notes: notes
        })
    }).then(resp => {
        if (!resp.ok) {
            throw new Error('add failed');
        }

        nicknameInput.value = '';
        lanInput.value = '';
        vpnInput.value = '';
        notesInput.value = '';

        lanInput.blur();
    }).catch(() => {
        alert('Failed to add host. Please try again.');
    });
};

window.toggleInfo = function (button, ip) {
    const row = button.closest('tr');
    const existingInfoRow = document.querySelector('.info-row');

    if (existingInfoRow) {
        existingInfoRow.remove();
        if (existingInfoRow.dataset.ip === ip) {
            return;
        }
    }

    const infoRow = document.createElement('tr');
    infoRow.className = 'info-row bg-black';
    infoRow.dataset.ip = ip;

    const infoCell = document.createElement('td');
    infoCell.colSpan = HOST_TABLE_COLUMN_COUNT;
    infoCell.className = 'px-4 py-4';
    infoCell.innerHTML = '<div class="text-desert-tan text-sm"><span class="text-cyan-400 italic">Loading device information...</span></div>';

    infoRow.appendChild(infoCell);
    row.after(infoRow);

    Promise.all([
        fetch(`/api/proxy/anthias?ip=${ip}&path=/api/v2/device_settings`).then(r => r.ok ? r.json() : null),
        fetch(`/api/proxy/anthias?ip=${ip}&path=/api/v1/info`).then(r => r.ok ? r.json() : null),
        fetch(`/api/proxy/anthias?ip=${ip}&path=/api/v1/assets?format=json`).then(r => r.ok ? r.json() : null)
    ]).then(([settings, info, assets]) => {
        let html = '<div class="text-desert-tan text-sm space-y-3">';

        if (settings) {
            html += '<div class="font-semibold text-desert-cyan mb-2">Device Settings:</div>';
            html += '<div class="grid grid-cols-3 gap-x-4 gap-y-2 ml-4">';
            for (const [key, value] of Object.entries(settings)) {
                if (value !== null && value !== undefined && value !== '') {
                    const label = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                    let displayValue = value;
                    if (typeof value === 'boolean') {
                        displayValue = value ? 'Yes' : 'No';
                    }
                    html += `<div><span class="text-gray-400">${label}:</span> ${displayValue}</div>`;
                }
            }
            html += '</div>';
        }

        if (info) {
            html += '<div class="font-semibold text-desert-cyan mt-3 mb-2">System Info:</div>';
            html += '<div class="grid grid-cols-3 gap-x-4 gap-y-2 ml-4">';
            for (const [key, value] of Object.entries(info)) {
                if (value !== null && value !== undefined && value !== '') {
                    const label = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                    html += `<div><span class="text-gray-400">${label}:</span> ${value}</div>`;
                }
            }
            html += '</div>';
        }

        if (assets && Array.isArray(assets)) {
            html += '<div class="font-semibold text-desert-cyan mt-3 mb-2">Assets: ' + assets.length + '</div>';
            if (assets.length > 0) {
                const mediaAssets = assets.filter(a =>
                    a.mimetype && (a.mimetype.startsWith('image/') || a.mimetype.startsWith('video/'))
                ).slice(0, 5);

                if (mediaAssets.length > 0) {
                    html += '<div class="ml-4 mb-3 flex gap-2 flex-wrap">';
                    mediaAssets.forEach(asset => {
                        if (asset.uri) {
                            const thumbnailUrl = `http://${ip}${asset.uri}`;
                            if (asset.mimetype.startsWith('image/')) {
                                html += `<img src="${thumbnailUrl}" class="h-20 w-auto object-cover rounded border border-gray-600" title="${asset.name || 'Unnamed'}" onerror="this.style.display='none'">`;
                            } else if (asset.mimetype.startsWith('video/')) {
                                html += `<video class="h-20 w-auto object-cover rounded border border-gray-600" title="${asset.name || 'Unnamed'}" muted><source src="${thumbnailUrl}" type="${asset.mimetype}"></video>`;
                            }
                        }
                    });
                    html += '</div>';
                }

                html += '<div class="ml-4 space-y-1 max-h-40 overflow-y-auto">';
                assets.forEach(asset => {
                    html += `<div class="text-xs flex items-center gap-2">`;
                    if (asset.is_enabled) html += `<span class="text-green-400">●</span>`;
                    else html += `<span class="text-gray-600">○</span>`;
                    html += `<span class="text-gray-300">${asset.name || 'Unnamed'}</span>`;
                    if (asset.mimetype) html += ` <span class="text-gray-500">(${asset.mimetype})</span>`;
                    if (asset.duration) html += ` <span class="text-gray-500">${asset.duration}s</span>`;
                    html += `</div>`;
                });
                html += '</div>';
            }
        }

        html += '</div>';

        if (!settings && !info && !assets) {
            html = '<div class="text-red-400 text-sm">Failed to retrieve device information. Anthias API may be unavailable.</div>';
        }

        infoCell.innerHTML = html;
    }).catch(() => {
        infoCell.innerHTML = '<div class="text-red-400 text-sm">Error fetching device information.</div>';
    });
};

// Advanced View Functions
function downloadHostList() {
  fetch('/api/hosts/export/download')
    .then(resp => resp.blob())
    .then(blob => {
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'nsm-hosts-' + new Date().toISOString().split('T')[0] + '.json';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
    })
    .catch(err => {
      alert('Failed to download host list: ' + err.message);
    });
}

function uploadHostList(input) {
  const file = input.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = function(e) {
    try {
      const hosts = JSON.parse(e.target.result);
      fetch('/api/hosts/import/upload', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(hosts)
      })
      .then(resp => {
        if (!resp.ok) throw new Error('Upload failed');
        alert('Host list imported successfully!');
        input.value = ''; // Clear the file input
      })
      .catch(err => {
        alert('Failed to import host list: ' + err.message);
      });
    } catch (err) {
      alert('Invalid JSON file: ' + err.message);
    }
  };
  reader.readAsText(file);
}

// WebSocket connection for diagnostics (Advanced View)
function initDiagnosticsWebSocket() {
  const wsIndicator = document.getElementById('diag-ws');
  const tEl = document.getElementById('diag-time');
  const idEl = document.getElementById('diag-nodeid');
  const hostsEl = document.getElementById('diag-hosts');
  const backupEl = document.getElementById('diag-backup');
  const consoleEl = document.getElementById('console-logs');

  // Only initialize if elements exist (we're on advanced page)
  if (!wsIndicator) return;

  // Track displayed log messages to avoid duplicates
  const displayedLogs = new Set();

  function connect() {
    const proto = (location.protocol === 'https:') ? 'wss' : 'ws';
    const url = proto + '://' + location.host + '/ws/diagnostics';
    const ws = new WebSocket(url);
    wsIndicator.textContent = 'connecting…'; 
    wsIndicator.className = 'text-desert-yellow';
    
    ws.onopen = () => { 
      wsIndicator.textContent = 'connected'; 
      wsIndicator.className = 'text-desert-green'; 
    };
    
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        if (msg.time) tEl.textContent = msg.time;
        if (msg.node_id) idEl.textContent = msg.node_id;
        if (typeof msg.hosts_count === 'number') hostsEl.textContent = msg.hosts_count;
        if (msg.last_backup) backupEl.textContent = msg.last_backup;
        
        // Accumulate console logs from both /ws/diagnostics and /ws/status
        if (msg.logs && Array.isArray(msg.logs)) {
          msg.logs.forEach(log => {
            const logKey = `${log.timestamp}-${log.text}`;
            if (!displayedLogs.has(logKey)) {
              displayedLogs.add(logKey);
              const ts = new Date(log.timestamp).toLocaleTimeString();
              const levelClass = log.level === 'error' ? 'text-red-400' : 
                                 log.level === 'warning' ? 'text-yellow-400' : 'text-desert-cyan';
              const logDiv = document.createElement('div');
              logDiv.className = levelClass;
              logDiv.textContent = `[${ts}] ${log.text}`;
              consoleEl.appendChild(logDiv);
              
              // Keep only last 200 messages
              while (consoleEl.children.length > 200) {
                const removed = consoleEl.firstChild;
                if (removed && removed.textContent) {
                  const match = removed.textContent.match(/\[(.*?)\] (.*)/);
                  if (match) {
                    displayedLogs.delete(`${match[1]}-${match[2]}`);
                  }
                }
                consoleEl.removeChild(removed);
              }
              
              consoleEl.scrollTop = consoleEl.scrollHeight;
            }
          });
        }
        
        // Also handle single message format (from /ws/status)
        if (msg.text && msg.timestamp && !msg.logs) {
          const logKey = `${msg.timestamp}-${msg.text}`;
          if (!displayedLogs.has(logKey)) {
            displayedLogs.add(logKey);
            const ts = new Date(msg.timestamp).toLocaleTimeString();
            const levelClass = msg.level === 'error' ? 'text-red-400' : 
                               msg.level === 'warning' ? 'text-yellow-400' : 'text-desert-cyan';
            const logDiv = document.createElement('div');
            logDiv.className = levelClass;
            logDiv.textContent = `[${ts}] ${msg.text}`;
            consoleEl.appendChild(logDiv);
            
            // Keep only last 200 messages
            while (consoleEl.children.length > 200) {
              consoleEl.removeChild(consoleEl.firstChild);
            }
            
            consoleEl.scrollTop = consoleEl.scrollHeight;
          }
        }
      } catch(e) {
        console.error('Error parsing WebSocket message:', e);
      }
    };
    
    ws.onclose = () => { 
      wsIndicator.textContent = 'disconnected'; 
      wsIndicator.className = 'text-desert-red'; 
      setTimeout(connect, 1500); 
    };
    
    ws.onerror = () => { 
      try { ws.close(); } catch(e) {} 
    };
  }
  
  connect();
  
  // Also connect to /ws/status to capture status messages
  function connectStatus() {
    const proto = (location.protocol === 'https:') ? 'wss' : 'ws';
    const url = proto + '://' + location.host + '/ws/status';
    const ws = new WebSocket(url);
    
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        if (msg.text && msg.timestamp && consoleEl) {
          const logKey = `${msg.timestamp}-${msg.text}`;
          if (!displayedLogs.has(logKey)) {
            displayedLogs.add(logKey);
            const ts = new Date(msg.timestamp).toLocaleTimeString();
            const levelClass = msg.level === 'error' ? 'text-red-400' : 
                               msg.level === 'warning' ? 'text-yellow-400' : 'text-desert-cyan';
            const logDiv = document.createElement('div');
            logDiv.className = levelClass;
            logDiv.textContent = `[${ts}] ${msg.text}`;
            consoleEl.appendChild(logDiv);
            
            // Keep only last 200 messages
            while (consoleEl.children.length > 200) {
              consoleEl.removeChild(consoleEl.firstChild);
            }
            
            consoleEl.scrollTop = consoleEl.scrollHeight;
          }
        }
      } catch(e) {
        console.error('Error parsing status WebSocket message:', e);
      }
    };
    
    ws.onclose = () => {
      setTimeout(connectStatus, 1500);
    };
    
    ws.onerror = () => {
      try { ws.close(); } catch(e) {}
    };
  }
  
  connectStatus();
}

// Load backup history
function loadBackupHistory() {
  const backupList = document.getElementById('backup-list');
  if (!backupList) return;

  fetch('/api/backups/list')
    .then(resp => resp.json())
    .then(backups => {
      if (!backups || backups.length === 0) {
        backupList.innerHTML = '<div class="text-desert-gray italic">No backups found</div>';
        return;
      }

      let html = '';
      backups.forEach(backup => {
        html += `<div class="text-desert-cyan hover:text-desert-yellow cursor-pointer" onclick="restoreBackup('${backup.filename}')">`;
        html += `[${backup.timestamp}] ${backup.filename} (${formatBytes(backup.size)})`;
        html += `</div>`;
      });
      backupList.innerHTML = html;
    })
    .catch(err => {
      backupList.innerHTML = '<div class="text-red-400">Failed to load backup history</div>';
      console.error('Error loading backup history:', err);
    });
}

// Restore from a specific backup
function restoreBackup(filename) {
  if (!confirm(`Restore from backup: ${filename}?\n\nThis will replace your current host list.`)) {
    return;
  }

  fetch(`/api/backups/restore?file=${encodeURIComponent(filename)}`, {
    method: 'POST'
  })
  .then(resp => {
    if (!resp.ok) throw new Error('Restore failed');
    return resp.json();
  })
  .then(data => {
    alert(`Successfully restored ${data.count} hosts from ${data.source}`);
    // Reload the page to show updated host list
    window.location.reload();
  })
  .catch(err => {
    alert('Failed to restore backup: ' + err.message);
  });
}

// Format bytes to human-readable format
function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

// Called when a view is loaded (triggered from navbar clicks)
function onViewLoad(viewName) {
  console.log('View loaded:', viewName);
  
  if (viewName === 'advanced') {
    // Initialize WebSocket for diagnostics
    setTimeout(() => {
      initDiagnosticsWebSocket();
      loadBackupHistory();
    }, 20); // Small delay to ensure DOM is ready
  }
}
