// Promptyly Desktop Renderer Logic
const API_BASE = 'http://127.0.0.1:6071';

// Global State
let currentView = 'hub';
let registeredApps = {};
let activeSessions = {}; // name -> { path, prompt }
let currentSessionName = null;
let currentConfig = null;
let selectedProvider = 'gemini';
let apiToken = '';

async function apiFetch(path, options = {}) {
  const url = `${API_BASE}${path}`;
  if (!options.headers) {
    options.headers = {};
  }
  options.headers['X-Promptyly-Token'] = apiToken;
  return fetch(url, options);
}

// Initialize App
async function init() {
  if (window.electronAPI && window.electronAPI.getToken) {
    apiToken = await window.electronAPI.getToken();
  }
  // Navigation binding
  document.querySelectorAll('.nav-item').forEach(item => {
    item.addEventListener('click', (e) => {
      const view = item.getAttribute('data-view');
      switchView(view);
    });
  });

  // Event bindings
  document.getElementById('btn-nav-create').addEventListener('click', () => switchView('create'));
  document.getElementById('btn-link-app').addEventListener('click', handleLinkApp);
  document.getElementById('btn-generate').addEventListener('click', handleCreateApp);
  document.getElementById('btn-save-ai').addEventListener('click', handleSaveAIConfig);
  document.getElementById('btn-save-system').addEventListener('click', handleSaveSystemConfig);
  document.getElementById('btn-save-sharing').addEventListener('click', handleSaveSharingConfig);
  document.getElementById('btn-remote-search').addEventListener('click', handleRemoteSearch);
  document.getElementById('btn-register-protocol').addEventListener('click', handleRegisterProtocol);
  document.getElementById('btn-browse-apps-dir').addEventListener('click', handleBrowseAppsDir);

  // Active session actions
  document.getElementById('btn-session-reload').addEventListener('click', handleSessionReload);
  document.getElementById('btn-session-browser').addEventListener('click', handleSessionOpenBrowser);
  document.getElementById('btn-session-browser-large').addEventListener('click', handleSessionOpenBrowser);
  document.getElementById('btn-session-folder').addEventListener('click', handleSessionOpenFolder);
  document.getElementById('btn-session-export').addEventListener('click', handleSessionExport);
  document.getElementById('btn-session-publish').addEventListener('click', handleSessionPublish);
  document.getElementById('btn-session-rename').addEventListener('click', handleSessionRenameTrigger);
  document.getElementById('btn-session-delete').addEventListener('click', handleSessionDeleteTrigger);
  document.getElementById('btn-session-update').addEventListener('click', handleSessionEdit);
  document.getElementById('btn-session-toggle-browser').addEventListener('click', handleSessionToggleBrowser);

  // Modal actions
  document.getElementById('btn-rename-cancel').addEventListener('click', () => closeModal('modal-rename'));
  document.getElementById('btn-rename-confirm').addEventListener('click', handleRenameConfirm);
  document.getElementById('rename-input-name').addEventListener('input', updateRenamePreviews);
  document.getElementById('rename-input-prompt').addEventListener('input', updateRenamePreviews);
  document.getElementById('btn-delete-cancel').addEventListener('click', () => closeModal('modal-delete'));
  document.getElementById('btn-delete-confirm').addEventListener('click', handleDeleteConfirm);

  // Settings Provider Tabs
  document.querySelectorAll('.provider-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.provider-tab').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      selectedProvider = tab.getAttribute('data-provider');
      renderProviderFields();
    });
  });

  // Global Copy Button Click Handler
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.copy-btn');
    if (!btn) return;
    
    e.stopPropagation();
    const text = btn.getAttribute('data-copy');
    if (!text) return;
    
    navigator.clipboard.writeText(text);
    
    const originalIcon = btn.getAttribute('data-lucide') || 'copy';
    btn.setAttribute('data-lucide', 'check');
    btn.style.color = 'var(--success-color)';
    lucide.createIcons();
    
    setTimeout(() => {
      btn.setAttribute('data-lucide', originalIcon);
      btn.style.color = '';
      lucide.createIcons();
    }, 1500);
  });


  // Handle deep links from the main process
  window.electronAPI.onDeepLink((url) => {
    handleProtocolURL(url);
  });

  // Initial Checkups
  lucide.createIcons();
  await checkDaemonStatus();
  await loadApps();
  await checkProtocolStatus();
  await loadConfig();
  await loadBackgroundSetting();
  setupSidebarResizer();

  // Periodic daemon status check
  setInterval(checkDaemonStatus, 5000);
}

// Sidebar Resizing Drag Logic
function setupSidebarResizer() {
  const resizer = document.getElementById('session-resizer');
  const sidebar = document.querySelector('.session-sidebar');
  const iframe = document.getElementById('app-preview-frame');

  if (!resizer || !sidebar) return;

  let startX, startWidth;

  resizer.addEventListener('mousedown', (e) => {
    e.preventDefault();
    startX = e.clientX;
    startWidth = sidebar.getBoundingClientRect().width;
    
    resizer.classList.add('dragging');
    iframe.style.pointerEvents = 'none'; // Avoid losing mouse events inside iframe

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  });

  function onMouseMove(e) {
    const deltaX = startX - e.clientX;
    let newWidth = startWidth + deltaX;

    const minWidth = 280;
    const maxWidth = window.innerWidth - 300; // Leave at least 300px for preview

    if (newWidth < minWidth) newWidth = minWidth;
    if (newWidth > maxWidth) newWidth = maxWidth;

    sidebar.style.width = `${newWidth}px`;
  }

  function onMouseUp() {
    resizer.classList.remove('dragging');
    iframe.style.pointerEvents = 'auto';

    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }
}


