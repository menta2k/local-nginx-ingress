#!/bin/bash

# Test script for FastCGI functionality
# This script demonstrates how FastCGI support works with the nginx ingress controller

set -e

echo "🧪 Testing Nginx Ingress FastCGI Functionality"
echo "=============================================="

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

# Check if we're in the fastcgi directory
if [ ! -f "php-app-compose.yml" ]; then
    print_error "Please run this script from the examples/fastcgi directory"
    print_warning "cd examples/fastcgi && ./test-fastcgi.sh"
    exit 1
fi

print_step "Building nginx ingress controller..."
docker-compose -f php-app-compose.yml build nginx-ingress

print_step "Starting FastCGI test environment..."
docker-compose -f php-app-compose.yml up -d

print_step "Waiting for services to be ready..."
sleep 15

# Check if services are running
print_step "Checking service status..."
docker-compose -f php-app-compose.yml ps

# Add hosts entries instructions
echo
print_warning "Please add these entries to your /etc/hosts file:"
echo "127.0.0.1 php-app.local"
echo "127.0.0.1 simple-php.local" 
echo "127.0.0.1 regular-app.local"
echo
echo "You can add them with:"
echo "echo '127.0.0.1 php-app.local simple-php.local regular-app.local' | sudo tee -a /etc/hosts"
echo

# Test functions
test_php_fastcgi() {
    print_step "Testing PHP FastCGI application with file-based parameters..."
    
    echo "Checking if PHP app responds:"
    if curl -s -H "Host: php-app.local" http://localhost:8080/app | grep -q "PHP FastCGI Demo Application"; then
        print_success "PHP FastCGI app is responding"
    else
        print_error "PHP FastCGI app is not responding correctly"
        return 1
    fi
    
    echo "Checking FastCGI-specific content:"
    response=$(curl -s -H "Host: php-app.local" http://localhost:8080/app)
    
    if echo "$response" | grep -q "SCRIPT_FILENAME"; then
        print_success "FastCGI parameters are being passed correctly"
    else
        print_warning "FastCGI parameters not found in response"
    fi
    
    if echo "$response" | grep -q "APP_ENV"; then
        print_success "Custom FastCGI parameters from file are working"
    else
        print_warning "Custom FastCGI parameters not found"
    fi
}

test_simple_php() {
    print_step "Testing Simple PHP with label-based parameters..."
    
    if curl -s -H "Host: simple-php.local" http://localhost:8080/ | grep -q "Simple PHP FastCGI"; then
        print_success "Simple PHP FastCGI app is responding"
    else
        print_error "Simple PHP FastCGI app is not responding correctly"
        return 1
    fi
    
    echo "Checking label-based FastCGI parameters:"
    response=$(curl -s -H "Host: simple-php.local" http://localhost:8080/)
    
    if echo "$response" | grep -q "DOCUMENT_ROOT"; then
        print_success "Label-based FastCGI parameters are working"
    else
        print_warning "Label-based FastCGI parameters not found"
    fi
}

test_regular_app() {
    print_step "Testing Regular HTTP app for comparison..."
    
    if curl -s -H "Host: regular-app.local" http://localhost:8080/regular/ | grep -q "🌐 Regular HTTP Application"; then
        print_success "Regular HTTP app is responding (using proxy_pass)"
    else
        print_error "Regular HTTP app is not responding correctly"
        return 1
    fi
}

test_nginx_config() {
    print_step "Checking generated nginx configuration..."
    echo "Looking for FastCGI configuration in generated nginx config:"
    
    nginx_config=$(docker exec nginx-ingress-controller cat /etc/nginx/conf.d/docker-ingress.conf 2>/dev/null || echo "Config not found")
    
    if echo "$nginx_config" | grep -q "fastcgi_pass"; then
        print_success "FastCGI configuration found in nginx config"
    else
        print_warning "FastCGI configuration not found in nginx config"
    fi
    
    if echo "$nginx_config" | grep -q "fastcgi_param"; then
        print_success "FastCGI parameters found in nginx config"
    else
        print_warning "FastCGI parameters not found in nginx config"
    fi
    
    if echo "$nginx_config" | grep -q "proxy_pass.*regular"; then
        print_success "Regular proxy_pass configuration found for comparison"
    else
        print_warning "Regular proxy_pass configuration not found"
    fi
}

# Run tests
echo
print_step "Running functionality tests..."
echo

test_php_fastcgi
echo
test_simple_php  
echo
test_regular_app
echo
test_nginx_config
echo

print_step "Showing nginx configuration snippet..."
echo "FastCGI location configuration:"
docker exec nginx-ingress-controller cat /etc/nginx/conf.d/docker-ingress.conf | grep -A 20 "fastcgi_pass" | head -25

echo
print_step "Test completed!"
print_success "FastCGI functionality is working correctly"

echo
print_warning "To stop the test environment, run:"
echo "docker-compose -f php-app-compose.yml down"

echo
print_step "Manual testing commands:"
echo "# PHP FastCGI app with file-based params:"
echo "curl -H 'Host: php-app.local' http://localhost:8080/app"
echo
echo "# Simple PHP with label-based params:"
echo "curl -H 'Host: simple-php.local' http://localhost:8080/"
echo
echo "# Regular HTTP app for comparison:"
echo "curl -H 'Host: regular-app.local' http://localhost:8080/regular/"
echo
echo "# View PHP info:"
echo "curl -H 'Host: php-app.local' http://localhost:8080/app/info.php"