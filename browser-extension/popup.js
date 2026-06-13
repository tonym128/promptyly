// Default settings values
const DEFAULT_SETTINGS = {
  daemonUrl: 'http://localhost:6071',
  daemonToken: '',
  registryUrl: 'http://localhost:6072',
  registryToken: '',
  generateOnRegistry: false,
  interceptLinks: true
};

// Load settings on popup open
document.addEventListener('DOMContentLoaded', () => {
  chrome.storage.local.get(DEFAULT_SETTINGS, (settings) => {
    document.getElementById('daemon-url').value = settings.daemonUrl;
    document.getElementById('daemon-token').value = settings.daemonToken;
    document.getElementById('registry-url').value = settings.registryUrl;
    document.getElementById('registry-token').value = settings.registryToken;
    document.getElementById('generate-on-registry').checked = settings.generateOnRegistry;
    document.getElementById('intercept-links').checked = settings.interceptLinks;
  });
});

// Save settings on button click
document.getElementById('save-btn').addEventListener('click', () => {
  const settings = {
    daemonUrl: document.getElementById('daemon-url').value.trim() || DEFAULT_SETTINGS.daemonUrl,
    daemonToken: document.getElementById('daemon-token').value.trim(),
    registryUrl: document.getElementById('registry-url').value.trim() || DEFAULT_SETTINGS.registryUrl,
    registryToken: document.getElementById('registry-token').value.trim(),
    generateOnRegistry: document.getElementById('generate-on-registry').checked,
    interceptLinks: document.getElementById('intercept-links').checked
  };

  chrome.storage.local.set(settings, () => {
    const saveBtn = document.getElementById('save-btn');
    const originalText = saveBtn.textContent;
    saveBtn.textContent = 'Saved!';
    saveBtn.style.background = '#10b981';
    
    setTimeout(() => {
      saveBtn.textContent = originalText;
      saveBtn.style.background = '';
    }, 1500);
  });
});
