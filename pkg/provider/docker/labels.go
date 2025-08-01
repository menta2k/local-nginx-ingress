package docker

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// Base label prefix for nginx ingress
	LabelPrefix = "nginx.ingress"
	
	// Core labels
	LabelEnable    = LabelPrefix + ".enable"
	LabelHost      = LabelPrefix + ".host"
	LabelPort      = LabelPrefix + ".port"
	LabelPath      = LabelPrefix + ".path"
	LabelProtocol  = LabelPrefix + ".protocol"
	
	// SSL/TLS labels
	LabelTLS       = LabelPrefix + ".tls"
	LabelCertName  = LabelPrefix + ".tls.certname"
	
	// Advanced routing labels
	LabelPriority  = LabelPrefix + ".priority"
	LabelRule      = LabelPrefix + ".rule"
	
	// Load balancing labels
	LabelLoadBalancer = LabelPrefix + ".loadbalancer"
	LabelMethod       = LabelPrefix + ".loadbalancer.method"
	
	// Health check labels
	LabelHealthCheck     = LabelPrefix + ".healthcheck"
	LabelHealthCheckPath = LabelPrefix + ".healthcheck.path"
	
	// Middleware labels
	LabelMiddleware = LabelPrefix + ".middleware"
	LabelAuth       = LabelPrefix + ".auth"
	LabelCORS       = LabelPrefix + ".cors"
	
	// Snippet labels (file-based configuration)
	LabelConfigurationSnippet = LabelPrefix + ".configuration-snippet"
	LabelServerSnippet        = LabelPrefix + ".server-snippet"
	
	// FastCGI labels
	LabelBackendProtocol    = LabelPrefix + ".backend-protocol"
	LabelFastCGIIndex       = LabelPrefix + ".fastcgi-index"
	LabelFastCGIParams      = LabelPrefix + ".fastcgi-params"
	LabelFastCGIParamsFile  = LabelPrefix + ".fastcgi-params-file"
	
	// Default values
	DefaultProtocol = "http"
	DefaultPort     = "80"
	DefaultPath     = "/"
	DefaultPriority = 100
)

// ContainerConfig represents the nginx configuration extracted from container labels
type ContainerConfig struct {
	ContainerID   string
	ContainerName string
	NetworkIP     string
	
	// Basic routing
	Enabled   bool
	Host      string
	Port      int
	Path      string
	Protocol  string
	Priority  int
	Rule      string
	
	// SSL/TLS
	TLS      bool
	CertName string
	
	// Load balancing
	LoadBalancer LoadBalancerConfig
	
	// Health check
	HealthCheck HealthCheckConfig
	
	// Middleware
	Middleware MiddlewareConfig
	
	// Nginx snippets (file-based)
	ConfigurationSnippet string // Path to location-level nginx config file
	ServerSnippet        string // Path to server-level nginx config file
	
	// FastCGI configuration
	FastCGI FastCGIConfig
}

type LoadBalancerConfig struct {
	Method string // round_robin, least_conn, ip_hash
}

type HealthCheckConfig struct {
	Enabled bool
	Path    string
}

type MiddlewareConfig struct {
	Auth AuthConfig
	CORS CORSConfig
}

type AuthConfig struct {
	Enabled  bool
	Type     string // basic, digest
	Realm    string
	Users    []string
}

type CORSConfig struct {
	Enabled          bool
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
}

type FastCGIConfig struct {
	Enabled       bool
	BackendProtocol string // "FCGI" to enable FastCGI mode
	Index         string   // FastCGI index file (e.g., "index.php")
	Params        map[string]string // FastCGI parameters
	ParamsFile    string   // Path to file containing FastCGI parameters
}

