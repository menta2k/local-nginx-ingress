# Nginx Ingress Snippets Demo

This example demonstrates how to use file-based configuration snippets with the Local Nginx Ingress Controller, similar to nginx-ingress controller annotations but with improved maintainability.

## Features

- **File-based configuration**: Instead of inline annotations, reference files in your containers
- **Two snippet types**:
  - `configuration-snippet`: Included in location blocks (like location-level config)
  - `server-snippet`: Included in server blocks (like server-level config)
- **Automatic download**: Controller downloads files from containers and caches them
- **Security validation**: Only allows safe file paths and validates nginx syntax

## Example Container with Snippets

### 1. Create a Docker Container with Config Files

```dockerfile
FROM nginx:alpine

# Copy your custom nginx configurations
COPY location.conf /app/config/location.conf
COPY server.conf /app/config/server.conf

# Copy your application
COPY app/ /usr/share/nginx/html/

# Add labels for nginx ingress with snippet references
LABEL nginx.ingress.enable="true"
LABEL nginx.ingress.host="snippets-demo.local"
LABEL nginx.ingress.port="80"
LABEL nginx.ingress.path="/api"
LABEL nginx.ingress.configuration-snippet="/app/config/location.conf"
LABEL nginx.ingress.server-snippet="/app/config/server.conf"
```

### 2. Run with Docker Run Command

```bash
# Create a test container with snippet files
docker run -d --name snippet-demo \
  --network local-nginx-ingress_nginx-ingress \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=snippets-demo.local" \
  --label "nginx.ingress.port=80" \
  --label "nginx.ingress.path=/api" \
  --label "nginx.ingress.configuration-snippet=/app/config/location.conf" \
  --label "nginx.ingress.server-snippet=/app/config/server.conf" \
  -v $(pwd)/examples/snippets:/app/config:ro \
  nginx:alpine
```

### 3. Docker Compose Example

```yaml
services:
  advanced-api:
    image: nginx:alpine
    volumes:
      - ./examples/snippets:/app/config:ro
      - ./examples/api:/usr/share/nginx/html:ro
    labels:
      - "nginx.ingress.enable=true"
      - "nginx.ingress.host=advanced-api.local"
      - "nginx.ingress.port=80"
      - "nginx.ingress.path=/api/v2"
      - "nginx.ingress.configuration-snippet=/app/config/location.conf"
      - "nginx.ingress.server-snippet=/app/config/server.conf"
    networks:
      - nginx-ingress
```

## Generated Nginx Configuration

The ingress controller will generate nginx configuration like this:

```nginx
server {
    listen 80;
    server_name snippets-demo.local;
    
    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    
    # Custom server configuration (from server.conf)
    error_page 404 /custom_404.html;
    error_page 500 502 503 504 /custom_50x.html;
    limit_req_zone $binary_remote_addr zone=server_limit:10m rate=30r/m;
    gzip on;
    gzip_types text/plain application/json application/javascript text/css;
    client_max_body_size 50M;
    client_body_timeout 60s;
    
    location /api {
        # Proxy settings
        proxy_pass http://backend_snippets-demo_local_snippet-demo;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # Custom location configuration (from location.conf)
        limit_req zone=api burst=10 nodelay;
        add_header X-API-Version "v2.0";
        add_header X-Custom-Feature "snippets-enabled";
        proxy_cache_valid 200 302 10m;
        proxy_cache_valid 404 1m;
        proxy_read_timeout 120s;
        access_log /var/log/nginx/api_access.log main;
    }
}
```

## Benefits

1. **Maintainable**: Configuration files can be version controlled separately
2. **Reusable**: Same config files can be used across multiple containers
3. **Flexible**: Full nginx configuration power without annotation limitations
4. **Secure**: File path validation and syntax checking
5. **Cached**: Downloaded files are cached for performance

## Label Reference

| Label | Description | Example |
|-------|-------------|---------|
| `nginx.ingress.configuration-snippet` | Path to location-level config file | `/app/config/location.conf` |
| `nginx.ingress.server-snippet` | Path to server-level config file | `/app/config/server.conf` |

## Security Notes

- Only `.conf` and `.txt` files are allowed
- Path traversal (`..`) is blocked
- System directories (`/etc`, `/var`) are blocked
- Basic nginx syntax validation is performed
- Files are cached to avoid repeated downloads

## Testing

```bash
# Add host entry
echo "127.0.0.1 snippets-demo.local" | sudo tee -a /etc/hosts

# Test the service
curl -H "Host: snippets-demo.local" http://localhost:8080/api

# Check custom headers
curl -I -H "Host: snippets-demo.local" http://localhost:8080/api
```

You should see custom headers like:
- `X-API-Version: v2.0`
- `X-Custom-Feature: snippets-enabled`