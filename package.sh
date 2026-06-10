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
  if [ -x "$HOME/.local/go/bin/go" ]; then
    GO_CMD="$HOME/.local/go/bin/go"
  elif [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
  else
    echo "❌ Error: Go compiler not found! Please install Go or add it to your PATH."
    exit 1
  fi
fi

# 2. Compile Go daemon binaries for multiple OS architectures (Sidecar pattern)
echo "⚙️ Compiling Go daemon binaries..."
echo "👉 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 $GO_CMD build -o desktop/bin/promptyly main.go sharingclient.go

echo "👉 Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 $GO_CMD build -o desktop/bin/promptyly.exe main.go sharingclient.go

echo "👉 Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 $GO_CMD build -o desktop/bin/promptyly-mac main.go sharingclient.go

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
