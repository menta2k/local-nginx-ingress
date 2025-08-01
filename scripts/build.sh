#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
IMAGE_NAME="local-nginx-ingress"
VERSION=${VERSION:-"latest"}
REGISTRY=${REGISTRY:-""}

echo -e "${BLUE}🐳 Building Local Nginx Ingress Controller${NC}"
echo "======================================"

# Parse command line arguments
PUSH=false
PLATFORM=""
CACHE_FROM=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --push)
            PUSH=true
            shift
            ;;
        --platform)
            PLATFORM="--platform $2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --registry)
            REGISTRY="$2"
            shift 2
            ;;
        --cache-from)
            CACHE_FROM="--cache-from $2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --push              Push image to registry after build"
            echo "  --platform PLATFORM  Target platform (e.g., linux/amd64,linux/arm64)"
            echo "  --version VERSION   Image version (default: latest)"
            echo "  --registry REGISTRY Registry prefix (e.g., myregistry.com/)"
            echo "  --cache-from IMAGE  Use image as cache source"
            echo "  -h, --help          Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Construct full image name
FULL_IMAGE_NAME="${REGISTRY}${IMAGE_NAME}:${VERSION}"

echo -e "${YELLOW}Configuration:${NC}"
echo "  Image: ${FULL_IMAGE_NAME}"
echo "  Platform: ${PLATFORM:-default}"
echo "  Push: ${PUSH}"
echo "  Cache from: ${CACHE_FROM:-none}"
echo ""

# Build the Docker image
echo -e "${BLUE}🔨 Building Docker image...${NC}"

BUILD_ARGS="--tag ${FULL_IMAGE_NAME}"

if [[ -n "$PLATFORM" ]]; then
    BUILD_ARGS="$BUILD_ARGS $PLATFORM"
fi

if [[ -n "$CACHE_FROM" ]]; then
    BUILD_ARGS="$BUILD_ARGS $CACHE_FROM"
fi

# Add build metadata
BUILD_ARGS="$BUILD_ARGS --label org.opencontainers.image.created=$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
BUILD_ARGS="$BUILD_ARGS --label org.opencontainers.image.version=${VERSION}"
BUILD_ARGS="$BUILD_ARGS --label org.opencontainers.image.title='Local Nginx Ingress Controller'"
BUILD_ARGS="$BUILD_ARGS --label org.opencontainers.image.description='Docker-based nginx ingress controller with label-based configuration'"

docker build $BUILD_ARGS .

if [[ $? -eq 0 ]]; then
    echo -e "${GREEN}✅ Docker image built successfully!${NC}"
else
    echo -e "${RED}❌ Docker build failed!${NC}"
    exit 1
fi

# Show image size
echo -e "${YELLOW}📊 Image information:${NC}"
docker images "${FULL_IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

# Push if requested
if [[ "$PUSH" == "true" ]]; then
    echo -e "${BLUE}📤 Pushing image to registry...${NC}"
    docker push "${FULL_IMAGE_NAME}"
    
    if [[ $? -eq 0 ]]; then
        echo -e "${GREEN}✅ Image pushed successfully!${NC}"
    else
        echo -e "${RED}❌ Failed to push image!${NC}"
        exit 1
    fi
fi

echo ""
echo -e "${GREEN}🎉 Build completed successfully!${NC}"
echo -e "${YELLOW}Run with:${NC} docker run -d -p 80:80 -p 443:443 -v /var/run/docker.sock:/var/run/docker.sock ${FULL_IMAGE_NAME}"