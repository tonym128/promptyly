#!/usr/bin/env bash
set -e

# Promptyly Startup Script (Linux/macOS)
echo "=================================================="
echo "🚀 Bootstrapping Promptyly Desktop & Daemon..."
echo "=================================================="

# 1. Compile the Go developer daemon locally
echo "⚙️  Building local developer daemon..."
go build -o promptyly main.go sharingclient.go

# 2. Run Go Daemon in background
echo "🔌 Starting local daemon on port 6071..."
./promptyly serve > daemon.log 2>&1 &
DAEMON_PID=$!
echo "✓ Daemon running in background (PID: $DAEMON_PID)"

# 3. Start Electron desktop application
echo "💻 Launching Electron front-end app..."
cd desktop
if [ ! -d "node_modules" ]; then
    echo "📦 node_modules not found, running npm install..."
    npm install
fi
npm start &
ELECTRON_PID=$!

# Cleanup trap to kill child processes on termination
cleanup() {
  echo ""
  echo "🧹 Shutting down all background Promptyly services..."
  kill $DAEMON_PID 2>/dev/null || true
  kill $ELECTRON_PID 2>/dev/null || true
  echo "✓ Cleanup complete. Goodbye!"
  exit 0
}
trap cleanup SIGINT SIGTERM EXIT

# Keep script active to wait for user exit
wait
