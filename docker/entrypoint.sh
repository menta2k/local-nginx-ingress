#!/bin/sh

set -e

echo "🐳 Starting Local Nginx Ingress Controller..."

# Create basic directories (detailed setup handled by Go app)
mkdir -p /var/log/nginx /etc/nginx/ssl /etc/nginx/auth /etc/nginx/conf.d

# Check if Docker socket is accessible
if [ -S /var/run/docker.sock ]; then
    echo "✅ Docker socket is accessible"
else
    echo "⚠️  Warning: Docker socket not found at /var/run/docker.sock"
    echo "   Make sure to mount it with: -v /var/run/docker.sock:/var/run/docker.sock"
fi

# Display configuration
echo "📋 Configuration:"
echo "   • Nginx config: ${NGINX_CONFIG_PATH:-/etc/nginx/conf.d/docker-ingress.conf}"
echo "   • Nginx binary: ${NGINX_BINARY:-nginx}"
echo "   • Docker socket: ${DOCKER_HOST:-unix:///var/run/docker.sock}"

echo "🚀 Starting Local Nginx Ingress Controller..."

# Start the Go application directly
exec /app/local-nginx-ingress