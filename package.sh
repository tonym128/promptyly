#!/usr/bin/env bash
set -e

echo "=================================================="
echo "📦 Packaging Promptyly Desktop Suite..."
echo "=================================================="

# 1. Create clean output directories
mkdir -p dist
mkdir -p desktop/bin

# Detect go executable path
GO_CMD="go"
if ! command -v go >/dev/null 2>&1; then
  if [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
  else
    echo "❌ Error: Go compiler not found! Please install Go or add it to your PATH."
    exit 1
  fi
fi

# 2. Compile Go daemon binaries for multiple OS architectures
echo "⚙️ Compiling Go daemon binaries..."
mkdir -p sharing/data/binaries

echo "👉 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 $GO_CMD build -o sharing/data/binaries/promptyly-linux-amd64 main.go sharingclient.go
cp sharing/data/binaries/promptyly-linux-amd64 desktop/bin/promptyly

echo "👉 Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 $GO_CMD build -o sharing/data/binaries/promptyly-linux-arm64 main.go sharingclient.go

echo "👉 Building for Linux (arm)..."
GOOS=linux GOARCH=arm GOARM=7 $GO_CMD build -o sharing/data/binaries/promptyly-linux-arm main.go sharingclient.go

echo "👉 Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 $GO_CMD build -o sharing/data/binaries/promptyly-windows-amd64.exe main.go sharingclient.go
cp sharing/data/binaries/promptyly-windows-amd64.exe desktop/bin/promptyly.exe

echo "👉 Building for Windows (arm64)..."
GOOS=windows GOARCH=arm64 $GO_CMD build -o sharing/data/binaries/promptyly-windows-arm64.exe main.go sharingclient.go

echo "👉 Building for macOS (arm64 - Apple Silicon)..."
GOOS=darwin GOARCH=arm64 $GO_CMD build -o sharing/data/binaries/promptyly-darwin-arm64 main.go sharingclient.go
cp sharing/data/binaries/promptyly-darwin-arm64 desktop/bin/promptyly-mac

echo "👉 Building for macOS (amd64 - Intel)..."
GOOS=darwin GOARCH=amd64 $GO_CMD build -o sharing/data/binaries/promptyly-darwin-amd64 main.go sharingclient.go

echo "👉 Building for Android (arm64)..."
GOOS=android GOARCH=arm64 $GO_CMD build -o sharing/data/binaries/promptyly-android-arm64 main.go sharingclient.go

# 2.5 Optional local llamafile download
if [ "$INCLUDE_LLAMAFILE" = "true" ]; then
  echo "📥 INCLUDE_LLAMAFILE is set to true. Downloading model to local cache..."
  LLAMAFILE_DEST="sharing/data/binaries/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile"
  if [ ! -f "$LLAMAFILE_DEST" ]; then
    echo "🔗 Downloading Qwen2.5-Coder-1.5B llamafile from Hugging Face..."
    curl -L -f -# "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf" -o "$LLAMAFILE_DEST"
    chmod +x "$LLAMAFILE_DEST"
    echo "✓ Model downloaded successfully!"
  else
    echo "✓ Model is already cached at $LLAMAFILE_DEST"
  fi
fi

# 3. Zip Browser Extension
echo "🔌 Packaging browser extension..."
if command -v zip >/dev/null 2>&1; then
  rm -f dist/promptyly-extension.zip
  zip -r dist/promptyly-extension.zip browser-extension -x "*.git*" -x "*node_modules*"
  echo "✓ Browser extension packaged to dist/promptyly-extension.zip"
else
  echo "⚠️  'zip' command not found, skipping extension compression."
fi

# 4. Prompt Electron build steps
echo "💻 Electron app is ready for compilation."
echo "   To package the installer for your current OS, execute:"
echo "   cd desktop && npm install && npm run build"
echo ""
echo "=================================================="
echo "✅ Packaging complete. Distribution files created in desktop/bin/ and dist/"
echo "=================================================="
