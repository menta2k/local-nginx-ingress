# Build stage
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates (needed for go modules)
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o local-nginx-ingress .

# Runtime stage
FROM nginx:alpine

# Install necessary tools
RUN apk add --no-cache curl jq openssl

# Create necessary directories (app directory only, others created by Go code)
RUN mkdir -p /app

# Forward nginx logs to docker log collector (default nginx behavior)
RUN ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log

# Copy the compiled application
COPY --from=builder /app/local-nginx-ingress /app/

# Copy nginx configuration
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/default.conf /etc/nginx/conf.d/default.conf

# Copy nginx template
COPY templates/ /app/templates/

# Copy entrypoint script
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set environment variables
ENV NGINX_CONFIG_PATH=/etc/nginx/conf.d/docker-ingress.conf
ENV NGINX_BINARY=nginx
ENV DOCKER_HOST=unix:///var/run/docker.sock

# Expose ports
EXPOSE 80 443 8080

# Health check using our health monitoring endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Use entrypoint script for pre-flight checks
ENTRYPOINT ["/entrypoint.sh"]