// Switch Views
function switchView(viewName) {
  document.querySelectorAll('.view').forEach(view => {
    view.classList.remove('active');
  });
  
  const targetView = document.getElementById(`view-${viewName}`);
  if (targetView) {
    targetView.classList.add('active');
  } else {
    // If it's an app session, it's inside view-session
    document.getElementById('view-session').classList.add('active');
  }

  document.querySelectorAll('.nav-item').forEach(item => {
    item.classList.remove('active');
    if (item.getAttribute('data-view') === viewName) {
      item.classList.add('active');
    }
  });

  // Highlight active session sidebar link
  document.querySelectorAll('.active-app-item').forEach(item => {
    item.classList.remove('active');
    if (item.getAttribute('data-app') === viewName) {
      item.classList.add('active');
    }
  });

  currentView = viewName;
}

// Check Daemon Connection
async function checkDaemonStatus() {
  const statusDot = document.getElementById('daemon-status-dot');
  const statusText = document.getElementById('daemon-status-text');

  try {
    const res = await apiFetch('/api/apps');
    if (res.ok) {
      statusDot.className = 'status-dot online';
      statusText.textContent = 'Daemon: Online';
      return true;
    }
  } catch (e) {
    // Fail
  }

  statusDot.className = 'status-dot';
  statusText.textContent = 'Daemon: Offline';
  return false;
}

// Load Apps from API
async function loadApps() {
  const container = document.getElementById('apps-container');

  try {
    const res = await apiFetch('/api/apps');
    if (!res.ok) throw new Error('Failed to fetch apps');
    
    registeredApps = await res.json();
    renderAppDashboard();
    renderActiveSessionSidebarLinks();
  } catch (err) {
    container.innerHTML = `
      <div class="empty-state">
        <h3 style="color: var(--error-color);">Daemon Connection Lost</h3>
        <p>Ensure the background Go server is running. Retrying automatically...</p>
      </div>
    `;
  }
}

// Render Hub Dashboard
function renderAppDashboard() {
  const container = document.getElementById('apps-container');
  const appKeys = Object.keys(registeredApps);

  if (appKeys.length === 0) {
    container.innerHTML = `
      <div class="empty-state" style="grid-column: 1 / -1;">
        <i data-lucide="layout" style="width: 48px; height: 48px; margin-bottom: 15px; color: var(--text-muted);"></i>
        <h3>No applications registered</h3>
        <p>Link an existing project folder or create a new one to get started!</p>
        <button class="btn btn-primary" id="btn-empty-create" style="margin-top: 15px;">Create App</button>
      </div>
    `;
    document.getElementById('btn-empty-create').addEventListener('click', () => switchView('create'));
    lucide.createIcons();
    return;
  }

  container.innerHTML = '';
  appKeys.forEach(name => {
    const app = registeredApps[name];
    const displayName = name.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
    
    const card = document.createElement('div');
    card.className = 'app-card';
    const runLink = `prompt://${name}`;
    const createLink = app.prompt ? `prompt://${encodeURIComponent(app.prompt)}` : '';
    
    card.innerHTML = `
      <div class="app-card-top">
        <h3 class="app-card-title" title="${displayName}">${displayName}</h3>
        <p class="app-card-desc" style="margin-bottom: 12px;">${app.prompt || 'Imported or custom application.'}</p>
        
        <div style="background: rgba(0,0,0,0.18); padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(255,255,255,0.03); margin-top: 10px; font-size: 0.75rem; display: flex; flex-direction: column; gap: 4px;">
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <span style="color: var(--text-muted); font-size: 0.7rem;">App Link:</span>
            <span style="color: #a5b4fc; font-family: monospace; overflow: hidden; text-overflow: ellipsis; max-width: 140px; white-space: nowrap;">${runLink}</span>
            <i data-lucide="copy" class="copy-btn" data-copy="${runLink}" style="width: 12px; height: 12px; cursor: pointer; color: var(--text-muted);" title="Copy App Deep Link"></i>
          </div>
          ${createLink ? `
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <span style="color: var(--text-muted); font-size: 0.7rem;">Prompt Link:</span>
            <span style="color: #a5b4fc; font-family: monospace; overflow: hidden; text-overflow: ellipsis; max-width: 140px; white-space: nowrap;">prompt://${app.prompt.substring(0, 15)}...</span>
            <i data-lucide="copy" class="copy-btn" data-copy="${createLink}" style="width: 12px; height: 12px; cursor: pointer; color: var(--text-muted);" title="Copy Prompt Deep Link"></i>
          </div>` : ''}
        </div>
      </div>
      <div class="app-card-footer">
        <span class="app-card-meta" style="font-family: monospace; font-size: 0.75rem;">${name}</span>
        <div class="app-card-actions">
          <button class="btn btn-secondary btn-sm" data-action="publish" data-app="${name}" title="Publish App to Registry">
            <i data-lucide="cloud" style="width: 14px; height: 14px;"></i>
          </button>
          <button class="btn btn-secondary btn-sm" data-action="unlink" data-app="${name}" title="Unlink App">
            <i data-lucide="link-2-off" style="width: 14px; height: 14px;"></i>
          </button>
          <button class="btn btn-secondary btn-sm" data-action="browser" data-app="${name}" title="Open in default browser">
            <i data-lucide="external-link" style="width: 14px; height: 14px;"></i>
          </button>
          <button class="btn btn-primary btn-sm" data-action="run" data-app="${name}">
            Open App
          </button>
        </div>
      </div>
    `;

    // Event listener for click card
    card.addEventListener('click', (e) => {
      // Don't open if clicking action buttons
      if (e.target.closest('button')) return;
      openAppSession(name);
    });

    // Button event listeners
    card.querySelector('button[data-action="run"]').addEventListener('click', () => openAppSession(name));
    card.querySelector('button[data-action="publish"]').addEventListener('click', (e) => {
      e.stopPropagation();
      publishApp(name, e.currentTarget);
    });
    card.querySelector('button[data-action="unlink"]').addEventListener('click', (e) => {
      e.stopPropagation();
      handleUnlinkShortcut(name);
    });
    card.querySelector('button[data-action="browser"]').addEventListener('click', (e) => {
      e.stopPropagation();
      const url = `${API_BASE}/apps/${name}/`;
      window.electronAPI.openBrowser(url);
    });

    container.appendChild(card);
  });

  lucide.createIcons();
}

