package docker

import (
	"fmt"
	"strings"
)

// Example label configurations for common use cases

// ExampleLabels provides example label configurations
var ExampleLabels = map[string]map[string]string{
	"simple_web_app": {
		"nginx.ingress.enable":   "true",
		"nginx.ingress.host":     "myapp.local",
		"nginx.ingress.port":     "3000",
		"nginx.ingress.path":     "/",
	},
	
	"api_with_auth": {
		"nginx.ingress.enable":   "true",
		"nginx.ingress.host":     "api.local",
		"nginx.ingress.port":     "8080",
		"nginx.ingress.path":     "/api",
		"nginx.ingress.auth":     "basic",
		"nginx.ingress.priority": "200",
	},
	
	"ssl_app": {
		"nginx.ingress.enable":     "true",
		"nginx.ingress.host":       "secure.local",
		"nginx.ingress.port":       "443",
		"nginx.ingress.protocol":   "https",
		"nginx.ingress.tls":        "true",
		"nginx.ingress.tls.certname": "secure.local",
	},
	
	"microservice_with_healthcheck": {
		"nginx.ingress.enable":           "true",
		"nginx.ingress.host":             "service.local",
		"nginx.ingress.port":             "8080",
		"nginx.ingress.path":             "/service",
		"nginx.ingress.healthcheck":      "true",
		"nginx.ingress.healthcheck.path": "/health",
		"nginx.ingress.loadbalancer.method": "least_conn",
	},
	
	"service_with_snippets": {
		"nginx.ingress.enable":                "true",
		"nginx.ingress.host":                  "advanced.local",
		"nginx.ingress.port":                  "8080",
		"nginx.ingress.path":                  "/api",
		"nginx.ingress.configuration-snippet": "/app/config/location.conf",
		"nginx.ingress.server-snippet":        "/app/config/server.conf",
	},
	
	"php_fastcgi_app": {
		"nginx.ingress.enable":            "true",
		"nginx.ingress.host":              "php-app.local",
		"nginx.ingress.backend-protocol":  "FCGI",
		"nginx.ingress.port":              "9000",
		"nginx.ingress.path":              "/app",
		"nginx.ingress.fastcgi-index":     "index.php",
		"nginx.ingress.fastcgi-params-file": "/app/config/fastcgi.conf",
	},
	
	"simple_php_fastcgi": {
		"nginx.ingress.enable":           "true",
		"nginx.ingress.host":             "simple-php.local",
		"nginx.ingress.backend-protocol": "FCGI",
		"nginx.ingress.port":             "9000",
		"nginx.ingress.fastcgi-index":    "index.php",
		"nginx.ingress.fastcgi-params":   "SCRIPT_FILENAME=/var/www/html$$fastcgi_script_name,DOCUMENT_ROOT=/var/www/html",
	},
	
	"cors_enabled_api": {
		"nginx.ingress.enable":        "true",
		"nginx.ingress.host":          "cors-api.local",
		"nginx.ingress.port":          "3000",
		"nginx.ingress.path":          "/api/v1",
		"nginx.ingress.cors":          "true",
		"nginx.ingress.cors.origins":  "https://app.local,https://admin.local",
		"nginx.ingress.cors.methods":  "GET,POST,PUT,DELETE",
	},
}

// GenerateDockerRunCommand generates a docker run command with nginx ingress labels
func GenerateDockerRunCommand(image, containerName string, labels map[string]string, port int) string {
	var cmd strings.Builder
	
	cmd.WriteString(fmt.Sprintf("docker run -d --name %s", containerName))
	
	// Add port mapping if specified
	if port > 0 {
		cmd.WriteString(fmt.Sprintf(" -p %d:%d", port, port))
	}
	
	// Add labels
	for key, value := range labels {
		cmd.WriteString(fmt.Sprintf(" --label \"%s=%s\"", key, value))
	}
	
	cmd.WriteString(fmt.Sprintf(" %s", image))
	
	return cmd.String()
}

// GenerateDockerComposeService generates a docker-compose service with nginx ingress labels
func GenerateDockerComposeService(serviceName, image string, labels map[string]string, port int) string {
	var yml strings.Builder
	
	yml.WriteString(fmt.Sprintf("%s:\n", serviceName))
	yml.WriteString(fmt.Sprintf("  image: %s\n", image))
	yml.WriteString("  labels:\n")
	
	for key, value := range labels {
		yml.WriteString(fmt.Sprintf("    - \"%s=%s\"\n", key, value))
	}
	
	if port > 0 {
		yml.WriteString("  ports:\n")
		yml.WriteString(fmt.Sprintf("    - \"%d:%d\"\n", port, port))
	}
	
	return yml.String()
}

// ValidateHostname validates a hostname for nginx ingress
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	
	if len(hostname) > 253 {
		return fmt.Errorf("hostname too long (max 253 characters)")
	}
	
	if strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") {
		return fmt.Errorf("hostname cannot start or end with a dot")
	}
	
	parts := strings.Split(hostname, ".")
	for _, part := range parts {
		if len(part) == 0 {
			return fmt.Errorf("hostname cannot have empty parts")
		}
		if len(part) > 63 {
			return fmt.Errorf("hostname part '%s' too long (max 63 characters)", part)
		}
	}
	
	return nil
}

// SanitizeContainerName sanitizes container name for use in nginx upstream names
func SanitizeContainerName(name string) string {
	// Replace invalid characters with underscores
	sanitized := strings.ReplaceAll(name, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	
	// Remove leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")
	
	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "unnamed"
	}
	
	return sanitized
}

// GetLabelDocumentation returns documentation for available labels
func GetLabelDocumentation() map[string]string {
	return map[string]string{
		LabelEnable:    "Enable nginx ingress for this container (true/false)",
		LabelHost:      "Hostname for this service (required when enabled)",
		LabelPort:      "Container port to proxy to (default: 80)",
		LabelPath:      "URL path prefix for this service (default: /)",
		LabelProtocol:  "Protocol to use: http or https (default: http)",
		LabelPriority:  "Priority for location matching (higher = first, default: 100)",
		LabelRule:      "Custom nginx location rule (advanced)",
		
		LabelTLS:       "Enable TLS/SSL (true/false)",
		LabelCertName:  "SSL certificate name (when TLS enabled)",
		
		LabelMethod:    "Load balancing method: round_robin, least_conn, ip_hash",
		
		LabelHealthCheck:     "Enable health checks (true/false)",
		LabelHealthCheckPath: "Health check endpoint path (default: /health)",
		
		LabelAuth:      "Authentication type: basic, digest",
		LabelCORS:      "Enable CORS (true/false)",
		LabelCORS + ".origins":  "Allowed CORS origins (comma-separated)",
		LabelCORS + ".methods":  "Allowed CORS methods (comma-separated)",
		
		LabelConfigurationSnippet: "Path to nginx location configuration file in container",
		LabelServerSnippet:        "Path to nginx server configuration file in container",
		
		LabelBackendProtocol:    "Backend protocol: http, https, or FCGI (for FastCGI)",
		LabelFastCGIIndex:       "FastCGI index file (e.g., index.php)",
		LabelFastCGIParams:      "FastCGI parameters as comma-separated key=value pairs",
		LabelFastCGIParamsFile:  "Path to FastCGI parameters file in container",
	}
}