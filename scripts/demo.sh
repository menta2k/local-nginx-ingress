#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🎬 Local Nginx Ingress Demo${NC}"
echo "=========================="
echo ""

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ docker-compose is required but not installed${NC}"
    echo -e "${YELLOW}Please install docker-compose and try again${NC}"
    exit 1
fi

# Parse command line arguments
ACTION="start"
FOLLOW_LOGS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        start)
            ACTION="start"
            shift
            ;;
        stop)
            ACTION="stop"
            shift
            ;;
        restart)
            ACTION="restart"
            shift
            ;;
        logs)
            ACTION="logs"
            shift
            ;;
        status)
            ACTION="status"
            shift
            ;;
        clean)
            ACTION="clean"
            shift
            ;;
        -f|--follow)
            FOLLOW_LOGS=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [ACTION] [OPTIONS]"
            echo ""
            echo "Actions:"
            echo "  start     Start the demo environment (default)"
            echo "  stop      Stop the demo environment"
            echo "  restart   Restart the demo environment"
            echo "  logs      Show logs"
            echo "  status    Show status of services"
            echo "  clean     Clean up everything"
            echo ""
            echo "Options:"
            echo "  -f, --follow    Follow logs (for logs action)"
            echo "  -h, --help      Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

case $ACTION in
    start)
        echo -e "${BLUE}🚀 Starting demo environment...${NC}"
        
        # Add hosts entries
        echo -e "${YELLOW}📝 Adding hosts entries...${NC}"
        
        HOSTS_ENTRIES=(
            "127.0.0.1 webapp.local"
            "127.0.0.1 api.local"
            "127.0.0.1 secure.local"
            "127.0.0.1 service.local"
            "127.0.0.1 cors-api.local"
        )
        
        for entry in "${HOSTS_ENTRIES[@]}"; do
            if ! grep -q "$entry" /etc/hosts 2>/dev/null; then
                echo "  Adding: $entry"
                echo "$entry" | sudo tee -a /etc/hosts > /dev/null
            else
                echo "  Already exists: $entry"
            fi
        done
        
        # Start services
        echo -e "${BLUE}🐳 Starting Docker services...${NC}"
        docker-compose up -d --build
        
        # Wait for services to be ready
        echo -e "${YELLOW}⏳ Waiting for services to be ready...${NC}"
        sleep 10
        
        # Show status
        echo -e "${GREEN}✅ Demo environment started!${NC}"
        echo ""
        echo -e "${YELLOW}📊 Service Status:${NC}"
        docker-compose ps
        echo ""
        echo -e "${YELLOW}🌐 Available Services:${NC}"
        echo "  • Web App:      http://webapp.local"
        echo "  • API Service:  http://api.local/api"
        echo "  • Secure App:   https://secure.local (self-signed cert)"
        echo "  • Microservice: http://service.local/service"
        echo "  • CORS API:     http://cors-api.local/api/v1"
        echo ""
        echo -e "${YELLOW}🔍 Health Check:${NC}"
        echo "  • Health:       http://localhost/health"
        echo ""
        echo -e "${YELLOW}💡 Useful Commands:${NC}"
        echo "  • View logs:    $0 logs"
        echo "  • Check status: $0 status"
        echo "  • Stop demo:    $0 stop"
        echo "  • Clean up:     $0 clean"
        
        if [[ "$FOLLOW_LOGS" == "true" ]]; then
            echo ""
            echo -e "${BLUE}📋 Following logs...${NC}"
            docker-compose logs -f
        fi
        ;;
        
    stop)
        echo -e "${YELLOW}🛑 Stopping demo environment...${NC}"
        docker-compose down
        echo -e "${GREEN}✅ Demo environment stopped${NC}"
        ;;
        
    restart)
        echo -e "${YELLOW}🔄 Restarting demo environment...${NC}"
        docker-compose down
        docker-compose up -d --build
        echo -e "${GREEN}✅ Demo environment restarted${NC}"
        ;;
        
    logs)
        if [[ "$FOLLOW_LOGS" == "true" ]]; then
            echo -e "${BLUE}📋 Following logs...${NC}"
            docker-compose logs -f
        else
            echo -e "${BLUE}📋 Recent logs:${NC}"
            docker-compose logs --tail=50
        fi
        ;;
        
    status)
        echo -e "${BLUE}📊 Service Status:${NC}"
        docker-compose ps
        echo ""
        echo -e "${BLUE}🔍 Health Checks:${NC}"
        
        SERVICES=(
            "http://localhost/health:Nginx Ingress"
            "http://webapp.local:Web App"
            "http://api.local/api:API Service"
            "http://service.local/service:Microservice"
            "http://cors-api.local/api/v1:CORS API"
        )
        
        for service in "${SERVICES[@]}"; do
            url="${service%:*}"
            name="${service#*:}"
            
            if curl -s -f "$url" > /dev/null 2>&1; then
                echo -e "  • ${name}: ${GREEN}✅ Healthy${NC}"
            else
                echo -e "  • ${name}: ${RED}❌ Unhealthy${NC}"
            fi
        done
        ;;
        
    clean)
        echo -e "${YELLOW}🧹 Cleaning up demo environment...${NC}"
        
        # Stop and remove containers
        docker-compose down -v --remove-orphans
        
        # Remove images
        echo -e "${YELLOW}🗑️  Removing demo images...${NC}"
        docker-compose down --rmi all
        
        # Remove hosts entries
        echo -e "${YELLOW}📝 Removing hosts entries...${NC}"
        HOSTS_ENTRIES=(
            "webapp.local"
            "api.local"
            "secure.local"
            "service.local"
            "cors-api.local"
        )
        
        for host in "${HOSTS_ENTRIES[@]}"; do
            if grep -q "$host" /etc/hosts 2>/dev/null; then
                echo "  Removing: $host"
                sudo sed -i "/$host/d" /etc/hosts
            fi
        done
        
        # Clean up directories
        echo -e "${YELLOW}🗂️  Cleaning up directories...${NC}"
        rm -rf logs/* ssl/* auth/*
        
        echo -e "${GREEN}✅ Demo environment cleaned up!${NC}"
        ;;
        
    *)
        echo -e "${RED}Unknown action: $ACTION${NC}"
        exit 1
        ;;
esac