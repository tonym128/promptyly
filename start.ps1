# Promptyly Startup Script (Windows PowerShell)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "🚀 Bootstrapping Promptyly Desktop & Daemon..." -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 1. Compile the Go developer daemon locally
Write-Host "⚙️ Building local developer daemon..." -ForegroundColor Yellow
go build -o promptyly.exe main.go sharingclient.go

# 2. Run Go Daemon in background
Write-Host "🔌 Starting local daemon on port 6071..." -ForegroundColor Yellow
$DaemonProcess = Start-Process -FilePath ".\promptyly.exe" -ArgumentList "serve" -NoNewWindow -PassThru
Write-Host "✓ Daemon running in background (PID: $($DaemonProcess.Id))" -ForegroundColor Green

# 3. Start Electron desktop application
Write-Host "💻 Launching Electron front-end app..." -ForegroundColor Yellow
Set-Location .\desktop
if (-not (Test-Path "node_modules")) {
    Write-Host "📦 node_modules not found, running npm install..." -ForegroundColor DarkYellow
    npm install
}
$ElectronProcess = Start-Process -FilePath "npm" -ArgumentList "start" -NoNewWindow -PassThru

# Keep the script active and handle cleanup on exit
try {
    Write-Host "Press Ctrl+C to terminate all services..." -ForegroundColor Green
    while ($true) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host ""
    Write-Host "🧹 Shutting down all background Promptyly services..." -ForegroundColor Red
    
    # Terminate the Go daemon
    if ($DaemonProcess) {
        Stop-Process -Id $DaemonProcess.Id -Force -ErrorAction SilentlyContinue
    }
    # Terminate Electron
    if ($ElectronProcess) {
        Stop-Process -Id $ElectronProcess.Id -Force -ErrorAction SilentlyContinue
    }
    
    Write-Host "✓ Cleanup complete. Goodbye!" -ForegroundColor Green
}
