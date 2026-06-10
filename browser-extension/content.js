(function() {
  // Prevent duplicate injection
  if (window.PromptylyCompanionInjected) return;
  window.PromptylyCompanionInjected = true;

  const DEFAULT_SETTINGS = {
    daemonUrl: 'http://localhost:6071',
    daemonToken: '',
    registryUrl: 'http://localhost:6072',
    interceptLinks: true
  };

  // Intercept click events
  document.addEventListener('click', (e) => {
    chrome.storage.local.get(DEFAULT_SETTINGS, (settings) => {
      if (!settings.interceptLinks) return;

      const anchor = e.target.closest('a');
      if (!anchor) return;

      const href = anchor.getAttribute('href');
      if (href && href.startsWith('prompt://')) {
        e.preventDefault();
        e.stopPropagation();
        
        const rawUri = href.substring(9); // strip prompt://
        showOverlay(rawUri, settings);
      }
    });
  }, true);

  // Injects styling for the modal overlay
  function injectStyles() {
    if (document.getElementById('promptyly-extension-styles')) return;

    const style = document.createElement('style');
    style.id = 'promptyly-extension-styles';
    style.textContent = `
      #promptyly-overlay-container {
        position: fixed;
        top: 0;
        left: 0;
        width: 100vw;
        height: 100vh;
        background: rgba(4, 6, 10, 0.75);
        backdrop-filter: blur(8px);
        z-index: 99999999;
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'Plus Jakarta Sans', -apple-system, sans-serif;
        color: #f8fafc;
      }

      .promptyly-modal {
        background: #0f172a;
        border: 1px solid rgba(255, 255, 255, 0.08);
        border-radius: 16px;
        width: 480px;
        max-width: 90%;
        padding: 28px;
        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
        display: flex;
        flex-direction: column;
        gap: 20px;
        animation: promptyly-fade-in 0.25s ease-out;
      }

      @keyframes promptyly-fade-in {
        from { opacity: 0; transform: scale(0.95); }
        to { opacity: 1; transform: scale(1); }
      }

      .promptyly-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
        padding-bottom: 14px;
      }

      .promptyly-logo-title {
        display: flex;
        align-items: center;
        gap: 10px;
      }

      .promptyly-logo {
        width: 32px;
        height: 32px;
        background: linear-gradient(135deg, #a5b4fc 0%, #6366f1 100%);
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 800;
        font-size: 1.25rem;
        color: white;
        box-shadow: 0 0 15px rgba(99, 102, 241, 0.25);
      }

      .promptyly-title {
        font-size: 1.2rem;
        font-weight: 700;
        margin: 0;
        background: linear-gradient(135deg, #f8fafc 30%, #94a3b8 100%);
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      }

      .promptyly-close {
        background: transparent;
        border: none;
        color: #94a3b8;
        font-size: 1.5rem;
        cursor: pointer;
        padding: 0;
        line-height: 1;
        transition: color 0.2s;
      }

      .promptyly-close:hover {
        color: #f8fafc;
      }

      .promptyly-section {
        display: flex;
        flex-direction: column;
        gap: 6px;
      }

      .promptyly-label {
        font-size: 0.72rem;
        font-weight: 700;
        color: #64748b;
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .promptyly-val {
        font-size: 0.95rem;
        color: #e2e8f0;
        line-height: 1.5;
      }

      .promptyly-badge {
        display: inline-block;
        padding: 4px 10px;
        border-radius: 9999px;
        font-size: 0.75rem;
        font-weight: 600;
        width: fit-content;
      }

      .promptyly-badge-success {
        background: rgba(16, 185, 129, 0.15);
        color: #34d399;
        border: 1px solid rgba(16, 185, 129, 0.25);
      }

      .promptyly-badge-warn {
        background: rgba(239, 68, 68, 0.15);
        color: #f87171;
        border: 1px solid rgba(239, 68, 68, 0.25);
      }

      .promptyly-badge-neutral {
        background: rgba(255, 255, 255, 0.05);
        color: #94a3b8;
        border: 1px solid rgba(255, 255, 255, 0.08);
      }

      .promptyly-buttons {
        display: flex;
        gap: 12px;
        margin-top: 10px;
      }

      .promptyly-btn {
        flex: 1;
        padding: 12px;
        border-radius: 8px;
        font-weight: 600;
        font-size: 0.9rem;
        cursor: pointer;
        border: none;
        transition: all 0.2s;
        text-align: center;
        text-decoration: none;
      }

      .promptyly-btn-primary {
        background: linear-gradient(135deg, #a5b4fc 0%, #6366f1 100%);
        color: white;
        box-shadow: 0 4px 12px rgba(99, 102, 241, 0.2);
      }

      .promptyly-btn-primary:hover:not(:disabled) {
        box-shadow: 0 4px 20px rgba(99, 102, 241, 0.35);
        transform: translateY(-0.5px);
      }

      .promptyly-btn-secondary {
        background: rgba(255, 255, 255, 0.03);
        border: 1px solid rgba(255, 255, 255, 0.08);
        color: #f8fafc;
      }

      .promptyly-btn-secondary:hover:not(:disabled) {
        background: rgba(255, 255, 255, 0.06);
      }

      .promptyly-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }

      .promptyly-spinner {
        width: 20px;
        height: 20px;
        border: 2px solid rgba(99, 102, 241, 0.2);
        border-top-color: #6366f1;
        border-radius: 50%;
        animation: promptyly-spin 1s linear infinite;
        margin: 10px auto;
      }

      @keyframes promptyly-spin {
        to { transform: rotate(360deg); }
      }
    `;
    document.head.appendChild(style);
  }

  // Helper to call local daemon APIs
  async function daemonFetch(endpoint, settings, method = 'GET', body = null) {
    const headers = {
      'Content-Type': 'application/json',
      'X-Promptyly-Token': settings.daemonToken
    };
    const options = { method, headers };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const res = await fetch(`${settings.daemonUrl}${endpoint}`, options);
    return res;
  }

  // Shows the overlay modal
  async function showOverlay(rawUri, settings) {
    injectStyles();

    // Close any existing modals
    const oldOverlay = document.getElementById('promptyly-overlay-container');
    if (oldOverlay) oldOverlay.remove();

    const overlay = document.createElement('div');
    overlay.id = 'promptyly-overlay-container';
    
    const decodedUri = decodeURIComponent(rawUri);
    const isNewPrompt = decodedUri.includes(' ') || decodedUri.length > 50;

    overlay.innerHTML = `
      <div class="promptyly-modal" id="promptyly-modal-body">
        <div class="promptyly-header">
          <div class="promptyly-logo-title">
            <div class="promptyly-logo">P</div>
            <h2 class="promptyly-title">Promptyly Link Interceptor</h2>
          </div>
          <button class="promptyly-close" id="promptyly-btn-close">&times;</button>
        </div>
        
        <div id="promptyly-modal-content">
          <div class="promptyly-spinner"></div>
          <p style="text-align: center; color: #94a3b8; font-size: 0.9rem; margin-top: 8px;">Connecting to local daemon & registry...</p>
        </div>
      </div>
    `;

    document.body.appendChild(overlay);

    // Event bindings for closing
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) overlay.remove();
    });
    document.getElementById('promptyly-btn-close').addEventListener('click', () => {
      overlay.remove();
    });

    const modalContent = document.getElementById('promptyly-modal-content');

    try {
      // 1. Check if local daemon is running & load installed apps
      let daemonRunning = false;
      let installedApps = {};
      try {
        const daemonRes = await daemonFetch('/api/apps', settings);
        if (daemonRes.ok) {
          installedApps = await daemonRes.json();
          daemonRunning = true;
        }
      } catch (err) {
        daemonRunning = false;
      }

      // 2. Fetch app details depending on type
      if (isNewPrompt) {
        // This is a direct prompt link (e.g. prompt://Build%20a%20sleek%20weather%20widget...)
        renderNewPromptView(modalContent, decodedUri, daemonRunning, settings);
      } else {
        // This is a named app slug (e.g. prompt://neon-calculator)
        const appName = decodedUri;
        const localApp = installedApps[appName];

        if (localApp) {
          // Already installed locally
          renderInstalledView(modalContent, appName, localApp, settings);
        } else {
          // Not installed locally, check remote registry server
          await checkRegistryAndRender(modalContent, appName, daemonRunning, settings);
        }
      }
    } catch (err) {
      modalContent.innerHTML = `
        <div class="promptyly-section">
          <span class="promptyly-badge promptyly-badge-warn">Daemon Error</span>
          <p style="margin-top: 10px; font-size: 0.9rem; line-height: 1.5; color: #cbd5e1;">
            Failed to communicate with your local Promptyly daemon. Ensure it is running by executing <code>promptyly run</code> or opening the Desktop application.
          </p>
        </div>
      `;
    }
  }

  // Render when app is already installed locally
  function renderInstalledView(container, name, appInfo, settings) {
    const formattedName = name.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
    container.innerHTML = `
      <div style="display: flex; flex-direction: column; gap: 16px;">
        <div class="promptyly-section">
          <span class="promptyly-badge promptyly-badge-success">Installed Locally</span>
        </div>
        <div class="promptyly-section">
          <span class="promptyly-label">App Name</span>
          <div class="promptyly-val" style="font-weight: 700; font-size: 1.1rem; color: #fff;">${formattedName}</div>
        </div>
        <div class="promptyly-section">
          <span class="promptyly-label">Original Generation Prompt</span>
          <div class="promptyly-val" style="font-style: italic; color: #94a3b8; font-size: 0.85rem; max-height: 80px; overflow-y: auto; background: rgba(0,0,0,0.15); padding: 8px; border-radius: 6px;">
            "${appInfo.prompt || 'Generated web application.'}"
          </div>
        </div>
        <div class="promptyly-buttons">
          <button class="promptyly-btn promptyly-btn-primary" id="btn-run-app">Open Application</button>
        </div>
      </div>
    `;

    document.getElementById('btn-run-app').addEventListener('click', () => {
      window.open(`${settings.daemonUrl}/apps/${name}/`, '_blank');
      document.getElementById('promptyly-overlay-container').remove();
    });
  }

  // Render for an unknown prompt (generation request)
  function renderNewPromptView(container, promptText, daemonRunning, settings) {
    container.innerHTML = `
      <div style="display: flex; flex-direction: column; gap: 16px;">
        <div class="promptyly-section">
          <span class="promptyly-badge promptyly-badge-neutral">New Application Prompt</span>
        </div>
        <div class="promptyly-section">
          <span class="promptyly-label">Prompt Description</span>
          <div class="promptyly-val" style="font-style: italic; background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; border: 1px solid rgba(255,255,255,0.03); max-height: 140px; overflow-y: auto;">
            "${promptText}"
          </div>
        </div>
        
        ${!daemonRunning ? `
          <div class="promptyly-badge promptyly-badge-warn" style="width: 100%; text-align: center; padding: 10px;">
            Daemon Offline - Run "promptyly run" to generate
          </div>
        ` : ''}

        <div class="promptyly-buttons">
          <button class="promptyly-btn promptyly-btn-primary" id="btn-gen-app" ${!daemonRunning ? 'disabled' : ''}>
            Generate Locally
          </button>
        </div>
      </div>
    `;

    if (daemonRunning) {
      document.getElementById('btn-gen-app').addEventListener('click', async (e) => {
        const btn = e.target;
        btn.disabled = true;
        btn.textContent = 'Generating Application (15-30s)...';
        
        try {
          const res = await daemonFetch('/api/apps/create', settings, 'POST', { prompt: promptText });
          if (res.ok) {
            const data = await res.json();
            btn.textContent = 'Success! Opening...';
            btn.style.background = '#10b981';
            setTimeout(() => {
              window.open(`${settings.daemonUrl}/apps/${data.appName}/`, '_blank');
              document.getElementById('promptyly-overlay-container').remove();
            }, 1000);
          } else {
            const txt = await res.text();
            alert('Generation failed: ' + txt);
            btn.disabled = false;
            btn.textContent = 'Generate Locally';
          }
        } catch (err) {
          alert('Generation request failed: ' + err.message);
          btn.disabled = false;
          btn.textContent = 'Generate Locally';
        }
      });
    }
  }

  // Check remote registry and render download option
  async function checkRegistryAndRender(container, appName, daemonRunning, settings) {
    try {
      const searchRes = await fetch(`${settings.registryUrl}/api/apps/search?q=${encodeURIComponent(appName)}`);
      if (!searchRes.ok) throw new Error('Registry search failed');
      
      const apps = await searchRes.json();
      // Look for exact slug name match
      const matchingApp = apps.find(a => a.id === appName || a.name === appName);

      if (matchingApp) {
        const formattedName = matchingApp.name.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
        const viewsText = matchingApp.views === 1 ? '1 view' : matchingApp.views + ' views';
        const downloadsText = matchingApp.downloads === 1 ? '1 download' : matchingApp.downloads + ' downloads';

        container.innerHTML = `
          <div style="display: flex; flex-direction: column; gap: 16px;">
            <div class="promptyly-section" style="flex-direction: row; justify-content: space-between; align-items: center;">
              <span class="promptyly-badge promptyly-badge-neutral">Registry App Available</span>
              <span style="font-size: 0.75rem; color: #64748b;">by ${matchingApp.username}</span>
            </div>
            
            <div class="promptyly-section">
              <span class="promptyly-label">App Name</span>
              <div class="promptyly-val" style="font-weight: 700; font-size: 1.1rem; color: #fff;">${formattedName}</div>
            </div>

            <div class="promptyly-section">
              <span class="promptyly-label">Original Prompt</span>
              <div class="promptyly-val" style="font-style: italic; color: #94a3b8; font-size: 0.85rem; max-height: 80px; overflow-y: auto; background: rgba(0,0,0,0.15); padding: 8px; border-radius: 6px;">
                "${matchingApp.prompt}"
              </div>
            </div>

            <div style="font-size: 0.75rem; color: #64748b; display: flex; gap: 12px; border-top: 1px solid rgba(255,255,255,0.04); padding-top: 10px;">
              <span>👁️ ${viewsText}</span>
              <span>💾 ${downloadsText}</span>
            </div>

            <div class="promptyly-buttons">
              <a href="${settings.registryUrl}/apps/${matchingApp.id}/" target="_blank" class="promptyly-btn promptyly-btn-secondary">
                View Live Demo
              </a>
              <button class="promptyly-btn promptyly-btn-primary" id="btn-install-app" ${!daemonRunning ? 'disabled' : ''}>
                Install Locally
              </button>
            </div>
            
            ${!daemonRunning ? `
              <p style="font-size: 0.75rem; color: #f87171; text-align: center; margin: 0;">Local daemon offline. Cannot install.</p>
            ` : ''}
          </div>
        `;

        if (daemonRunning) {
          document.getElementById('btn-install-app').addEventListener('click', async (e) => {
            const btn = e.target;
            btn.disabled = true;
            btn.textContent = 'Installing...';

            try {
              const res = await daemonFetch('/api/apps/download', settings, 'POST', { appId: matchingApp.id });
              if (res.ok) {
                const data = await res.json();
                btn.textContent = 'Success! Opening...';
                btn.style.background = '#10b981';
                setTimeout(() => {
                  window.open(`${settings.daemonUrl}/apps/${data.appName}/`, '_blank');
                  document.getElementById('promptyly-overlay-container').remove();
                }, 1000);
              } else {
                const txt = await res.text();
                alert('Install failed: ' + txt);
                btn.disabled = false;
                btn.textContent = 'Install Locally';
              }
            } catch (err) {
              alert('Install error: ' + err.message);
              btn.disabled = false;
              btn.textContent = 'Install Locally';
            }
          });
        }
      } else {
        // Not in registry, not installed locally
        renderNotFoundView(container, appName, daemonRunning);
      }
    } catch (err) {
      renderNotFoundView(container, appName, daemonRunning);
    }
  }

  // Render when app is not found anywhere
  function renderNotFoundView(container, name, daemonRunning) {
    const formattedName = name.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
    container.innerHTML = `
      <div style="display: flex; flex-direction: column; gap: 16px;">
        <div class="promptyly-section">
          <span class="promptyly-badge promptyly-badge-warn">App Not Found</span>
        </div>
        <p style="font-size: 0.9rem; line-height: 1.5; color: #cbd5e1; margin: 0;">
          The application name <code>${name}</code> was not found in your local workspace or on the remote sharing server.
        </p>
        <div class="promptyly-section">
          <span class="promptyly-label">Suggested Action</span>
          <p style="font-size: 0.85rem; color: #94a3b8; line-height: 1.4;">
            You can create a new application named <strong>${formattedName}</strong> by clicking the button below.
          </p>
        </div>
        <div class="promptyly-buttons">
          <button class="promptyly-btn promptyly-btn-primary" id="btn-create-missing" ${!daemonRunning ? 'disabled' : ''}>
            Create App "${formattedName}"
          </button>
        </div>
      </div>
    `;

    if (daemonRunning) {
      document.getElementById('btn-create-missing').addEventListener('click', () => {
        // Prompt for generation details
        const promptText = prompt(`Enter a prompt to generate the application "${formattedName}":`, `A sleek ${formattedName} with clean UI and dark mode`);
        if (promptText) {
          renderNewPromptView(container, promptText, daemonRunning, DEFAULT_SETTINGS);
        }
      });
    }
  }
})();
