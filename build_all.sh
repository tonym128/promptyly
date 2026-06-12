#!/usr/bin/env bash
set -e

echo "=================================================="
echo "🏗️  Building Promptyly Suite Locally..."
echo "=================================================="

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

# 1. Compile the local CLI & Daemon
echo "⚙️  1. Incrementing version revision..."
$GO_CMD run bump_version.go
echo "⚙️  1. Compiling Promptyly CLI/Daemon binary..."
$GO_CMD build -o promptyly main.go sharingclient.go
echo "   ✓ Built: ./promptyly"

# 2. Compile the Sharing Registry Server
echo "⚙️  2. Compiling Sharing Registry Server binary..."
$GO_CMD build -o sharing/sharing-server ./sharing
echo "   ✓ Built: ./sharing/sharing-server"
echo "⚙️  2. Packaging all platform binaries into sharing/data/binaries/..."
./package.sh

# 3. Package Browser Extension
echo "⚙️  3. Packaging Browser Extension..."
mkdir -p dist
if command -v zip >/dev/null 2>&1; then
  rm -f dist/promptyly-extension.zip
  zip -r dist/promptyly-extension.zip browser-extension -x "*.git*" > /dev/null
  echo "   ✓ Extension packaged to ./dist/promptyly-extension.zip"
else
  echo "   ⚠️  'zip' command not found. Skipping extension zipping."
fi

echo "=================================================="
echo "🎉 Local Build Success Summary:"
echo "=================================================="
echo "  [✓] Go Daemon:           ./promptyly"
echo "  [✓] Go Sharing Registry: ./sharing/sharing-server"
if [ -f "dist/promptyly-extension.zip" ]; then
echo "  [✓] Browser Extension:   ./dist/promptyly-extension.zip"
fi
echo "=================================================="
