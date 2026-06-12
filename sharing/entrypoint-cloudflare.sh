#!/bin/sh
set -e

# Ensure data directory exists
mkdir -p /app/data

echo "🚀 Starting Promptyly Registry Server..."
# Start the registry server in the background
./sharing-server -data /app/data &
SERVER_PID=$!

# Handle shutdown signals
cleanup() {
    echo "Shutting down registry server..."
    kill -TERM "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    exit 0
}
trap cleanup INT TERM

# Wait a moment to let the registry server start
sleep 2

if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "❌ Promptyly Registry Server failed to start."
    exit 1
fi

echo "✅ Promptyly Registry Server is running (PID: $SERVER_PID)."

# Start Cloudflare Tunnel
if [ -n "$CLOUDFLARE_TUNNEL_TOKEN" ]; then
    echo "🔑 Starting named Cloudflare Tunnel..."
    exec cloudflared tunnel --no-autoupdate run --token "$CLOUDFLARE_TUNNEL_TOKEN"
else
    echo "🌐 Starting anonymous Cloudflare Quick Tunnel..."
    exec cloudflared tunnel --no-autoupdate --url http://localhost:6072
fi