// ExtractConfig extracts nginx configuration from container labels
func ExtractConfig(containerID, containerName, networkIP string, labels map[string]string) (*ContainerConfig, error) {
	config := &ContainerConfig{
		ContainerID:   containerID,
		ContainerName: containerName,
		NetworkIP:     networkIP,
		Protocol:      DefaultProtocol,
		Port:          80,
		Path:          DefaultPath,
		Priority:      DefaultPriority,
	}
	
	// Check if nginx ingress is enabled
	if enabled, exists := labels[LabelEnable]; !exists || !parseBool(enabled) {
		config.Enabled = false
		return config, nil
	}
	config.Enabled = true
	
	// Extract host
	if host, exists := labels[LabelHost]; exists {
		config.Host = host
	} else {
		return nil, fmt.Errorf("container %s: %s label is required when nginx ingress is enabled", containerName, LabelHost)
	}
	
	// Extract port
	if portStr, exists := labels[LabelPort]; exists {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 && port <= 65535 {
			config.Port = port
		} else {
			return nil, fmt.Errorf("container %s: invalid port %s", containerName, portStr)
		}
	}
	
	// Extract other basic config
	if protocol, exists := labels[LabelProtocol]; exists {
		if protocol == "http" || protocol == "https" {
			config.Protocol = protocol
		} else {
			return nil, fmt.Errorf("container %s: invalid protocol %s, must be http or https", containerName, protocol)
		}
	}
	
	if path, exists := labels[LabelPath]; exists {
		config.Path = path
	}
	
	if priorityStr, exists := labels[LabelPriority]; exists {
		if priority, err := strconv.Atoi(priorityStr); err == nil {
			config.Priority = priority
		}
	}
	
	if rule, exists := labels[LabelRule]; exists {
		config.Rule = rule
	}
	
	// Extract TLS config
	config.TLS = parseBool(labels[LabelTLS])
	if certName, exists := labels[LabelCertName]; exists {
		config.CertName = certName
	}
	
	// Extract load balancer config
	config.LoadBalancer = extractLoadBalancerConfig(labels)
	
	// Extract health check config
	config.HealthCheck = extractHealthCheckConfig(labels)
	
	// Extract middleware config
	config.Middleware = extractMiddlewareConfig(labels)
	
	// Extract snippet file paths
	if configSnippet, exists := labels[LabelConfigurationSnippet]; exists {
		config.ConfigurationSnippet = configSnippet
	}
	
	if serverSnippet, exists := labels[LabelServerSnippet]; exists {
		config.ServerSnippet = serverSnippet
	}
	
	// Extract FastCGI config
	config.FastCGI = extractFastCGIConfig(labels)
	
	return config, nil
}

func extractLoadBalancerConfig(labels map[string]string) LoadBalancerConfig {
	config := LoadBalancerConfig{
		Method: "round_robin", // default
	}
	
	if method, exists := labels[LabelMethod]; exists {
		switch method {
		case "round_robin", "least_conn", "ip_hash":
			config.Method = method
		}
	}
	
	return config
}

func extractHealthCheckConfig(labels map[string]string) HealthCheckConfig {
	config := HealthCheckConfig{
		Enabled: parseBool(labels[LabelHealthCheck]),
		Path:    "/health",
	}
	
	if path, exists := labels[LabelHealthCheckPath]; exists {
		config.Path = path
	}
	
	return config
}

func extractMiddlewareConfig(labels map[string]string) MiddlewareConfig {
	config := MiddlewareConfig{}
	
	// Extract auth config
	if authType, exists := labels[LabelAuth]; exists {
		config.Auth.Enabled = true
		config.Auth.Type = authType
		// Parse additional auth labels if needed
	}
	
	// Extract CORS config
	if corsEnabled := parseBool(labels[LabelCORS]); corsEnabled {
		config.CORS.Enabled = true
		// Parse CORS specific labels
		if origins, exists := labels[LabelCORS+".origins"]; exists {
			config.CORS.AllowOrigins = strings.Split(origins, ",")
		}
		if methods, exists := labels[LabelCORS+".methods"]; exists {
			config.CORS.AllowMethods = strings.Split(methods, ",")
		}
	}
	
	return config
}

func extractFastCGIConfig(labels map[string]string) FastCGIConfig {
	config := FastCGIConfig{}
	
	// Check if backend protocol is FCGI
	if backendProtocol, exists := labels[LabelBackendProtocol]; exists {
		config.BackendProtocol = backendProtocol
		if strings.ToUpper(backendProtocol) == "FCGI" {
			config.Enabled = true
		}
	}
	
	// Extract FastCGI index
	if index, exists := labels[LabelFastCGIIndex]; exists {
		config.Index = index
	}
	
	// Extract FastCGI parameters from direct label
	if params, exists := labels[LabelFastCGIParams]; exists {
		config.Params = parseFastCGIParams(params)
	}
	
	// Extract FastCGI parameters file path
	if paramsFile, exists := labels[LabelFastCGIParamsFile]; exists {
		config.ParamsFile = paramsFile
	}
	
	return config
}

func parseFastCGIParams(paramStr string) map[string]string {
	params := make(map[string]string)
	if paramStr == "" {
		return params
	}
	
	// Parse comma-separated key=value pairs
	pairs := strings.Split(paramStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			params[key] = value
		}
	}
	
	return params
}

func parseBool(value string) bool {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// ValidateConfig validates the extracted configuration
func ValidateConfig(config *ContainerConfig) error {
	if !config.Enabled {
		return nil
	}
	
	if config.Host == "" {
		return fmt.Errorf("host is required when nginx ingress is enabled")
	}
	
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("invalid port %d", config.Port)
	}
	
	if config.Protocol != "http" && config.Protocol != "https" {
		return fmt.Errorf("invalid protocol %s", config.Protocol)
	}
	
	// Validate FastCGI configuration
	if config.FastCGI.Enabled {
		if config.FastCGI.BackendProtocol != "FCGI" {
			return fmt.Errorf("backend-protocol must be 'FCGI' when FastCGI is enabled")
		}
	}
	
	if !strings.HasPrefix(config.Path, "/") {
		return fmt.Errorf("path must start with '/'")
	}
	
	return nil
}