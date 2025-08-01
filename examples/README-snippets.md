# Nginx Ingress Snippets Testing

This directory contains a complete testing environment for the nginx ingress controller's file-based snippet functionality.

## Quick Start

1. **Run the test environment:**
   ```bash
   cd examples
   ./test-snippets.sh
   ```

2. **Add host entries:**
   ```bash
   echo '127.0.0.1 advanced-api.local simple-api.local cors-api.local' | sudo tee -a /etc/hosts
   ```

3. **Test the APIs:**
   ```bash
   # Advanced API with snippets
   curl -H 'Host: advanced-api.local' http://localhost:8080/api/v2
   curl -I -H 'Host: advanced-api.local' http://localhost:8080/api/v2
   
   # Simple API without snippets  
   curl -H 'Host: simple-api.local' http://localhost:8080/api
   
   # CORS API with CORS-specific snippets
   curl -I -X OPTIONS -H 'Host: cors-api.local' -H 'Origin: https://example.com' http://localhost:8080/api/v1
   ```

## Files Overview

- `snippet-test-compose.yml` - Complete docker-compose environment
- `test-snippets.sh` - Automated test script
- `snippets/` - Configuration snippet files
  - `location.conf` - Location-level nginx config
  - `server.conf` - Server-level nginx config  
  - `cors-location.conf` - CORS-specific location config
  - `cors-server.conf` - CORS-specific server config
- `api-content/` - Test API content

## What Gets Tested

1. **Advanced API** - Uses both location and server snippets
   - Custom headers: `X-API-Version`, `X-Custom-Feature`
   - Rate limiting and caching configuration
   - Custom error pages and logging

2. **Simple API** - No snippets for comparison
   - Basic nginx ingress functionality
   - Verifies snippets don't affect other services

3. **CORS API** - CORS-specific snippets
   - Enhanced CORS handling
   - Custom error responses with CORS headers
   - Preflight request optimization

## Expected Results

### Advanced API Headers
```
X-API-Version: v2.0
X-Custom-Feature: snippets-enabled
```

### CORS API Headers
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
X-API-Version: v1.0
X-CORS-Enabled: snippets
```

## Architecture

The snippet system:
1. **Downloads** config files from containers
2. **Validates** file paths and nginx syntax
3. **Caches** files for performance
4. **Includes** content in generated nginx config

## Cleanup

```bash
docker-compose -f snippet-test-compose.yml down
```