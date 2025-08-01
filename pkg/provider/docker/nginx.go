package docker

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// NginxConfig represents the complete nginx configuration
type NginxConfig struct {
	Upstreams []UpstreamConfig
	Servers   []ServerConfig
	Generated time.Time
}

// UpstreamConfig represents an nginx upstream block
type UpstreamConfig struct {
	Name          string
	Method        string // load balancing method
	Servers       []UpstreamServer
	HealthCheck   bool
	HealthPath    string
}

// UpstreamServer represents a server in an upstream
type UpstreamServer struct {
	Address string
	Weight  int
	Backup  bool
}

// ServerConfig represents an nginx server block
type ServerConfig struct {
	ServerName string
	Listen     []string
	SSL        SSLConfig
	Locations  []LocationConfig
	
	// Custom server snippet (server-level)
	ServerSnippet string
}

// SSLConfig represents SSL/TLS configuration
type SSLConfig struct {
	Enabled     bool
	Certificate string
	PrivateKey  string
	Protocols   []string
}

// LocationConfig represents an nginx location block
type LocationConfig struct {
	Path      string
	Upstream  string
	Priority  int
	ProxyPass string
	
	// Middleware
	Auth     bool
	AuthType string
	CORS     CORSConfig
	
	// Headers and proxy settings
	ProxyHeaders map[string]string
	
	// Custom configuration snippet (location-level)
	ConfigurationSnippet string
	
	// FastCGI configuration
	FastCGI FastCGILocationConfig
}

// FastCGILocationConfig represents FastCGI-specific location configuration
type FastCGILocationConfig struct {
	Enabled    bool
	Pass       string // FastCGI backend address
	Index      string // FastCGI index file
	Params     map[string]string // FastCGI parameters
}

// loadTemplate loads a template file from the specified path
func loadTemplate(templatePath string) (string, error) {
	// Try absolute path first
	if filepath.IsAbs(templatePath) {
		if content, err := os.ReadFile(templatePath); err == nil {
			return string(content), nil
		}
	}
	
	// Try relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		fullPath := filepath.Join(execDir, templatePath)
		if content, err := os.ReadFile(fullPath); err == nil {
			return string(content), nil
		}
	}
	
	// Try current working directory
	if content, err := os.ReadFile(templatePath); err == nil {
		return string(content), nil
	}
	
	// Try common locations
	commonPaths := []string{
		"/app/templates/nginx.conf.tmpl",
		"/etc/nginx-ingress/templates/nginx.conf.tmpl",
		"templates/nginx.conf.tmpl",
		"../templates/nginx.conf.tmpl",
		"../../templates/nginx.conf.tmpl",
	}
	
	for _, path := range commonPaths {
		if content, err := os.ReadFile(path); err == nil {
			return string(content), nil
		}
	}
	
	return "", fmt.Errorf("template file not found: %s", templatePath)
}