// Render active session links in sidebar
function renderActiveSessionSidebarLinks() {
  const container = document.getElementById('active-app-list');
  const header = document.getElementById('active-session-header');
  const sessionNames = Object.keys(activeSessions);

  if (sessionNames.length === 0) {
    header.style.display = 'none';
    container.innerHTML = '';
    return;
  }

  header.style.display = 'block';
  container.innerHTML = '';

  sessionNames.forEach(name => {
    const displayName = name.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
    const li = document.createElement('li');
    li.className = `active-app-item ${currentView === name ? 'active' : ''}`;
    li.setAttribute('data-app', name);
    li.textContent = displayName;
    li.addEventListener('click', () => openAppSession(name));
    container.appendChild(li);
  });
}

// Open App Preview and Editor Session
async function openAppSession(name) {
  const app = registeredApps[name];
  if (!app) return;

  activeSessions[name] = app;
  currentSessionName = name;

  // Reset browser visibility to visible
  const mainPane = document.querySelector('.session-main');
  const resizer = document.getElementById('session-resizer');
  const sidebar = document.querySelector('.session-sidebar');
  const label = document.getElementById('text-toggle-browser');
  const icon = document.getElementById('icon-toggle-browser');
  if (mainPane && resizer && sidebar) {
    mainPane.style.display = '';
    resizer.style.display = '';
    sidebar.classList.remove('expanded');
    if (label) label.textContent = 'Hide Browser';
    if (icon) {
      icon.setAttribute('data-lucide', 'eye-off');
      if (typeof lucide !== 'undefined') lucide.createIcons();
    }
  }
  
  // Render details
  const displayName = name.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  document.getElementById('session-app-title').textContent = displayName;
  document.getElementById('session-app-path').textContent = app.path;
  
  // Populate deep links panel
  document.getElementById('session-app-slug').textContent = name;
  const runLink = `prompt://${name}`;
  document.getElementById('session-run-link-preview').textContent = runLink;
  document.getElementById('btn-copy-session-run').setAttribute('data-copy', runLink);

  const createLinkRow = document.getElementById('session-create-link-row');
  if (app.prompt) {
    createLinkRow.style.display = 'flex';
    const createLink = `prompt://${encodeURIComponent(app.prompt)}`;
    document.getElementById('session-create-link-preview').textContent = `prompt://${app.prompt.substring(0, 15)}...`;
    document.getElementById('btn-copy-session-create').setAttribute('data-copy', createLink);
  } else {
    createLinkRow.style.display = 'none';
  }
  
  const previewURL = `${API_BASE}/apps/${name}/`;
  const addressBar = document.getElementById('session-address-bar');
  addressBar.textContent = previewURL;
  addressBar.setAttribute('data-copy', previewURL);
  document.getElementById('btn-copy-address').setAttribute('data-copy', previewURL);
  document.getElementById('app-preview-frame').src = previewURL;

  // Clear edit text area
  document.getElementById('session-edit-prompt').value = '';
  document.getElementById('edit-pulse').style.display = 'none';

  // Open view
  switchView(name);
  renderActiveSessionSidebarLinks();

  // Load history
  await loadAppHistory(name);
}

