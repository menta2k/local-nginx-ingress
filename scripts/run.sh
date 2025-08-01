#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_NAME="nginx-ingress"
IMAGE_NAME="local-nginx-ingress:latest"
HTTP_PORT=${HTTP_PORT:-80}
HTTPS_PORT=${HTTPS_PORT:-443}

echo -e "${BLUE}🚀 Running Local Nginx Ingress Controller${NC}"
echo "========================================="

# Parse command line arguments
DETACH=true
REMOVE_EXISTING=false
LOGS=false
STOP=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --foreground|-f)
            DETACH=false
            shift
            ;;
        --remove-existing|-r)
            REMOVE_EXISTING=true
            shift
            ;;
        --logs|-l)
            LOGS=true
            shift
            ;;
        --stop|-s)
            STOP=true
            shift
            ;;
        --http-port)
            HTTP_PORT="$2"
            shift 2
            ;;
        --https-port)
            HTTPS_PORT="$2"
            shift 2
            ;;
        --name)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        --image)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -f, --foreground      Run in foreground (default: background)"
            echo "  -r, --remove-existing Remove existing container before starting"
            echo "  -l, --logs            Show logs after starting"
            echo "  -s, --stop            Stop and remove the container"
            echo "  --http-port PORT      HTTP port (default: 80)"
            echo "  --https-port PORT     HTTPS port (default: 443)"
            echo "  --name NAME           Container name (default: nginx-ingress)"
            echo "  --image IMAGE         Docker image (default: local-nginx-ingress:latest)"
            echo "  -h, --help            Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Function to stop and remove container
stop_container() {
    echo -e "${YELLOW}🛑 Stopping container...${NC}"
    if docker ps -q -f name="^${CONTAINER_NAME}$" | grep -q .; then
        docker stop "${CONTAINER_NAME}"
        echo -e "${GREEN}✅ Container stopped${NC}"
    else
        echo -e "${YELLOW}ℹ️  Container is not running${NC}"
    fi
    
    if docker ps -aq -f name="^${CONTAINER_NAME}$" | grep -q .; then
        docker rm "${CONTAINER_NAME}"
        echo -e "${GREEN}✅ Container removed${NC}"
    fi
}

# Stop container if requested
if [[ "$STOP" == "true" ]]; then
    stop_container
    exit 0
fi

# Remove existing container if requested
if [[ "$REMOVE_EXISTING" == "true" ]]; then
    stop_container
fi

# Check if container already exists
if docker ps -aq -f name="^${CONTAINER_NAME}$" | grep -q .; then
    if docker ps -q -f name="^${CONTAINER_NAME}$" | grep -q .; then
        echo -e "${YELLOW}ℹ️  Container '${CONTAINER_NAME}' is already running${NC}"
        echo -e "${YELLOW}Use --remove-existing to recreate or --logs to view logs${NC}"
        
        if [[ "$LOGS" == "true" ]]; then
            echo -e "${BLUE}📋 Showing logs...${NC}"
            docker logs -f "${CONTAINER_NAME}"
        fi
        exit 0
    else
        echo -e "${YELLOW}ℹ️  Starting existing container...${NC}"
        docker start "${CONTAINER_NAME}"
        
        if [[ "$LOGS" == "true" ]]; then
            echo -e "${BLUE}📋 Showing logs...${NC}"
            docker logs -f "${CONTAINER_NAME}"
        fi
        exit 0
    fi
fi

# Check if Docker socket is accessible
if [[ ! -S /var/run/docker.sock ]]; then
    echo -e "${RED}❌ Docker socket not found at /var/run/docker.sock${NC}"
    echo -e "${YELLOW}Make sure Docker is running${NC}"
    exit 1
fi

# Check if ports are available
if lsof -Pi :${HTTP_PORT} -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${RED}❌ Port ${HTTP_PORT} is already in use${NC}"
    exit 1
fi

if lsof -Pi :${HTTPS_PORT} -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${RED}❌ Port ${HTTPS_PORT} is already in use${NC}"
    exit 1
fi

echo -e "${YELLOW}Configuration:${NC}"
echo "  Container: ${CONTAINER_NAME}"
echo "  Image: ${IMAGE_NAME}"
echo "  HTTP Port: ${HTTP_PORT}"
echo "  HTTPS Port: ${HTTPS_PORT}"
echo "  Mode: $([ "$DETACH" == "true" ] && echo "Background" || echo "Foreground")"
echo ""

# Build run command
RUN_ARGS="--name ${CONTAINER_NAME}"
RUN_ARGS="$RUN_ARGS -p ${HTTP_PORT}:80"
RUN_ARGS="$RUN_ARGS -p ${HTTPS_PORT}:443"
RUN_ARGS="$RUN_ARGS -v /var/run/docker.sock:/var/run/docker.sock:ro"

# Create directories for SSL and auth if they don't exist
mkdir -p ssl auth logs

RUN_ARGS="$RUN_ARGS -v $(pwd)/ssl:/etc/nginx/ssl:ro"
RUN_ARGS="$RUN_ARGS -v $(pwd)/auth:/etc/nginx/auth:ro"
RUN_ARGS="$RUN_ARGS -v $(pwd)/logs:/var/log/nginx"

if [[ "$DETACH" == "true" ]]; then
    RUN_ARGS="$RUN_ARGS -d"
fi

# Run the container
echo -e "${BLUE}🚀 Starting container...${NC}"
docker run $RUN_ARGS "${IMAGE_NAME}"

if [[ $? -eq 0 ]]; then
    if [[ "$DETACH" == "true" ]]; then
        echo -e "${GREEN}✅ Container started successfully!${NC}"
        echo ""
        echo -e "${YELLOW}📋 Container information:${NC}"
        docker ps -f name="^${CONTAINER_NAME}$" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        echo ""
        echo -e "${YELLOW}💡 Useful commands:${NC}"
        echo "  View logs:    docker logs -f ${CONTAINER_NAME}"
        echo "  Stop:         docker stop ${CONTAINER_NAME}"
        echo "  Remove:       docker rm ${CONTAINER_NAME}"
        echo "  Shell:        docker exec -it ${CONTAINER_NAME} /bin/sh"
        echo ""
        echo -e "${YELLOW}🌐 Access URLs:${NC}"
        echo "  HTTP:         http://localhost:${HTTP_PORT}"
        echo "  HTTPS:        https://localhost:${HTTPS_PORT}"
        echo "  Health:       http://localhost:${HTTP_PORT}/health"
        
        if [[ "$LOGS" == "true" ]]; then
            echo ""
            echo -e "${BLUE}📋 Showing logs...${NC}"
            docker logs -f "${CONTAINER_NAME}"
        fi
    else
        echo -e "${GREEN}✅ Container started in foreground mode${NC}"
    fi
else
    echo -e "${RED}❌ Failed to start container!${NC}"
    exit 1
fi