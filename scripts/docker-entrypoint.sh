#!/bin/sh
set -e

# Entrypoint script to start both:
# 1. Discovery backend service (main application)
# 2. NGINX (serving /version endpoint internally)

# Function to handle shutdown signals
cleanup() {
    echo "Received shutdown signal, stopping services..."
    kill -TERM "$NGINX_PID" 2>/dev/null || true
    kill -TERM "$SERVER_PID" 2>/dev/null || true
    wait "$NGINX_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    exit 0
}

# Trap signals for graceful shutdown
trap cleanup TERM INT

# Start NGINX in background (serving /version on 0.0.0.0:8082)
# Accessible from other containers in the same Docker network
echo "Starting NGINX (version endpoint on 0.0.0.0:8082)..."
nginx -g "daemon off;" &
NGINX_PID=$!

# Wait a moment for NGINX to start
sleep 1

# Verify NGINX is running
if ! kill -0 "$NGINX_PID" 2>/dev/null; then
    echo "ERROR: NGINX failed to start"
    exit 1
fi

# Verify version.json exists
if [ ! -f /usr/share/nginx/html/version.json ]; then
    echo "WARNING: /usr/share/nginx/html/version.json not found, /version will return 500"
fi

# Start the main application server in background
echo "Starting discovery backend service..."
/app/server &
SERVER_PID=$!

# Wait a moment for server to start
sleep 1

# Verify server is running
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "ERROR: Discovery backend service failed to start"
    kill "$NGINX_PID" 2>/dev/null || true
    exit 1
fi

echo "Both services started successfully"
echo "  - Discovery backend: port 8080 (public via docker-compose)"
echo "  - NGINX version endpoint: 0.0.0.0:8082/version (Docker network only, not exposed publicly)"

# Wait for both processes
# Monitor both processes and exit if either dies
while true; do
    if ! kill -0 "$NGINX_PID" 2>/dev/null; then
        echo "NGINX process exited"
        cleanup
    fi
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Discovery backend process exited"
        cleanup
    fi
    sleep 1
done
