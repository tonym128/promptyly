# Promptyly Local Build Script (Windows PowerShell)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "🏗️  Building Promptyly Suite Locally..." -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 1. Compile the local CLI & Daemon
Write-Host "⚙️ 1. Incrementing version revision..." -ForegroundColor Yellow
go run bump_version.go
Write-Host "⚙️ 1. Compiling Promptyly CLI/Daemon binary..." -ForegroundColor Yellow
go build -o promptyly.exe main.go sharingclient.go
Write-Host "   ✓ Built: .\promptyly.exe" -ForegroundColor Green

# 2. Compile the Sharing Registry Server
Write-Host "⚙️ 2. Compiling Sharing Registry Server binary..." -ForegroundColor Yellow
go build -o sharing\sharing-server.exe .\sharing
Write-Host "   ✓ Built: .\sharing\sharing-server.exe" -ForegroundColor Green

# 3. Package Browser Extension
Write-Host "⚙️ 3. Packaging Browser Extension..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path "dist" -Force | Out-Null
if (Get-Command tar -ErrorAction SilentlyContinue) {
    # Compress using tar since tar is built-in to modern Windows 10/11
    if (Test-Path "dist\promptyly-extension.zip") {
        Remove-Item "dist\promptyly-extension.zip" -Force
    }
    tar -a -c -f dist\promptyly-extension.zip browser-extension
    Write-Host "   ✓ Extension packaged to .\dist\promptyly-extension.zip" -ForegroundColor Green
} else {
    Write-Host "   ⚠️ tar/zip command not found. Skipping extension packaging." -ForegroundColor DarkYellow
}

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "🎉 Local Build Success Summary:" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  [✓] Go Daemon:           .\promptyly.exe" -ForegroundColor Green
Write-Host "  [✓] Go Sharing Registry: .\sharing\sharing-server.exe" -ForegroundColor Green
if (Test-Path "dist\promptyly-extension.zip") {
    Write-Host "  [✓] Browser Extension:   .\dist\promptyly-extension.zip" -ForegroundColor Green
}
Write-Host "==================================================" -ForegroundColor Cyan
