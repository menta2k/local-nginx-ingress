# FastCGI Support in Nginx Ingress

This directory contains examples and documentation for using FastCGI backends with the nginx ingress controller.

## Overview

FastCGI support allows you to directly expose FastCGI servers (like PHP-FPM) without needing a separate web server. This is particularly useful for:

- PHP applications with PHP-FPM
- Python applications with FastCGI servers
- Other applications that support the FastCGI protocol

## Labels

### Required Labels

- `nginx.ingress.backend-protocol=FCGI` - Enables FastCGI mode
- `nginx.ingress.enable=true` - Enables nginx ingress
- `nginx.ingress.host=example.com` - Sets the hostname

### Optional Labels

- `nginx.ingress.fastcgi-index=index.php` - Sets the default index file
- `nginx.ingress.fastcgi-params=KEY=value,KEY2=value2` - Direct parameter specification
- `nginx.ingress.fastcgi-params-file=/app/config/fastcgi.conf` - Path to parameter file in container

## Configuration Methods

### 1. Label-based Parameters

```yaml
labels:
  - "nginx.ingress.enable=true"
  - "nginx.ingress.host=php-app.local"
  - "nginx.ingress.backend-protocol=FCGI"
  - "nginx.ingress.port=9000"
  - "nginx.ingress.fastcgi-index=index.php"
  - "nginx.ingress.fastcgi-params=SCRIPT_FILENAME=/var/www/html$fastcgi_script_name"
```

### 2. File-based Parameters

```yaml
labels:
  - "nginx.ingress.enable=true"
  - "nginx.ingress.host=php-app.local"
  - "nginx.ingress.backend-protocol=FCGI"
  - "nginx.ingress.port=9000"
  - "nginx.ingress.fastcgi-params-file=/app/config/fastcgi.conf"
```

## FastCGI Parameter File Format

The parameter file can use two formats:

### Nginx Format
```nginx
# FastCGI parameters for PHP-FPM
fastcgi_param SCRIPT_FILENAME /var/www/html$fastcgi_script_name;
fastcgi_param DOCUMENT_ROOT /var/www/html;
fastcgi_param PATH_INFO $fastcgi_path_info;
fastcgi_param PATH_TRANSLATED /var/www/html$fastcgi_path_info;
```

### Key-Value Format
```
SCRIPT_FILENAME=/var/www/html$fastcgi_script_name
DOCUMENT_ROOT=/var/www/html
PATH_INFO=$fastcgi_path_info
PATH_TRANSLATED=/var/www/html$fastcgi_path_info
```

## Generated Nginx Configuration

When FastCGI is enabled, the nginx ingress controller generates configuration like:

```nginx
location /app {
    # FastCGI configuration
    fastcgi_index index.php;
    
    # Default FastCGI parameters
    fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    fastcgi_param QUERY_STRING $query_string;
    fastcgi_param REQUEST_METHOD $request_method;
    # ... other default parameters
    
    # Custom FastCGI parameters
    fastcgi_param SCRIPT_FILENAME /var/www/html$fastcgi_script_name;
    fastcgi_param DOCUMENT_ROOT /var/www/html;
    
    # Pass to FastCGI backend
    fastcgi_pass 172.18.0.3:9000;
}
```

## Default Parameters

The controller automatically includes these standard FastCGI parameters:

- `SCRIPT_FILENAME` - Path to the script file
- `QUERY_STRING` - URL query parameters
- `REQUEST_METHOD` - HTTP method (GET, POST, etc.)
- `CONTENT_TYPE` - Request content type
- `CONTENT_LENGTH` - Request content length
- `REQUEST_URI` - Full request URI
- `DOCUMENT_ROOT` - Document root path
- `SERVER_NAME` - Server hostname
- `REMOTE_ADDR` - Client IP address
- `SERVER_PORT` - Server port
- And many more...

## Security Considerations

1. **File Path Validation**: Parameter files are validated for security
2. **Parameter Validation**: Required parameters are checked
3. **Container Isolation**: Files are downloaded from containers securely
4. **Default Restrictions**: Only safe file extensions are allowed

## Examples

See the example files in this directory:
- `php-app-compose.yml` - Complete PHP-FPM example
- `fastcgi-params.conf` - Example parameter file
- `test-fastcgi.sh` - Testing script