// GenerateNginxConfig generates nginx configuration from container data
func GenerateNginxConfig(containers []*ContainerData, snippetManager *SnippetManager, fastcgiManager *FastCGIParameterManager) (*NginxConfig, error) {
	config := &NginxConfig{
		Generated: time.Now(),
	}
	
	// Group containers by host for server blocks
	hostGroups := GroupContainersByHost(containers)
	
	for host, hostContainers := range hostGroups {
		serverConfig := ServerConfig{
			ServerName: host,
			Listen:     []string{"80"},
		}
		
		// Check if any container requires SSL
		needsSSL := false
		for _, container := range hostContainers {
			if container.Config.TLS {
				needsSSL = true
				break
			}
		}
		
		if needsSSL {
			serverConfig.Listen = append(serverConfig.Listen, "443 ssl")
			serverConfig.SSL = SSLConfig{
				Enabled:     true,
				Certificate: "/etc/nginx/ssl/default.crt", // Use default cert for now
				PrivateKey:  "/etc/nginx/ssl/default.key", // Use default key for now
				Protocols:   []string{"TLSv1.2", "TLSv1.3"},
			}
		}
		
		// Download server snippet if needed
		var serverSnippetContent string
		for _, container := range hostContainers {
			if container.Config.ServerSnippet != "" {
				snippets, err := snippetManager.DownloadAllSnippets(container.Config)
				if err != nil {
					fmt.Printf("Warning: failed to download snippets for container %s: %v\n", container.Config.ContainerName, err)
				} else if serverSnippet, exists := snippets["server"]; exists {
					serverSnippetContent = serverSnippet.Content
				}
				break // Use first server snippet found for this host
			}
		}
		
		// Create upstream and locations for each container
		for _, container := range hostContainers {
			upstreamName := fmt.Sprintf("backend_%s_%s", 
				strings.ReplaceAll(host, ".", "_"), 
				container.Config.ContainerName)
			
			// Create upstream
			upstream := UpstreamConfig{
				Name:   upstreamName,
				Method: container.Config.LoadBalancer.Method,
				Servers: []UpstreamServer{
					{
						Address: fmt.Sprintf("%s:%d", container.IPAddress, container.Config.Port),
						Weight:  1,
					},
				},
				HealthCheck: container.Config.HealthCheck.Enabled,
				HealthPath:  container.Config.HealthCheck.Path,
			}
			config.Upstreams = append(config.Upstreams, upstream)
			
			// Download configuration snippet if needed
			var configSnippetContent string
			if container.Config.ConfigurationSnippet != "" {
				snippets, err := snippetManager.DownloadAllSnippets(container.Config)
				if err != nil {
					fmt.Printf("Warning: failed to download snippets for container %s: %v\n", container.Config.ContainerName, err)
				} else if configSnippet, exists := snippets["configuration"]; exists {
					configSnippetContent = configSnippet.Content
				}
			}
			
			// Create location
			location := LocationConfig{
				Path:      container.Config.Path,
				Upstream:  upstreamName,
				Priority:  container.Config.Priority,
				ProxyPass: fmt.Sprintf("http://%s", upstreamName),
				Auth:      container.Config.Middleware.Auth.Enabled,
				AuthType:  container.Config.Middleware.Auth.Type,
				CORS:      container.Config.Middleware.CORS,
				ProxyHeaders: map[string]string{
					"X-Container-Name": container.Config.ContainerName,
					"X-Container-ID":   container.Config.ContainerID[:12],
				},
				ConfigurationSnippet: configSnippetContent,
			}
			
			// Configure FastCGI if enabled
			if container.Config.FastCGI.Enabled {
				// Load FastCGI parameters (from file or labels)
				fastcgiParams, err := fastcgiManager.LoadFastCGIParams(container.Config)
				if err != nil {
					return nil, fmt.Errorf("failed to load FastCGI params for container %s: %w", container.Config.ContainerName, err)
				}
				
				// Validate FastCGI parameters
				if err := fastcgiManager.ValidateFastCGIParams(fastcgiParams); err != nil {
					return nil, fmt.Errorf("invalid FastCGI params for container %s: %w", container.Config.ContainerName, err)
				}
				
				location.FastCGI = FastCGILocationConfig{
					Enabled:    true,
					Pass:       fmt.Sprintf("%s:%d", container.IPAddress, container.Config.Port),
					Index:      container.Config.FastCGI.Index,
					Params:     fastcgiParams,
				}
				// For FastCGI, we don't use proxy_pass
				location.ProxyPass = ""
			}
			
			serverConfig.Locations = append(serverConfig.Locations, location)
		}
		
		// Add server snippet content
		serverConfig.ServerSnippet = serverSnippetContent
		
		config.Servers = append(config.Servers, serverConfig)
	}
	
	return config, nil
}

// RenderNginxConfig renders the nginx configuration to string using a template file
func RenderNginxConfig(config *NginxConfig, templatePath string) (string, error) {
	// Load template from file
	templateContent, err := loadTemplate(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to load template: %w", err)
	}
	
	funcMap := template.FuncMap{
		"join": strings.Join,
		"sortLocationsByPriority": sortLocationsByPriority,
	}
	
	tmpl, err := template.New("nginx").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse nginx template: %w", err)
	}
	
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute nginx template: %w", err)
	}
	
	return buf.String(), nil
}

// sortLocationsByPriority sorts locations by priority (higher priority first)
func sortLocationsByPriority(locations []LocationConfig) []LocationConfig {
	sorted := make([]LocationConfig, len(locations))
	copy(sorted, locations)
	
	sort.Slice(sorted, func(i, j int) bool {
		// Higher priority first, then by path specificity
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		
		// More specific paths (longer) should come first
		return len(sorted[i].Path) > len(sorted[j].Path)
	})
	
	return sorted
}

// WriteNginxConfig writes the nginx configuration to a file using a template
func WriteNginxConfig(config *NginxConfig, filename string, templatePath string) error {
	content, err := RenderNginxConfig(config, templatePath)
	if err != nil {
		return err
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	// Write configuration to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config to %s: %w", filename, err)
	}
	
	fmt.Printf("✅ Nginx configuration written to %s\n", filename)
	return nil
}

// ValidateNginxConfig performs basic validation on the nginx configuration
func ValidateNginxConfig(config *NginxConfig) error {
	// Check for duplicate upstream names
	upstreamNames := make(map[string]bool)
	for _, upstream := range config.Upstreams {
		if upstreamNames[upstream.Name] {
			return fmt.Errorf("duplicate upstream name: %s", upstream.Name)
		}
		upstreamNames[upstream.Name] = true
		
		if len(upstream.Servers) == 0 {
			return fmt.Errorf("upstream %s has no servers", upstream.Name)
		}
	}
	
	// Check for duplicate server names
	serverNames := make(map[string]bool)
	for _, server := range config.Servers {
		if serverNames[server.ServerName] {
			return fmt.Errorf("duplicate server name: %s", server.ServerName)
		}
		serverNames[server.ServerName] = true
		
		if len(server.Listen) == 0 {
			return fmt.Errorf("server %s has no listen directives", server.ServerName)
		}
	}
	
	return nil
}