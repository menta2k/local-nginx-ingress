# Local Nginx Ingress Controller

A high-performance Docker-based nginx ingress controller that automatically configures nginx based on container labels. Built with Go and direct nginx process management for optimal performance and reliability.

> ⚠️ **DISCLAIMER: This software is currently in development and is NOT ready for production use.** 
> 
> This project is intended for development, testing, and learning purposes only. It may contain bugs, security vulnerabilities, or incomplete features. Use at your own risk and do not deploy in production environments without thorough testing and security review.

## Features

- 🐳 **Docker Integration**: Monitors Docker containers with label-based configuration
- 🔄 **Auto-Reload**: Automatically reloads nginx when containers start/stop
- 🏷️ **Label-Based Config**: Uses Docker labels similar to Traefik for service discovery
- 🚀 **Direct Process Management**: Controls nginx directly without supervisord overhead
- 📝 **External Templates**: Customizable nginx configuration templates
- 🔒 **SSL/TLS Support**: Automatic SSL configuration with custom certificates
- ⚖️ **Load Balancing**: Support for different load balancing methods
- 🏥 **Health Checks**: Built-in health check configuration
- 🔐 **Authentication**: HTTP Basic/Digest authentication support
- 🌐 **CORS**: Configurable CORS headers
- 🧩 **FastCGI Support**: Built-in PHP FastCGI application support
- 📊 **Unified Logging**: All logs visible through `docker logs`
- 📊 **Monitoring**: Real-time monitoring of container lifecycle events

## Quick Start

> 🧪 **For Development/Testing Only**: These examples are for development and testing environments only.

### 1. Build and Run

```bash
# Build the application
go build -o local-nginx-ingress .

# Run with default settings
sudo ./local-nginx-ingress
```

### 2. Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NGINX_CONFIG_PATH` | `/etc/nginx/conf.d/docker-ingress.conf` | Path to nginx config file |
| `NGINX_BINARY` | `nginx` | Nginx binary path |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker API socket |
| `SNIPPET_CACHE_DIR` | `/tmp/nginx-ingress-snippets` | Directory for configuration snippets |

### 3. Docker Usage (Recommended)

```bash
# Run with Docker (easiest method)
docker run -d \
  --name nginx-ingress \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  local-nginx-ingress:latest
```

### 4. Run a Test Container

```bash
# Simple web application
docker run -d --name my-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=myapp.local" \
  --label "nginx.ingress.port=80" \
  nginx:alpine

# Add to /etc/hosts
echo "127.0.0.1 myapp.local" | sudo tee -a /etc/hosts

# Test
curl http://myapp.local
```

## Label Configuration

All labels use the prefix `nginx.ingress.` followed by the configuration key.

### Core Labels

| Label | Required | Default | Description |
|-------|----------|---------|-------------|
| `nginx.ingress.enable` | ✅ | - | Enable nginx ingress (`true`/`false`) |
| `nginx.ingress.host` | ✅ | - | Hostname for the service |
| `nginx.ingress.port` | ❌ | `80` | Container port to proxy to |
| `nginx.ingress.path` | ❌ | `/` | URL path prefix |
| `nginx.ingress.protocol` | ❌ | `http` | Protocol (`http`/`https`) |
| `nginx.ingress.priority` | ❌ | `100` | Location matching priority |

### SSL/TLS Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.tls` | Enable TLS/SSL (`true`/`false`) |
| `nginx.ingress.tls.certname` | SSL certificate name |

### Load Balancing Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.loadbalancer.method` | Method: `round_robin`, `least_conn`, `ip_hash` |

### Health Check Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.healthcheck` | Enable health checks (`true`/`false`) |
| `nginx.ingress.healthcheck.path` | Health check endpoint (default: `/health`) |

### Authentication Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.auth` | Auth type: `basic`, `digest` |

### CORS Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.cors` | Enable CORS (`true`/`false`) |
| `nginx.ingress.cors.origins` | Allowed origins (comma-separated) |
| `nginx.ingress.cors.methods` | Allowed methods (comma-separated) |

