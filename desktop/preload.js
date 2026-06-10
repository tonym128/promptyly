const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  openDirectory: () => ipcRenderer.invoke('dialog:open-directory'),
  registerProtocol: () => ipcRenderer.invoke('protocol:register'),
  checkProtocol: () => ipcRenderer.invoke('protocol:check'),
  setBackground: (enabled) => ipcRenderer.invoke('background:set', enabled),
  getBackground: () => ipcRenderer.invoke('background:get'),
  quitApp: () => ipcRenderer.invoke('app:quit'),
  getToken: () => ipcRenderer.invoke('app:get-token'),
  onDeepLink: (callback) => ipcRenderer.on('deep-link', (event, url) => callback(url)),
  openPath: (path) => ipcRenderer.invoke('shell:open-path', path),
  saveFileDialog: (appName) => ipcRenderer.invoke('dialog:save-file', appName),
  openBrowser: (url) => ipcRenderer.invoke('shell:open-external', url),
});