// Load history log file from app folder
async function loadAppHistory(name) {
  const container = document.getElementById('session-history-container');
  container.innerHTML = '<div style="font-size: 0.8rem; color: var(--text-muted); text-align: center; padding-top: 10px;">Loading history...</div>';

  try {
    // Fetch history.json which is exposed statically by server inside .promptyly
    const res = await fetch(`${API_BASE}/apps/${name}/.promptyly/history.json`);
    if (!res.ok) {
      container.innerHTML = '<div style="font-size: 0.8rem; color: var(--text-muted); text-align: center; padding-top: 10px;">No modifications made yet.</div>';
      return;
    }
    const history = await res.json();
    if (!history || history.length === 0) {
      container.innerHTML = '<div style="font-size: 0.8rem; color: var(--text-muted); text-align: center; padding-top: 10px;">No modifications made yet.</div>';
      return;
    }

    container.innerHTML = '';
    // Display in reverse order (most recent first)
    history.slice().reverse().forEach(entry => {
      const bubble = document.createElement('div');
      bubble.className = `chat-bubble ${entry.action === 'edit' ? 'user' : ''}`;
      
      const timeStr = new Date(entry.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      const actionTitle = entry.action === 'create' ? 'Initial Creation' : 'User Instruction';

      bubble.innerHTML = `
        <div class="chat-bubble-title">${actionTitle} • ${timeStr}</div>
        <div style="font-weight: 500; margin-bottom: 6px;">"${entry.prompt}"</div>
        <div style="font-size: 0.75rem; color: var(--text-secondary); line-height: 1.4; border-top: 1px dashed rgba(255,255,255,0.06); padding-top: 6px;">
          <strong>AI:</strong> ${entry.summary}
        </div>
      `;
      container.appendChild(bubble);
    });

  } catch (e) {
    container.innerHTML = '<div style="font-size: 0.8rem; color: var(--text-muted); text-align: center; padding-top: 10px;">No modifications made yet.</div>';
  }
}

// Link an existing app folder
async function handleLinkApp() {
  const path = await window.electronAPI.openDirectory();
  if (!path) return;

  try {
    const res = await apiFetch('/api/apps/link', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path })
    });
    
    const result = await res.json();
    if (res.ok && result.success) {
      await loadApps();
      openAppSession(result.appName);
    } else {
      alert(`Failed to link: ${result.error || 'Check if directory contains index.html'}`);
    }
  } catch (err) {
    alert(`Server error: ${err.message}`);
  }
}

// Create New App Form Submission
async function handleCreateApp() {
  const promptInput = document.getElementById('create-prompt');
  const prompt = promptInput.value.trim();
  if (!prompt) return;

  const btn = document.getElementById('btn-generate');
  const pulse = document.getElementById('generation-pulse');
  const consoleLog = document.getElementById('generation-console');
  const statusText = document.getElementById('generation-status-text');

  // Lock UI
  btn.disabled = true;
  promptInput.disabled = true;
  pulse.style.display = 'flex';
  consoleLog.style.display = 'block';
  consoleLog.innerHTML = `<div class="log-entry">> Initializing generation request...</div>`;

  // Start fake logger interval for better feedback while waiting for API
  let step = 0;
  const logSteps = [
    { text: 'Setting up Git repository in promptyly-apps directory...', color: '' },
    { text: 'Connecting to AI model endpoint...', color: '' },
    { text: 'AI Agent is creating code architecture (HTML/CSS/JS)...', color: '' },
    { text: 'Analyzing instructions, writing database hooks...', color: '' },
    { text: 'Compiling assets and validating markup...', color: '' }
  ];

  const logInterval = setInterval(() => {
    if (step < logSteps.length) {
      statusText.textContent = logSteps[step].text;
      const entry = document.createElement('div');
      entry.className = `log-entry ${logSteps[step].color}`;
      entry.textContent = `> ${logSteps[step].text}`;
      consoleLog.appendChild(entry);
      consoleLog.scrollTop = consoleLog.scrollHeight;
      step++;
    }
  }, 3000);

  try {
    const res = await apiFetch('/api/apps/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt })
    });

    clearInterval(logInterval);

    if (res.ok) {
      const result = await res.json();
      const entry = document.createElement('div');
      entry.className = 'log-entry log-success';
      entry.textContent = `> Success! App created as: ${result.appName}`;
      consoleLog.appendChild(entry);

      setTimeout(async () => {
        // Unlock
        btn.disabled = false;
        promptInput.disabled = false;
        promptInput.value = '';
        pulse.style.display = 'none';
        consoleLog.style.display = 'none';

        await loadApps();
        openAppSession(result.appName);
        const url = `${API_BASE}/apps/${result.appName}/`;
        window.electronAPI.openBrowser(url);
      }, 1000);
    } else {
      const errText = await res.text();
      throw new Error(errText);
    }
  } catch (err) {
    clearInterval(logInterval);
    btn.disabled = false;
    promptInput.disabled = false;
    pulse.style.display = 'none';
    
    const entry = document.createElement('div');
    entry.className = 'log-entry log-error';
    entry.textContent = `> Generation failed: ${err.message}`;
    consoleLog.appendChild(entry);
  }
}

// Modify/Edit Existing App inside Session
async function handleSessionEdit() {
  if (!currentSessionName) return;

  const promptInput = document.getElementById('session-edit-prompt');
  const prompt = promptInput.value.trim();
  if (!prompt) return;

  const btn = document.getElementById('btn-session-update');
  const pulse = document.getElementById('edit-pulse');

  btn.disabled = true;
  promptInput.disabled = true;
  pulse.style.display = 'flex';

  try {
    const res = await apiFetch('/api/apps/edit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: currentSessionName,
        prompt: prompt
      })
    });

    if (res.ok) {
      promptInput.value = '';
      // Reload frame
      handleSessionReload();
      // Reload history
      await loadAppHistory(currentSessionName);
    } else {
      const errText = await res.text();
      alert(`Failed to apply changes: ${errText}`);
    }
  } catch (err) {
    alert(`Request error: ${err.message}`);
  } finally {
    btn.disabled = false;
    promptInput.disabled = false;
    pulse.style.display = 'none';
  }
}

