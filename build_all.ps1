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

Write-Host "⚙️ 2. Packaging all platform binaries into sharing\data\binaries\..." -ForegroundColor Yellow
$targets = @(
    @{ OS = "linux"; Arch = "amd64"; Out = "sharing\data\binaries\promptyly-linux-amd64" },
    @{ OS = "linux"; Arch = "arm64"; Out = "sharing\data\binaries\promptyly-linux-arm64" },
    @{ OS = "linux"; Arch = "arm"; Out = "sharing\data\binaries\promptyly-linux-arm"; Env = @{ GOARM = "7" } },
    @{ OS = "windows"; Arch = "amd64"; Out = "sharing\data\binaries\promptyly-windows-amd64.exe" },
    @{ OS = "windows"; Arch = "arm64"; Out = "sharing\data\binaries\promptyly-windows-arm64.exe" },
    @{ OS = "darwin"; Arch = "arm64"; Out = "sharing\data\binaries\promptyly-darwin-arm64" },
    @{ OS = "darwin"; Arch = "amd64"; Out = "sharing\data\binaries\promptyly-darwin-amd64" },
    @{ OS = "android"; Arch = "arm64"; Out = "sharing\data\binaries\promptyly-android-arm64" }
)

New-Item -ItemType Directory -Path "sharing\data\binaries" -Force | Out-Null
foreach ($t in $targets) {
    Write-Host "  👉 Building for $($t.OS) ($($t.Arch))..." -ForegroundColor Yellow
    $env:GOOS = $t.OS
    $env:GOARCH = $t.Arch
    if ($t.Env) {
        foreach ($k in $t.Env.Keys) {
            Set-Item "env:$k" $t.Env[$k]
        }
    }
    go build -o $t.Out main.go sharingclient.go
    if ($t.Env) {
        foreach ($k in $t.Env.Keys) {
            Remove-Item "env:$k"
        }
    }
}
Remove-Item env:GOOS -ErrorAction SilentlyContinue
Remove-Item env:GOARCH -ErrorAction SilentlyContinue
Write-Host "   ✓ All target binaries packaged!" -ForegroundColor Green

# 2.5 Optional local llamafile download
if ($env:INCLUDE_LLAMAFILE -eq "true") {
    Write-Host "📥 INCLUDE_LLAMAFILE is set to true. Downloading model to local cache..." -ForegroundColor Yellow
    $llamafileDest = "sharing\data\binaries\qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile"
    if (-not (Test-Path $llamafileDest)) {
        Write-Host "🔗 Downloading Qwen2.5-Coder-1.5B llamafile from Hugging Face..." -ForegroundColor Yellow
        $url = "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
        Invoke-WebRequest -Uri $url -OutFile $llamafileDest -UserAgent "Mozilla/5.0"
        Write-Host "   ✓ Model downloaded successfully!" -ForegroundColor Green
    } else {
        Write-Host "   ✓ Model is already cached at $llamafileDest" -ForegroundColor Green
    }
}

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