### FastCGI Labels

| Label | Description |
|-------|-------------|
| `nginx.ingress.backend-protocol` | Set to `FCGI` for FastCGI applications |
| `nginx.ingress.fastcgi-index` | FastCGI index file (e.g., `index.php`) |
| `nginx.ingress.fastcgi-params` | Custom FastCGI parameters (comma-separated) |

### Configuration Snippets

| Label | Description |
|-------|-------------|
| `nginx.ingress.configuration-snippet` | URL to custom nginx location configuration |
| `nginx.ingress.server-snippet` | URL to custom nginx server configuration |

## Usage Examples

### Simple Web Application

```bash
docker run -d --name webapp \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=webapp.local" \
  --label "nginx.ingress.port=3000" \
  my-web-app:latest
```

### API with Authentication

```bash
docker run -d --name api \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=api.local" \
  --label "nginx.ingress.port=8080" \
  --label "nginx.ingress.path=/api" \
  --label "nginx.ingress.auth=basic" \
  --label "nginx.ingress.priority=200" \
  my-api:latest
```

### SSL-Enabled Service

```bash
docker run -d --name secure-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=secure.local" \
  --label "nginx.ingress.port=443" \
  --label "nginx.ingress.protocol=https" \
  --label "nginx.ingress.tls=true" \
  --label "nginx.ingress.tls.certname=secure.local" \
  secure-app:latest
```

### Microservice with Health Checks

```bash
docker run -d --name microservice \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=service.local" \
  --label "nginx.ingress.port=8080" \
  --label "nginx.ingress.path=/service" \
  --label "nginx.ingress.healthcheck=true" \
  --label "nginx.ingress.healthcheck.path=/health" \
  --label "nginx.ingress.loadbalancer.method=least_conn" \
  microservice:latest
```

### CORS-Enabled API

```bash
docker run -d --name cors-api \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=cors-api.local" \
  --label "nginx.ingress.port=3000" \
  --label "nginx.ingress.path=/api/v1" \
  --label "nginx.ingress.cors=true" \
  --label "nginx.ingress.cors.origins=https://app.local,https://admin.local" \
  --label "nginx.ingress.cors.methods=GET,POST,PUT,DELETE" \
  cors-api:latest
```

### PHP FastCGI Application

```bash
docker run -d --name php-app \
  --label "nginx.ingress.enable=true" \
  --label "nginx.ingress.host=php-app.local" \
  --label "nginx.ingress.port=9000" \
  --label "nginx.ingress.backend-protocol=FCGI" \
  --label "nginx.ingress.fastcgi-index=index.php" \
  --label "nginx.ingress.fastcgi-params=SCRIPT_FILENAME=/var/www/html/index.php,DOCUMENT_ROOT=/var/www/html" \
  php:8.2-fpm-alpine
```

## Docker Compose Example

```yaml
version: '3.8'

services:
  web-app:
    image: nginx:alpine
    labels:
      - "nginx.ingress.enable=true"
      - "nginx.ingress.host=webapp.local"
      - "nginx.ingress.port=80"
      - "nginx.ingress.path=/"
  
  api:
    image: my-api:latest
    labels:
      - "nginx.ingress.enable=true"
      - "nginx.ingress.host=api.local"
      - "nginx.ingress.port=8080"
      - "nginx.ingress.path=/api"
      - "nginx.ingress.auth=basic"
      - "nginx.ingress.priority=200"
  
  admin:
    image: admin-panel:latest
    labels:
      - "nginx.ingress.enable=true"
      - "nginx.ingress.host=admin.local"
      - "nginx.ingress.port=3000"
      - "nginx.ingress.path=/admin"
      - "nginx.ingress.tls=true"
      - "nginx.ingress.tls.certname=admin.local"
```

## Generated Nginx Configuration

The controller generates nginx configuration like this:

```nginx
upstream backend_webapp_local_webapp {
    server 172.17.0.2:80 weight=1;
}

server {
    listen 80;
    server_name webapp.local;
    
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    
    location / {
        proxy_pass http://backend_webapp_local_webapp;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # ... additional proxy settings
    }
}
```