// Reload Frame preview
function handleSessionReload() {
  const frame = document.getElementById('app-preview-frame');
  frame.src = frame.src; // Refreshes iframe
}

// Open App preview in external default OS Browser
function handleSessionOpenBrowser() {
  if (!currentSessionName) return;
  const url = `${API_BASE}/apps/${currentSessionName}/`;
  window.electronAPI.openBrowser(url);
}

// Open project folder in file manager
async function handleSessionOpenFolder() {
  if (!currentSessionName) return;
  const app = activeSessions[currentSessionName];
  if (app && app.path) {
    await window.electronAPI.openPath(app.path);
  }
}

// Toggle visibility of the browser preview pane
function handleSessionToggleBrowser() {
  const mainPane = document.querySelector('.session-main');
  const resizer = document.getElementById('session-resizer');
  const sidebar = document.querySelector('.session-sidebar');
  const label = document.getElementById('text-toggle-browser');
  const icon = document.getElementById('icon-toggle-browser');

  if (!mainPane || !resizer || !sidebar) return;

  const isHidden = mainPane.style.display === 'none';

  if (isHidden) {
    // Show browser
    mainPane.style.display = '';
    resizer.style.display = '';
    sidebar.classList.remove('expanded');
    
    if (label) label.textContent = 'Hide Browser';
    if (icon) {
      icon.setAttribute('data-lucide', 'eye-off');
      lucide.createIcons();
    }
  } else {
    // Hide browser
    mainPane.style.display = 'none';
    resizer.style.display = 'none';
    sidebar.classList.add('expanded');
    
    if (label) label.textContent = 'Show Browser';
    if (icon) {
      icon.setAttribute('data-lucide', 'eye');
      lucide.createIcons();
    }
  }
}

// Export app as ZIP archive
async function handleSessionExport() {
  if (!currentSessionName) return;

  const destPath = await window.electronAPI.saveFileDialog(currentSessionName);
  if (!destPath) return;

  try {
    const res = await apiFetch('/api/apps/export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: currentSessionName,
        zipPath: destPath
      })
    });

    if (res.ok) {
      alert(`Successfully exported ZIP archive to:\n${destPath}`);
    } else {
      const errText = await res.text();
      alert(`Export failed: ${errText}`);
    }
  } catch (err) {
    alert(`Request failed: ${err.message}`);
  }
}

// Publish active session app to registry
async function handleSessionPublish(e) {
  if (!currentSessionName) return;
  await publishApp(currentSessionName, e.currentTarget);
}

// Publish app helper function
async function publishApp(name, btnElement = null) {
  const desc = prompt("Enter an optional description for the sharing registry:", "");
  if (desc === null) return; // cancelled

  let originalHTML = '';
  if (btnElement) {
    originalHTML = btnElement.innerHTML;
    btnElement.disabled = true;
    btnElement.innerHTML = '...';
  }

  try {
    const res = await apiFetch('/api/apps/publish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name, description: desc.trim() })
    });

    if (res.ok) {
      const data = await res.json();
      alert(`Success! Application successfully published to the registry!\n\nLive URL: ${data.liveUrl}\nDetail Page: ${data.detailUrl}`);
    } else {
      const txt = await res.text();
      alert(`Publish failed: ${txt}`);
    }
  } catch (err) {
    alert(`Publish error: ${err.message}`);
  } finally {
    if (btnElement) {
      btnElement.disabled = false;
      btnElement.innerHTML = originalHTML;
    }
  }
}

// Update Deep Link URL previews in modal dynamically
function updateRenamePreviews() {
  const nameInput = document.getElementById('rename-input-name').value.trim();
  // slugify client-side for visual preview
  const slugName = nameInput.toLowerCase()
    .replace(/[^a-z0-9\s-_]/g, '')
    .trim()
    .replace(/[\s-_]+/g, '-');
    
  const promptText = document.getElementById('rename-input-prompt').value.trim();
  
  document.getElementById('preview-run-url').textContent = slugName ? `prompt://run?name=${slugName}` : 'prompt://run?name=...';
  document.getElementById('preview-create-url').textContent = promptText ? `prompt://create?prompt=${encodeURIComponent(promptText)}` : 'prompt://create?prompt=...';
}

// Rename/Edit Details app trigger
function handleSessionRenameTrigger() {
  if (!currentSessionName) return;
  const app = activeSessions[currentSessionName] || {};
  
  document.getElementById('rename-input-name').value = currentSessionName;
  document.getElementById('rename-input-prompt').value = app.prompt || '';
  
  updateRenamePreviews();
  openModal('modal-rename');
}

async function handleRenameConfirm() {
  const newName = document.getElementById('rename-input-name').value.trim();
  const newPrompt = document.getElementById('rename-input-prompt').value.trim();
  
  if (!newName) {
    alert('Application name/slug cannot be empty.');
    return;
  }

  closeModal('modal-rename');

  try {
    const res = await apiFetch('/api/apps/update-metadata', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: currentSessionName,
        newName: newName,
        newPrompt: newPrompt
      })
    });

    const result = await res.json();
    if (res.ok && result.success) {
      // Clear old session
      delete activeSessions[currentSessionName];
      
      // Update dashboard registry list
      await loadApps();
      
      // Reopen preview with new slug name
      openAppSession(result.appName);
    } else {
      alert(`Update failed: ${result.error || 'Unknown error'}`);
    }
  } catch (err) {
    alert(`Update failed: ${err.message}`);
  }
}


