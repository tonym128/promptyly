const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage } = require('electron');
const path = require('path');
const { spawn, exec } = require('child_process');
const net = require('net');
const fs = require('fs');
const os = require('os');

let mainWindow = null;
let tray = null;
let backendProcess = null;
let isQuitting = false;
let runInBackground = false; // default, can be toggled by user
let settingsPath = '';

function loadSettings() {
  try {
    if (!settingsPath) {
      settingsPath = path.join(app.getPath('userData'), 'settings.json');
    }
    if (fs.existsSync(settingsPath)) {
      const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
      runInBackground = !!data.runInBackground;
      console.log('Loaded settings. runInBackground =', runInBackground);
    }
  } catch (err) {
    console.error('Failed to load settings:', err);
  }
}

function saveSettings() {
  try {
    if (!settingsPath) {
      settingsPath = path.join(app.getPath('userData'), 'settings.json');
    }
    const data = JSON.stringify({ runInBackground });
    fs.writeFileSync(settingsPath, data, 'utf8');
    console.log('Saved settings. runInBackground =', runInBackground);
  } catch (err) {
    console.error('Failed to save settings:', err);
  }
}

// Check if server is running on port 6071
function checkServerRunning(port) {
  return new Promise((resolve) => {
    const client = new net.Socket();
    client.once('connect', () => {
      client.end();
      resolve(true);
    });
    client.once('error', () => {
      resolve(false);
    });
    client.connect({ port, host: '127.0.0.1' });
  });
}

// Start Go backend server
async function startBackend() {
  const isRunning = await checkServerRunning(6071);
  if (isRunning) {
    console.log('Go server is already running.');
    return;
  }

  const isDev = !app.isPackaged;
  const exeName = process.platform === 'win32' ? 'promptyly.exe' : 'promptyly';
  const backendPath = isDev
    ? path.join(__dirname, '..', exeName)
    : path.join(process.resourcesPath, 'bin', exeName);

  console.log(`Starting backend server at: ${backendPath}`);
  
  backendProcess = spawn(backendPath, ['serve']);

  backendProcess.stdout.on('data', (data) => {
    console.log(`[Backend]: ${data}`);
  });

  backendProcess.stderr.on('data', (data) => {
    console.error(`[Backend Err]: ${data}`);
  });

  backendProcess.on('close', (code) => {
    console.log(`Backend process exited with code ${code}`);
    backendProcess = null;
  });
}

// Handle deep link URL parsing and forwarding
function handleDeepLink(urlStr) {
  if (!urlStr || !urlStr.startsWith('prompt://')) return;
  console.log('Handling deep link URL:', urlStr);
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.show();
    mainWindow.focus();
    // Send to renderer
    mainWindow.webContents.send('deep-link', urlStr);
  }
}

// Single instance lock
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', (event, commandLine) => {
    // Someone tried to run a second instance, focus our window.
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    }
    // Find prompt:// link in command line args
    const url = commandLine.find(arg => arg.startsWith('prompt://'));
    if (url) {
      handleDeepLink(url);
    }
  });

  // Handle scheme registration for macOS / Linux / Windows
  app.on('open-url', (event, url) => {
    event.preventDefault();
    handleDeepLink(url);
  });
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 750,
    minWidth: 800,
    minHeight: 600,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
    title: 'Promptyly Desktop',
    backgroundColor: '#0b0f19',
    frame: true,
  });

  mainWindow.loadFile(path.join(__dirname, 'index.html'));

  mainWindow.on('close', (e) => {
    if (!isQuitting && runInBackground) {
      e.preventDefault();
      mainWindow.hide();
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  // Check arguments for URL scheme on cold startup
  const urlArg = process.argv.find(arg => arg.startsWith('prompt://'));
  if (urlArg) {
    // Wait a little bit for page to load
    setTimeout(() => {
      handleDeepLink(urlArg);
    }, 1500);
  }
}

// Setup System Tray
function setupTray() {
  if (tray) return;

  const base64Data = 'iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAMklEQVQ4T2NkoBAwUqifgdF/HEmMyECoAB5jMAwqgMcYDIVK4DEGwqACeIwBMRgGjAxgAABO7wEfUeD9igAAAABJRU5ErkJggg==';
  const icon = nativeImage.createFromBuffer(Buffer.from(base64Data, 'base64'));
  
  tray = new Tray(icon);
  const contextMenu = Menu.buildFromTemplate([
    { label: 'Show App', click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        } else {
          createWindow();
        }
      } 
    },
    { label: 'Restart Go Server', click: () => {
        if (backendProcess) {
          backendProcess.kill();
        }
        setTimeout(startBackend, 1000);
      }
    },
    { type: 'separator' },
    { label: 'Quit', click: () => {
        isQuitting = true;
        app.quit();
      } 
    }
  ]);

  tray.setToolTip('Promptyly Developer Hub');
  tray.setContextMenu(contextMenu);

  tray.on('click', () => {
    if (mainWindow) {
      if (mainWindow.isVisible()) {
        mainWindow.hide();
      } else {
        mainWindow.show();
        mainWindow.focus();
      }
    }
  });
}

