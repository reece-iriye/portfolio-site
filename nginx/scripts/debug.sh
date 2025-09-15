#!/bin/bash

echo "=== NGINX DEBUG SCRIPT ==="
echo "Timestamp: $(date)"
echo ""

# Check if running in container
if [ -f /.dockerenv ]; then
    echo "Running in Docker container"
    CONTAINER_MODE=true
else
    echo "Running on host system"
    CONTAINER_MODE=false
fi

echo ""

# Check nginx processes
echo "=== NGINX PROCESSES ==="
ps aux | grep nginx | grep -v grep
echo ""

# Check if nginx is actually running
echo "=== NGINX STATUS ==="
if command -v systemctl >/dev/null 2>&1; then
    systemctl status nginx 2>/dev/null || echo "systemctl not available or nginx service not found"
elif [ "$CONTAINER_MODE" = true ]; then
    if pgrep nginx > /dev/null; then
        echo "nginx processes are running"
    else
        echo "nginx processes are NOT running"
    fi
fi
echo ""

# Check nginx configuration
echo "=== NGINX CONFIG TEST ==="
if [ "$CONTAINER_MODE" = true ]; then
    /usr/local/openresty/bin/openresty -t 2>&1
else
    nginx -t 2>&1
fi
echo ""

# Check lua modules
echo "=== LUA MODULE CHECK ==="
if [ "$CONTAINER_MODE" = true ]; then
    echo "Checking OpenResty Lua modules..."
    /usr/local/openresty/bin/openresty -V 2>&1 | grep -o 'lua[^ ]*'
    echo ""
    
    echo "Checking Lua files in /usr/local/openresty/lualib/prometheus/:"
    ls -la /usr/local/openresty/lualib/prometheus/ 2>/dev/null || echo "Directory not found"
    echo ""
    
    echo "Checking Lua files in /etc/nginx/lua/:"
    ls -la /etc/nginx/lua/ 2>/dev/null || echo "Directory not found"
    echo ""
fi

# Check error logs
echo "=== RECENT ERROR LOGS ==="
if [ -f /var/log/nginx/error.log ]; then
    echo "Last 20 lines from /var/log/nginx/error.log:"
    tail -20 /var/log/nginx/error.log
    echo ""
fi

if [ -f /var/log/nginx/lua_debug.log ]; then
    echo "Last 20 lines from /var/log/nginx/lua_debug.log:"
    tail -20 /var/log/nginx/lua_debug.log
    echo ""
fi

# Check access logs for 500 errors
echo "=== RECENT 500 ERRORS ==="
if [ -f /var/log/nginx/access.log ]; then
    echo "Recent 500 errors from access.log:"
    grep " 500 " /var/log/nginx/access.log | tail -10
    echo ""
fi

# Check listening ports
echo "=== LISTENING PORTS ==="
netstat -tlnp 2>/dev/null | grep -E ':(80|443|8080|9145)' || ss -tlnp 2>/dev/null | grep -E ':(80|443|8080|9145)' || echo "Cannot check ports (netstat/ss not available)"
echo ""

# Test endpoints if nginx is running
echo "=== ENDPOINT TESTS ==="
if pgrep nginx > /dev/null; then
    echo "Testing /health endpoint:"
    curl -s -w "HTTP Status: %{http_code}\n" http://localhost:9145/health 2>/dev/null || echo "Cannot connect to health endpoint"
    echo ""
    
    echo "Testing /debug endpoint:"
    curl -s -w "HTTP Status: %{http_code}\n" http://localhost:9145/debug 2>/dev/null || echo "Cannot connect to debug endpoint"
    echo ""
    
    echo "Testing /metrics endpoint:"
    curl -s -w "HTTP Status: %{http_code}\n" -o /tmp/metrics_test.out http://localhost:9145/metrics 2>/dev/null || echo "Cannot connect to metrics endpoint"
    if [ -f /tmp/metrics_test.out ]; then
        echo "Metrics response size: $(wc -c < /tmp/metrics_test.out) bytes"
        echo "First few lines of metrics:"
        head -10 /tmp/metrics_test.out
        rm -f /tmp/metrics_test.out
    fi
else
    echo "nginx is not running, skipping endpoint tests"
fi
echo ""

# Check shared memory
echo "=== SHARED MEMORY ==="
if [ "$CONTAINER_MODE" = true ]; then
    echo "Checking /dev/shm:"
    df -h /dev/shm 2>/dev/null || echo "/dev/shm not available"
    ls -la /dev/shm/ 2>/dev/null | head -10
fi
echo ""

# Check file permissions
echo "=== FILE PERMISSIONS ==="
if [ "$CONTAINER_MODE" = true ]; then
    echo "Checking key file permissions:"
    ls -la /usr/local/openresty/nginx/conf/nginx.conf 2>/dev/null || echo "Main config not found"
    ls -la /etc/nginx/conf.d/nginx.prometheus.conf 2>/dev/null || echo "Prometheus config not found"
    ls -la /usr/local/openresty/lualib/prometheus/ 2>/dev/null | head -5
fi
echo ""

# Memory and disk space
echo "=== SYSTEM RESOURCES ==="
echo "Memory usage:"
free -h 2>/dev/null || echo "free command not available"
echo ""
echo "Disk usage:"
df -h / 2>/dev/null || echo "df command not available"
echo ""

# Environment variables
echo "=== ENVIRONMENT ==="
echo "User: $(whoami)"
echo "Working directory: $(pwd)"
if [ "$CONTAINER_MODE" = true ]; then
    echo "Container environment variables:"
    env | grep -E "(NGINX|LUA|PATH)" | sort
fi
echo ""

echo "=== DEBUG COMPLETE ==="
echo "To get more detailed logs, run:"
echo "  tail -f /var/log/nginx/error.log"
echo "  tail -f /var/log/nginx/lua_debug.log"
echo ""
echo "To test configuration:"
if [ "$CONTAINER_MODE" = true ]; then
    echo "  /usr/local/openresty/bin/openresty -t"
else
    echo "  nginx -t"
fi