## Architecture

The controller uses a streamlined architecture with direct nginx process management:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Docker API    │────│ Go Application  │────│ Nginx Process   │
│                 │    │                 │    │                 │
│ Container       │    │ • Label Parser  │    │ • Direct Control│
│ Events          │    │ • Config Gen    │    │ • SIGHUP Reload │
│ • start/stop    │    │ • Template Eng  │    │ • Graceful Stop │
│ • label change  │    │ • Process Mgmt  │    │ • Access Logs   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌────────────────┐               │
         └──────────────│ Unified Logging │───────────────┘
                        │                │
                        │ • docker logs  │
                        │ • Access Logs  │
                        │ • Error Logs   │
                        └────────────────┘
```

### Key Components

- **Go Application**: Single process managing everything
- **Nginx Manager**: Direct process control without supervisord
- **Template Engine**: External template files for customization
- **Docker Provider**: Real-time container monitoring
- **Configuration Generator**: Dynamic nginx config creation
- **FastCGI Support**: Built-in PHP and FastCGI application support

## Template Customization

The nginx configuration is generated from external template files, making it easy to customize:

### Default Template Location
- Container: `/app/templates/nginx.conf.tmpl`
- Local: `templates/nginx.conf.tmpl`

### Custom Template
```bash
# Use custom template
docker run -d \
  --name nginx-ingress \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /path/to/custom-template.tmpl:/app/templates/nginx.conf.tmpl \
  local-nginx-ingress:latest
```

### Template Variables
The template receives a `NginxConfig` struct with:
- `Upstreams`: Array of upstream configurations
- `Servers`: Array of server block configurations
- `Generated`: Timestamp of generation

## Logging

All logs are unified and visible through standard Docker logging:

```bash
# View all logs (Go app + nginx)
docker logs nginx-ingress-controller

# Follow logs in real-time
docker logs -f nginx-ingress-controller

# View only recent logs
docker logs --since 5m nginx-ingress-controller
```

### Log Types
- **Application logs**: Go application startup, configuration changes
- **Nginx access logs**: HTTP request logs in standard format
- **Nginx error logs**: Nginx process and error messages

## Development

### Building

```bash
go build -o local-nginx-ingress .
```

### Testing

```bash
go test ./...
```

### Docker Build

```bash
docker build -t local-nginx-ingress .
```

## Requirements

### For Development
- Go 1.23+
- Docker API access
- Access to `/var/run/docker.sock`

### For Docker Usage (Recommended)
- Docker runtime
- Access to Docker socket
- Ports 80/443 available

**Note**: When using the Docker container, nginx is included and managed automatically. No external nginx installation required!

## Comparison with Traefik

| Feature | Local Nginx Ingress | Traefik |
|---------|-------------------|---------|
| **Backend** | Nginx (industry standard) | Built-in Go proxy |
| **Configuration** | Docker labels | Docker labels |
| **SSL** | External certificates | Let's Encrypt + External |
| **Performance** | Nginx performance | Go proxy performance |
| **FastCGI** | Native FastCGI support | Limited support |
| **Templates** | External customizable | Built-in |
| **Process Mgmt** | Direct nginx control | Built-in proxy |
| **Logging** | Unified docker logs | Separate access logs |
| **Complexity** | Simple, focused | Feature-rich |
| **Use Case** | Nginx-specific needs | General-purpose |

## Advantages

- ✅ **Nginx Performance**: Leverages nginx's proven performance and stability
- ✅ **FastCGI Native**: Built-in support for PHP and FastCGI applications  
- ✅ **Template Flexibility**: Easily customize nginx configuration templates
- ✅ **Unified Logging**: All logs available through standard `docker logs`
- ✅ **Direct Control**: No supervisord overhead, direct process management
- ✅ **Lightweight**: Minimal resource footprint with single Go process
- ✅ **Battle-tested Core**: Uses proven nginx as the core proxy engine

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.