// Create a dummy tray icon if it doesn't exist
function createDummyTrayIcon() {
  const iconPath = path.join(__dirname, 'tray_icon.png');
  if (!fs.existsSync(iconPath)) {
    // A 16x16 red/blue dot base64 png
    const base64Data = 'iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAMklEQVQ4T2NkoBAwUqifgdF/HEmMyECoAB5jMAwqgMcYDIMK4DEGwqACeIwBMRgGjAxgAABO7wEfUeD9igAAAABJRU5ErkJggg==';
    fs.writeFileSync(iconPath, Buffer.from(base64Data, 'base64'));
  }
}

app.whenReady().then(async () => {
  loadSettings();
  createDummyTrayIcon();
  await startBackend();
  createWindow();
  setupTray();

  // Register custom protocol client
  if (!app.isDefaultProtocolClient('prompt')) {
    app.setAsDefaultProtocolClient('prompt');
  }
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow();
  }
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin' && !runInBackground) {
    app.quit();
  }
});

app.on('before-quit', () => {
  isQuitting = true;
  if (backendProcess) {
    console.log('Killing backend server...');
    backendProcess.kill();
  }
});

// IPC Handler - Open directory dialog
ipcMain.handle('dialog:open-directory', async () => {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openDirectory']
  });
  if (result.canceled) {
    return null;
  } else {
    return result.filePaths[0];
  }
});

// IPC Handler - Register protocol handler manually (writes desktop entry)
ipcMain.handle('protocol:register', async () => {
  try {
    const isDev = !app.isPackaged;
    const electronPath = process.execPath;
    const appDir = app.getAppPath();
    const execCmd = isDev ? `"${electronPath}" "${appDir}" %u` : `"${electronPath}" %u`;
    
    if (process.platform === 'linux') {
      const home = os.homedir();
      const appsDir = path.join(home, '.local', 'share', 'applications');
      if (!fs.existsSync(appsDir)) {
        fs.mkdirSync(appsDir, { recursive: true });
      }

      const desktopFile = path.join(appsDir, 'promptyly-desktop.desktop');
      const content = `[Desktop Entry]
Type=Application
Name=Promptyly Desktop
Exec=${execCmd}
StartupNotify=true
Terminal=false
MimeType=x-scheme-handler/prompt;
NoDisplay=false
Icon=${path.join(__dirname, 'tray_icon.png')}
`;

      fs.writeFileSync(desktopFile, content, 'utf8');

      // Update databases
      exec(`update-desktop-database "${appsDir}"`);
      exec(`xdg-mime default promptyly-desktop.desktop x-scheme-handler/prompt`);

      return { success: true, message: 'Registered successfully on Linux.' };
    } else if (process.platform === 'win32') {
      // Windows registry keys
      const escapedExec = execCmd.replace(/"/g, '\\"');
      const regCommands = [
        `reg add HKCR\\prompt /ve /d "URL:Promptyly Protocol" /f`,
        `reg add HKCR\\prompt /v "URL Protocol" /d "" /f`,
        `reg add HKCR\\prompt\\shell\\open\\command /ve /d "${escapedExec}" /f`
      ];

      for (const cmd of regCommands) {
        await new Promise((resolve, reject) => {
          exec(cmd, (err) => {
            if (err) reject(err);
            else resolve();
          });
        });
      }
      return { success: true, message: 'Registered successfully in Windows Registry.' };
    } else {
      // macOS uses Info.plist, which is set at build time, but we can do a normal register call
      app.setAsDefaultProtocolClient('prompt');
      return { success: true, message: 'Registered using Electron client API on macOS.' };
    }
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// IPC Handler - Check protocol status
ipcMain.handle('protocol:check', async () => {
  if (process.platform === 'linux') {
    const home = os.homedir();
    const desktopFile = path.join(home, '.local', 'share', 'applications', 'promptyly-desktop.desktop');
    return fs.existsSync(desktopFile);
  } else {
    return app.isDefaultProtocolClient('prompt');
  }
});

// IPC Handler - Background toggle
ipcMain.handle('background:set', (event, enabled) => {
  runInBackground = enabled;
  saveSettings();
  console.log(`Run in background set to: ${runInBackground}`);
  return runInBackground;
});

ipcMain.handle('background:get', () => {
  return runInBackground;
});

// IPC Handler - Retrieve API Token from Go config
let cachedToken = '';
ipcMain.handle('app:get-token', () => {
  if (cachedToken) return cachedToken;
  try {
    const tokenPath = path.join(os.homedir(), '.config', 'promptyly', '.token');
    if (fs.existsSync(tokenPath)) {
      cachedToken = fs.readFileSync(tokenPath, 'utf8').trim();
    }
  } catch (err) {
    console.error('Failed to read API token:', err);
  }
  return cachedToken;
});

// IPC Handler - Quit
ipcMain.handle('app:quit', () => {
  isQuitting = true;
  app.quit();
});

// IPC Handler - Open folder path
ipcMain.handle('shell:open-path', async (event, folderPath) => {
  const { shell } = require('electron');
  await shell.openPath(folderPath);
});

// IPC Handler - Save file dialog for exports
ipcMain.handle('dialog:save-file', async (event, appName) => {
  const result = await dialog.showSaveDialog(mainWindow, {
    title: 'Export Application as ZIP',
    defaultPath: path.join(app.getPath('downloads'), `${appName}.zip`),
    filters: [{ name: 'ZIP Archives', extensions: ['zip'] }]
  });
  if (result.canceled) {
    return null;
  }
  return result.filePath;
});

// IPC Handler - Open external browser URL
ipcMain.handle('shell:open-external', async (event, url) => {
  const { shell } = require('electron');
  await shell.openExternal(url);
});