// Delete app trigger
function handleSessionDeleteTrigger() {
  if (!currentSessionName) return;
  const app = activeSessions[currentSessionName];
  const displayName = currentSessionName.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  
  document.getElementById('delete-app-display-name').textContent = displayName;
  document.getElementById('delete-app-path-label').textContent = app.path;
  document.getElementById('delete-disk-folder').checked = false;
  
  openModal('modal-delete');
}

// Handle unlink directly from Hub dashboard card
function handleUnlinkShortcut(name) {
  if (!name) return;
  const displayName = name.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  
  const confirmUnlink = confirm(`Are you sure you want to remove '${displayName}' from your Dashboard registry? (Your files on disk will NOT be deleted)`);
  if (!confirmUnlink) return;

  performDeletion(name, false);
}

async function handleDeleteConfirm() {
  const deleteFolder = document.getElementById('delete-disk-folder').checked;
  closeModal('modal-delete');
  await performDeletion(currentSessionName, deleteFolder);
}

async function performDeletion(name, deleteFolder) {
  try {
    const res = await apiFetch('/api/apps/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: name,
        deleteFolder: deleteFolder
      })
    });

    if (res.ok) {
      // Clear sessions
      delete activeSessions[name];
      if (currentSessionName === name) {
        currentSessionName = null;
        document.getElementById('app-preview-frame').src = '';
      }
      
      await loadApps();
      switchView('hub');
    } else {
      const errText = await res.text();
      alert(`Deletion failed: ${errText}`);
    }
  } catch (err) {
    alert(`Request error: ${err.message}`);
  }
}

// Configuration Section
async function loadConfig() {
  try {
    const res = await apiFetch('/api/config');
    if (!res.ok) throw new Error('Config not loaded');
    currentConfig = await res.json();
    
    // Fill apps directory input
    document.getElementById('cfg-apps-dir').value = currentConfig.apps_dir || '';
    
    // Fill sharing settings inputs
    document.getElementById('cfg-sharing-url').value = currentConfig.sharing_server_url || '';
    document.getElementById('cfg-sharing-token').value = currentConfig.sharing_token || '';
    document.getElementById('cfg-check-remote').checked = !!currentConfig.check_remote_first;
    
    // Select default provider tab
    selectedProvider = currentConfig.default_provider || 'gemini';
    document.querySelectorAll('.provider-tab').forEach(tab => {
      tab.classList.remove('active');
      if (tab.getAttribute('data-provider') === selectedProvider) {
        tab.classList.add('active');
      }
    });

    renderProviderFields();
    updateActiveModelUIInfo();
  } catch (e) {
    // Fail silently or alert
  }
}

function renderProviderFields() {
  if (!currentConfig) return;

  const pCfg = currentConfig.providers[selectedProvider] || {};
  const apiKeyInput = document.getElementById('cfg-api-key');
  const urlInput = document.getElementById('cfg-endpoint-url');
  const modelInput = document.getElementById('cfg-model');

  apiKeyInput.value = pCfg.api_key || '';
  urlInput.value = pCfg.url || '';
  modelInput.value = pCfg.model || '';

  // Show/Hide inputs depending on local or cloud provider
  const urlGroup = document.getElementById('form-url-group');
  const keyGroup = document.getElementById('form-key-group');
  const lblKey = document.getElementById('lbl-api-key');

  if (selectedProvider === 'ollama' || selectedProvider === 'lmstudio') {
    urlGroup.style.display = 'block';
    keyGroup.style.display = 'none';
    
    // Set default endpoints if empty
    if (!urlInput.value) {
      urlInput.value = selectedProvider === 'ollama' ? 'http://localhost:11434' : 'http://localhost:1234/v1';
    }
  } else {
    urlGroup.style.display = 'none';
    keyGroup.style.display = 'block';
    lblKey.textContent = selectedProvider === 'gemini' ? 'Gemini API Key' : 'Claude API Key';
  }
}

// Update settings info in other views
function updateActiveModelUIInfo() {
  if (!currentConfig) return;
  const activeProv = currentConfig.default_provider || 'gemini';
  const pCfg = currentConfig.providers[activeProv] || {};
  
  const span = document.getElementById('active-model-info');
  if (span) {
    span.innerHTML = `Active Provider: <strong style="text-transform: capitalize;">${activeProv}</strong> (${pCfg.model || 'Default Model'})`;
  }
}

// Save AI Config Form
async function handleSaveAIConfig() {
  if (!currentConfig) return;

  const apiKey = document.getElementById('cfg-api-key').value.trim();
  const url = document.getElementById('cfg-endpoint-url').value.trim();
  const model = document.getElementById('cfg-model').value.trim();

  // Update current provider values
  currentConfig.default_provider = selectedProvider;
  currentConfig.providers[selectedProvider] = {
    api_key: apiKey,
    url: url,
    model: model
  };

  try {
    const res = await apiFetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(currentConfig)
    });

    if (res.ok) {
      alert('AI Configuration saved successfully!');
      await loadConfig();
    } else {
      alert('Failed to save AI config.');
    }
  } catch (err) {
    alert(`Save error: ${err.message}`);
  }
}

