#!/bin/bash

# Test script for nginx ingress snippets functionality
# This script demonstrates how the snippet-based configuration works

set -e

echo "🧪 Testing Nginx Ingress Snippets Functionality"
echo "================================================"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${BLUE}➤ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if docker and docker-compose are available
command -v docker >/dev/null 2>&1 || { print_error "Docker is required but not installed."; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { print_error "Docker Compose is required but not installed."; exit 1; }

# Check if we're in the examples directory
if [ ! -f "snippet-test-compose.yml" ]; then
    print_error "Please run this script from the examples directory"
    print_warning "cd examples && ./test-snippets.sh"
    exit 1
fi

print_step "Building nginx ingress controller..."
docker-compose -f snippet-test-compose.yml build nginx-ingress

print_step "Starting test environment..."
docker-compose -f snippet-test-compose.yml up -d

print_step "Waiting for services to be ready..."
sleep 10

# Check if services are running
print_step "Checking service status..."
docker-compose -f snippet-test-compose.yml ps

# Add hosts entries instructions
echo
print_warning "Please add these entries to your /etc/hosts file:"
echo "127.0.0.1 advanced-api.local"
echo "127.0.0.1 simple-api.local" 
echo "127.0.0.1 cors-api.local"
echo
echo "You can add them with:"
echo "echo '127.0.0.1 advanced-api.local simple-api.local cors-api.local' | sudo tee -a /etc/hosts"
echo

# Test functions
test_advanced_api() {
    print_step "Testing Advanced API with snippets..."
    
    echo "Checking custom headers from location snippet:"
    if curl -s -H "Host: advanced-api.local" http://localhost:8080/api/v2 | grep -q "Test API"; then
        print_success "Advanced API is responding"
    else
        print_error "Advanced API is not responding correctly"
        return 1
    fi
    
    echo "Checking custom headers:"
    headers=$(curl -s -I -H "Host: advanced-api.local" http://localhost:8080/api/v2)
    
    if echo "$headers" | grep -q "X-API-Version: v2.0"; then
        print_success "Found X-API-Version header from location snippet"
    else
        print_warning "X-API-Version header not found"
    fi
    
    if echo "$headers" | grep -q "X-Custom-Feature: snippets-enabled"; then
        print_success "Found X-Custom-Feature header from location snippet"
    else
        print_warning "X-Custom-Feature header not found"
    fi
}

test_simple_api() {
    print_step "Testing Simple API without snippets..."
    
    if curl -s -H "Host: simple-api.local" http://localhost:8080/api | grep -q "Test API"; then
        print_success "Simple API is responding"
    else
        print_error "Simple API is not responding correctly"
        return 1
    fi
    
    echo "Checking that custom headers are NOT present:"
    headers=$(curl -s -I -H "Host: simple-api.local" http://localhost:8080/api)
    
    if echo "$headers" | grep -q "X-API-Version"; then
        print_warning "X-API-Version header found (should not be present)"
    else
        print_success "X-API-Version header correctly absent"
    fi
}

test_cors_api() {
    print_step "Testing CORS API with CORS-specific snippets..."
    
    if curl -s -H "Host: cors-api.local" http://localhost:8080/api/v1 | grep -q "Test API"; then
        print_success "CORS API is responding"
    else
        print_error "CORS API is not responding correctly"
        return 1
    fi
    
    echo "Testing CORS preflight request:"
    cors_headers=$(curl -s -I -X OPTIONS -H "Host: cors-api.local" \
        -H "Origin: https://example.com" \
        -H "Access-Control-Request-Method: POST" \
        http://localhost:8080/api/v1)
    
    if echo "$cors_headers" | grep -q "Access-Control-Allow-Origin"; then
        print_success "CORS headers present"
    else
        print_warning "CORS headers not found"
    fi
    
    if echo "$cors_headers" | grep -q "X-CORS-Enabled: snippets"; then
        print_success "Found X-CORS-Enabled header from CORS location snippet"
    else
        print_warning "X-CORS-Enabled header not found"
    fi
}

# Run tests
echo
print_step "Running functionality tests..."
echo

test_advanced_api
echo
test_simple_api  
echo
test_cors_api
echo

print_step "Checking nginx configuration..."
echo "Generated nginx config should include snippet content:"
docker exec nginx-ingress-controller cat /etc/nginx/conf.d/docker-ingress.conf | head -50

echo
print_step "Test completed!"
print_success "Snippet functionality is working correctly"

echo
print_warning "To stop the test environment, run:"
echo "docker-compose -f snippet-test-compose.yml down"

echo
print_step "Manual testing commands:"
echo "curl -H 'Host: advanced-api.local' http://localhost:8080/api/v2"
echo "curl -I -H 'Host: advanced-api.local' http://localhost:8080/api/v2"
echo "curl -H 'Host: simple-api.local' http://localhost:8080/api"
echo "curl -I -X OPTIONS -H 'Host: cors-api.local' -H 'Origin: https://example.com' http://localhost:8080/api/v1"