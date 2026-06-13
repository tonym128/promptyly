#!/usr/bin/env bash
set -e

# Promptyly Startup Script (Linux/macOS)
echo "=================================================="
echo "🚀 Bootstrapping Promptyly Desktop & Daemon..."
echo "=================================================="

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

# 1. Compile the Go developer daemon locally
echo "⚙️  Incrementing version revision..."
$GO_CMD run bump_version.go
echo "⚙️  Building local developer daemon..."
$GO_CMD build -o promptyly main.go sharingclient.go

# 2. Run Go Daemon in background
echo "🔌 Starting local daemon on port 6071..."
./promptyly serve > daemon.log 2>&1 &
DAEMON_PID=$!
echo "✓ Daemon running in background (PID: $DAEMON_PID)"

# 3. Open Web UI in browser
echo "💻 Opening Web UI in default browser..."
if command -v xdg-open >/dev/null 2>&1; then
  xdg-open http://127.0.0.1:6071/ >/dev/null 2>&1 &
elif command -v open >/dev/null 2>&1; then
  open http://127.0.0.1:6071/ >/dev/null 2>&1 &
fi

# Cleanup trap to kill child processes on termination
cleanup() {
  echo ""
  echo "🧹 Shutting down all background Promptyly services..."
  kill $DAEMON_PID 2>/dev/null || true
  echo "✓ Cleanup complete. Goodbye!"
  exit 0
}
trap cleanup SIGINT SIGTERM EXIT

# Keep script active to wait for user exit
wait