// Save System Config (Apps Folder and Background Tray)
async function handleSaveSystemConfig() {
  if (!currentConfig) return;

  const appsDir = document.getElementById('cfg-apps-dir').value.trim();
  const runBackground = document.getElementById('cfg-run-background').checked;

  currentConfig.apps_dir = appsDir;

  try {
    // Save directory to Go config
    const res = await apiFetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(currentConfig)
    });

    // Save background execution setting in Electron
    await window.electronAPI.setBackground(runBackground);

    if (res.ok) {
      alert('System settings updated successfully!');
      await loadConfig();
      await loadApps();
    } else {
      alert('Failed to save system config.');
    }
  } catch (err) {
    alert(`Save error: ${err.message}`);
  }
}

// Browse Directory Picker for storage path
async function handleBrowseAppsDir() {
  const path = await window.electronAPI.openDirectory();
  if (path) {
    document.getElementById('cfg-apps-dir').value = path;
  }
}

// Protocol scheme status check
async function checkProtocolStatus() {
  const statusBadge = document.getElementById('protocol-status-badge');
  const registerBtn = document.getElementById('btn-register-protocol');

  const registered = await window.electronAPI.checkProtocol();
  if (registered) {
    statusBadge.textContent = 'ASSOCIATED';
    statusBadge.style.backgroundColor = 'rgba(16, 185, 129, 0.15)';
    statusBadge.style.color = '#34d399';
    registerBtn.disabled = true;
    registerBtn.textContent = 'Registered';
  } else {
    statusBadge.textContent = 'NOT REGISTERED';
    statusBadge.style.backgroundColor = 'rgba(245, 158, 11, 0.15)';
    statusBadge.style.color = '#fbbf24';
    registerBtn.disabled = false;
    registerBtn.textContent = 'Register Now';
  }
}

// Trigger protocol scheme association
async function handleRegisterProtocol() {
  const statusBadge = document.getElementById('protocol-status-badge');
  statusBadge.textContent = 'Registering...';

  const result = await window.electronAPI.registerProtocol();
  if (result.success) {
    alert(`Protocol scheme association completed:\n${result.message}`);
    await checkProtocolStatus();
  } else {
    alert(`Registration failed:\n${result.error}`);
    await checkProtocolStatus();
  }
}

// Load background execution checkbox state
async function loadBackgroundSetting() {
  const enabled = await window.electronAPI.getBackground();
  document.getElementById('cfg-run-background').checked = enabled;
}

// Respond to deep links e.g. prompt://create?prompt=... or prompt://run?name=...
function handleProtocolURL(urlString) {
  try {
    if (!urlString.startsWith('prompt://')) return;
    
    let targetVal = '';
    const payload = urlString.substring('prompt://'.length);
    
    // Legacy query fallback compatibility
    if (payload.includes('?')) {
      const url = new URL(urlString);
      const action = url.host;
      if (action === 'create') {
        targetVal = url.searchParams.get('prompt');
      } else if (action === 'run' || action === 'open') {
        targetVal = url.searchParams.get('name');
      }
    }
    
    if (!targetVal) {
      targetVal = decodeURIComponent(payload.replace(/\+/g, ' ')).trim();
    }
    
    if (!targetVal) return;
    
    // Check if it exists in the registry
    let matchedName = null;
    const slugName = targetVal.toLowerCase()
      .replace(/[^a-z0-9\s-_]/g, '')
      .trim()
      .replace(/[\s-_]+/g, '-');
      
    if (registeredApps[targetVal]) {
      matchedName = targetVal;
    } else if (registeredApps[slugName]) {
      matchedName = slugName;
    } else {
      // Check prompts matching
      for (const name in registeredApps) {
        if (registeredApps[name].prompt && registeredApps[name].prompt.toLowerCase() === targetVal.toLowerCase()) {
          matchedName = name;
          break;
        }
      }
    }
    
    if (matchedName) {
      // Run the app session
      console.log(`Deep Link: App exists. Running '${matchedName}'`);
      openAppSession(matchedName);
      
      // Launch external browser automatically
      const url = `${API_BASE}/apps/${matchedName}/`;
      window.electronAPI.openBrowser(url);
    } else {
      // Prefill wizard to create
      console.log(`Deep Link: App doesn't exist. Prefilling prompt: '${targetVal}'`);
      switchView('create');
      document.getElementById('create-prompt').value = targetVal;
      document.getElementById('create-prompt').focus();
    }
  } catch (err) {
    console.error('Error handling deep link URL:', err);
  }
}

// Modal helper controls
function openModal(id) {
  document.getElementById(id).style.display = 'flex';
}

function closeModal(id) {
  document.getElementById(id).style.display = 'none';
}

// Sharing Server Config Save
async function handleSaveSharingConfig() {
  if (!currentConfig) return;

  const sharingUrl = document.getElementById('cfg-sharing-url').value.trim();
  const sharingToken = document.getElementById('cfg-sharing-token').value.trim();
  const checkRemote = document.getElementById('cfg-check-remote').checked;

  currentConfig.sharing_server_url = sharingUrl;
  currentConfig.sharing_token = sharingToken;
  currentConfig.check_remote_first = checkRemote;

  try {
    const res = await apiFetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(currentConfig)
    });

    if (res.ok) {
      alert('Sharing registry settings updated successfully!');
      await loadConfig();
    } else {
      alert('Failed to save sharing settings.');
    }
  } catch (err) {
    alert(`Save error: ${err.message}`);
  }
}

