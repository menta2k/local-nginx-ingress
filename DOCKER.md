# Docker Installation Guide

This guide covers running the Local Nginx Ingress Controller using Docker.

## Quick Start with Docker

### 1. Build the Image

```bash
# Build the Docker image
./scripts/build.sh

# Or manually
docker build -t local-nginx-ingress .
```

### 2. Run the Container

```bash
# Simple run
docker run -d \
  --name nginx-ingress \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  local-nginx-ingress

# Using the script
./scripts/run.sh
```

### 3. Test with a Sample Container

```bash
# Run a test web application
docker run -d --name test-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=test.local" \
  --label "nginx.ingress.port=80" \
  nginx:alpine

# Add to hosts file
echo "127.0.0.1 test.local" | sudo tee -a /etc/hosts

# Test
curl http://test.local
```

## Full Demo Environment

### Using Docker Compose

```bash
# Start the complete demo
./scripts/demo.sh start

# View logs
./scripts/demo.sh logs -f

# Check status
./scripts/demo.sh status

# Stop demo
./scripts/demo.sh stop

# Clean up everything
./scripts/demo.sh clean
```

This starts:
- Nginx Ingress Controller
- 5 example services with different configurations
- Automatic host file management

## Manual Docker Commands

### Build Options

```bash
# Build with specific version
./scripts/build.sh --version v1.0.0

# Build for multiple platforms
./scripts/build.sh --platform linux/amd64,linux/arm64

# Build and push to registry
./scripts/build.sh --registry myregistry.com/ --push
```

### Run Options

```bash
# Run in foreground
./scripts/run.sh --foreground

# Custom ports
./scripts/run.sh --http-port 8080 --https-port 8443

# Different container name
./scripts/run.sh --name my-ingress

# Show logs after start
./scripts/run.sh --logs
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NGINX_CONFIG_PATH` | `/etc/nginx/conf.d/docker-ingress.conf` | Nginx config file path |
| `NGINX_BINARY` | `nginx` | Nginx binary path |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker socket path |

## Volume Mounts

### Required Volumes

- `/var/run/docker.sock:/var/run/docker.sock:ro` - Docker socket (required)

### Optional Volumes

- `./ssl:/etc/nginx/ssl:ro` - SSL certificates
- `./auth:/etc/nginx/auth:ro` - Authentication files
- `./logs:/var/log/nginx` - Nginx logs

## Networking

### Ports

- `80` - HTTP traffic
- `443` - HTTPS traffic

### Network Mode

The container runs in bridge mode by default. All target containers must be reachable from the ingress controller container.

## SSL/TLS Configuration

### Self-Signed Certificates

The container automatically generates a default self-signed certificate at startup.

### Custom Certificates

```bash
# Create SSL directory
mkdir -p ssl

# Add your certificates
cp my-cert.crt ssl/
cp my-cert.key ssl/

# Mount the directory
docker run -d \
  --name nginx-ingress \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/ssl:/etc/nginx/ssl:ro \
  local-nginx-ingress
```

### Certificate Labels

```bash
docker run -d --name secure-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=secure.local" \
  --label "nginx.ingress.tls=true" \
  --label "nginx.ingress.tls.certname=secure.local" \
  my-app:latest
```

The controller will look for:
- `/etc/nginx/ssl/secure.local.crt`
- `/etc/nginx/ssl/secure.local.key`

## Authentication

### HTTP Basic Auth

```bash
# Create auth directory
mkdir -p auth

# Create password file
htpasswd -c auth/.htpasswd admin

# Mount the directory
docker run -d \
  --name nginx-ingress \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/auth:/etc/nginx/auth:ro \
  local-nginx-ingress
```

### Container with Auth

```bash
docker run -d --name protected-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=protected.local" \
  --label "nginx.ingress.auth=basic" \
  my-app:latest
```

## Monitoring and Logging

### Health Check

```bash
# Check container health
curl http://localhost/health

# Check specific service
curl http://your-service.local
```

### Logs

```bash
# Container logs
docker logs -f nginx-ingress

# Nginx access logs
docker exec nginx-ingress tail -f /var/log/nginx/access.log

# Nginx error logs
docker exec nginx-ingress tail -f /var/log/nginx/error.log
```

### Supervisor Status

```bash
# Check supervisor status
docker exec nginx-ingress supervisorctl status

# Restart nginx
docker exec nginx-ingress supervisorctl restart nginx

# Restart ingress controller
docker exec nginx-ingress supervisorctl restart ingress-controller
```

## Troubleshooting

### Common Issues

1. **Docker socket permission denied**
   ```bash
   # Add user to docker group
   sudo usermod -aG docker $USER
   
   # Or run with sudo
   sudo ./scripts/run.sh
   ```

2. **Port already in use**
   ```bash
   # Use different ports
   ./scripts/run.sh --http-port 8080 --https-port 8443
   ```

3. **Container not found**
   ```bash
   # Check if container is running
   docker ps -a
   
   # Check logs
   docker logs nginx-ingress
   ```

### Debug Mode

```bash
# Run in foreground with logs
./scripts/run.sh --foreground

# Shell into container
docker exec -it nginx-ingress /bin/sh

# Check nginx configuration
docker exec nginx-ingress nginx -t

# Reload nginx manually
docker exec nginx-ingress nginx -s reload
```

## Production Considerations

### Security

- Run with read-only Docker socket
- Use specific user instead of root
- Implement proper SSL certificates
- Set up monitoring and alerting

### Performance

- Adjust nginx worker processes
- Configure appropriate buffer sizes
- Implement rate limiting
- Use caching where appropriate

### High Availability

- Run multiple instances behind a load balancer
- Use external certificate management
- Implement health checks and auto-restart
- Monitor container and nginx metrics