// Remote Registry Search
async function handleRemoteSearch() {
  const container = document.getElementById('remote-apps-container');
  const query = document.getElementById('remote-search-input').value.trim();
  const serverUrl = (currentConfig && currentConfig.sharing_server_url) ? currentConfig.sharing_server_url : 'http://localhost:6072';

  container.innerHTML = `
    <div class="empty-state" style="grid-column: 1 / -1;">
      <div class="spinner" style="width: 28px; height: 28px; border: 2.5px solid rgba(255,255,255,0.1); border-top-color: var(--accent-color); border-radius: 50%; animation: spin 1s linear infinite; margin-bottom: 12px; margin-left: auto; margin-right: auto;"></div>
      <h3>Querying Remote Registry...</h3>
      <p>Searching matching apps on ${serverUrl}...</p>
    </div>
  `;

  try {
    const res = await fetch(`${serverUrl}/api/apps/search?q=${encodeURIComponent(query)}`);
    if (!res.ok) throw new Error('Remote registry returned error status');

    const apps = await res.json();
    renderRemoteApps(apps);
  } catch (err) {
    container.innerHTML = `
      <div class="empty-state" style="grid-column: 1 / -1;">
        <i data-lucide="cloud-off" style="width: 48px; height: 48px; color: var(--error-color); margin-bottom: 15px; margin-left: auto; margin-right: auto; display: block;"></i>
        <h3 style="color: var(--error-color);">Registry Connection Failed</h3>
        <p>Could not connect to sharing server at <code>${serverUrl}</code>.</p>
        <p style="font-size: 0.85rem; color: var(--text-muted); margin-top: 10px;">Verify the registry server is running and your URL is configured correctly.</p>
      </div>
    `;
    lucide.createIcons();
  }
}

// Render Remote Search Results
function renderRemoteApps(apps) {
  const container = document.getElementById('remote-apps-container');

  if (apps.length === 0) {
    container.innerHTML = `
      <div class="empty-state" style="grid-column: 1 / -1;">
        <i data-lucide="info" style="width: 48px; height: 48px; color: var(--text-muted); margin-bottom: 15px; margin-left: auto; margin-right: auto; display: block;"></i>
        <h3>No applications found</h3>
        <p>Try searching another keyword or create a new prompt template.</p>
      </div>
    `;
    lucide.createIcons();
    return;
  }

  container.innerHTML = '';
  apps.forEach(app => {
    const card = document.createElement('div');
    card.className = 'app-card';

    const viewsText = app.views === 1 ? '1 view' : `${app.views} views`;
    const downloadsText = app.downloads === 1 ? '1 download' : `${app.downloads} downloads`;

    card.innerHTML = `
      <div class="app-card-top">
        <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 8px; gap: 8px;">
          <h3 class="app-card-title" title="${app.name}">${app.name}</h3>
          <span style="font-size: 0.7rem; background: rgba(99, 102, 241, 0.1); color: #a5b4fc; padding: 2px 6px; border-radius: 4px; font-weight: 600; white-space: nowrap;">By ${app.username}</span>
        </div>
        <p class="app-card-desc" style="margin-bottom: 12px;">${app.prompt}</p>
        
        <div style="font-size: 0.72rem; color: var(--text-muted); display: flex; gap: 8px; margin-top: 8px;">
          <span>${viewsText}</span>
          <span>&bull;</span>
          <span>${downloadsText}</span>
        </div>
      </div>
      <div class="app-card-footer">
        <span class="app-card-meta" style="font-family: monospace; font-size: 0.75rem;">${app.id}</span>
        <div class="app-card-actions">
          <button class="btn btn-secondary btn-sm" data-action="remote-view" title="Launch live app">
            <i data-lucide="external-link" style="width: 14px; height: 14px;"></i>
          </button>
          <button class="btn btn-primary btn-sm" data-action="remote-download">
            Install App
          </button>
        </div>
      </div>
    `;

    // Trigger buttons
    card.querySelector('[data-action="remote-view"]').addEventListener('click', (e) => {
      e.stopPropagation();
      const serverUrl = (currentConfig && currentConfig.sharing_server_url) ? currentConfig.sharing_server_url : 'http://localhost:6072';
      window.open(`${serverUrl}/apps/${app.id}/`, '_blank');
    });

    const installBtn = card.querySelector('[data-action="remote-download"]');
    installBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      installBtn.disabled = true;
      installBtn.innerHTML = `
        <div class="spinner" style="width: 12px; height: 12px; border: 1.5px solid rgba(255,255,255,0.1); border-top-color: white; border-radius: 50%; animation: spin 1s linear infinite; display: inline-block; vertical-align: middle; margin-right: 4px;"></div>
        Installing...
      `;

      try {
        const res = await apiFetch('/api/apps/download', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ appId: app.id })
        });

        if (res.ok) {
          alert(`Application '${app.name}' has been downloaded and installed locally!`);
          installBtn.textContent = 'Installed';
          await loadApps();
        } else {
          const errText = await res.text();
          alert(`Installation failed: ${errText}`);
          installBtn.disabled = false;
          installBtn.textContent = 'Install App';
        }
      } catch (err) {
        alert(`Installation error: ${err.message}`);
        installBtn.disabled = false;
        installBtn.textContent = 'Install App';
      }
    });

    container.appendChild(card);
  });

  lucide.createIcons();
}

// Initialize on document load
document.addEventListener('DOMContentLoaded', init);
