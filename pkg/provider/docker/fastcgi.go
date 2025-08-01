package docker

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/client"
)

// FastCGIParameterManager handles FastCGI parameter file downloading and parsing
type FastCGIParameterManager struct {
	client    *client.Client
	cacheDir  string
	ctx       context.Context
	snippetManager *SnippetManager // Reuse snippet manager for file operations
}

// NewFastCGIParameterManager creates a new FastCGI parameter manager
func NewFastCGIParameterManager(client *client.Client, cacheDir string) *FastCGIParameterManager {
	return &FastCGIParameterManager{
		client:         client,
		cacheDir:       cacheDir,
		ctx:            context.Background(),
		snippetManager: NewSnippetManager(client, cacheDir),
	}
}

// LoadFastCGIParams loads FastCGI parameters from container file or labels
func (fpm *FastCGIParameterManager) LoadFastCGIParams(config *ContainerConfig) (map[string]string, error) {
	params := make(map[string]string)
	
	// First, add any parameters from direct labels
	if config.FastCGI.Params != nil {
		for key, value := range config.FastCGI.Params {
			params[key] = value
		}
	}
	
	// Then, load parameters from file if specified
	if config.FastCGI.ParamsFile != "" {
		fileParams, err := fpm.loadParamsFromFile(config.ContainerID, config.FastCGI.ParamsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load FastCGI params from file %s: %w", config.FastCGI.ParamsFile, err)
		}
		
		// Merge file parameters (file takes precedence over labels)
		for key, value := range fileParams {
			params[key] = value
		}
	}
	
	// Add default PHP-FPM parameters if not specified
	fpm.addDefaultPHPParams(params)
	
	return params, nil
}

// loadParamsFromFile downloads and parses FastCGI parameters from a container file
func (fpm *FastCGIParameterManager) loadParamsFromFile(containerID, filePath string) (map[string]string, error) {
	// Validate file path
	if err := fpm.snippetManager.validateFilePath(filePath); err != nil {
		return nil, err
	}
	
	// Download file using snippet manager's infrastructure
	snippetContent, err := fpm.snippetManager.DownloadSnippet(containerID, filePath)
	if err != nil {
		return nil, err
	}
	
	// Parse FastCGI parameters
	return fpm.parseFastCGIParamsFile(snippetContent.Content)
}

// parseFastCGIParamsFile parses FastCGI parameter file content
func (fpm *FastCGIParameterManager) parseFastCGIParamsFile(content string) (map[string]string, error) {
	params := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse fastcgi_param directives
		if strings.HasPrefix(line, "fastcgi_param ") {
			// Extract parameter name and value
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				paramName := parts[1]
				paramValue := strings.Join(parts[2:], " ")
				
				// Remove trailing semicolon if present
				paramValue = strings.TrimSuffix(paramValue, ";")
				
				// Remove quotes if present
				if (strings.HasPrefix(paramValue, "\"") && strings.HasSuffix(paramValue, "\"")) ||
				   (strings.HasPrefix(paramValue, "'") && strings.HasSuffix(paramValue, "'")) {
					paramValue = paramValue[1 : len(paramValue)-1]
				}
				
				params[paramName] = paramValue
			}
		} else {
			// Support simple key=value format as well
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Remove quotes if present
				if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				   (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
					value = value[1 : len(value)-1]
				}
				
				params[key] = value
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading FastCGI params file: %w", err)
	}
	
	return params, nil
}

// addDefaultPHPParams adds common PHP-FPM parameters if not already specified
func (fpm *FastCGIParameterManager) addDefaultPHPParams(params map[string]string) {
	defaults := map[string]string{
		"SCRIPT_FILENAME":   "$document_root$fastcgi_script_name",
		"QUERY_STRING":      "$query_string",
		"REQUEST_METHOD":    "$request_method",
		"CONTENT_TYPE":      "$content_type",
		"CONTENT_LENGTH":    "$content_length",
		"SCRIPT_NAME":       "$fastcgi_script_name",
		"REQUEST_URI":       "$request_uri",
		"DOCUMENT_URI":      "$document_uri",
		"DOCUMENT_ROOT":     "$document_root",
		"SERVER_PROTOCOL":   "$server_protocol",
		"REQUEST_SCHEME":    "$scheme",
		"HTTPS":            "$https if_not_empty",
		"GATEWAY_INTERFACE": "CGI/1.1",
		"SERVER_SOFTWARE":   "nginx/$nginx_version",
		"REMOTE_ADDR":       "$remote_addr",
		"REMOTE_PORT":       "$remote_port",
		"SERVER_ADDR":       "$server_addr",
		"SERVER_PORT":       "$server_port",
		"SERVER_NAME":       "$server_name",
		"REDIRECT_STATUS":   "200",
	}
	
	// Only add defaults that aren't already specified
	for key, value := range defaults {
		if _, exists := params[key]; !exists {
			params[key] = value
		}
	}
}

// ValidateFastCGIParams validates FastCGI parameters
func (fpm *FastCGIParameterManager) ValidateFastCGIParams(params map[string]string) error {
	// Check for required parameters
	required := []string{"SCRIPT_FILENAME", "REQUEST_METHOD"}
	for _, param := range required {
		if _, exists := params[param]; !exists {
			return fmt.Errorf("required FastCGI parameter %s is missing", param)
		}
	}
	
	// Validate SCRIPT_FILENAME contains proper variables (lenient check)
	if scriptFilename, exists := params["SCRIPT_FILENAME"]; exists {
		// Only warn if it doesn't contain common FastCGI variables (don't fail)
		if !strings.Contains(scriptFilename, "$fastcgi_script_name") && 
		   !strings.Contains(scriptFilename, "$document_root") {
			fmt.Printf("Warning: SCRIPT_FILENAME '%s' should typically contain $fastcgi_script_name or $document_root variables\n", scriptFilename)
		}
	}
	
	return nil
}

// GetSupportedFileExtensions returns supported file extensions for FastCGI params files
func (fpm *FastCGIParameterManager) GetSupportedFileExtensions() []string {
	return []string{".conf", ".txt", ".params"}
}

// GetDefaultFastCGIParams returns a complete set of default FastCGI parameters
func GetDefaultFastCGIParams() map[string]string {
	return map[string]string{
		"SCRIPT_FILENAME":   "$document_root$fastcgi_script_name",
		"QUERY_STRING":      "$query_string",
		"REQUEST_METHOD":    "$request_method",
		"CONTENT_TYPE":      "$content_type",
		"CONTENT_LENGTH":    "$content_length",
		"SCRIPT_NAME":       "$fastcgi_script_name",
		"REQUEST_URI":       "$request_uri",
		"DOCUMENT_URI":      "$document_uri",
		"DOCUMENT_ROOT":     "$document_root",
		"SERVER_PROTOCOL":   "$server_protocol",
		"REQUEST_SCHEME":    "$scheme",
		"HTTPS":            "$https if_not_empty",
		"GATEWAY_INTERFACE": "CGI/1.1",
		"SERVER_SOFTWARE":   "nginx/$nginx_version",
		"REMOTE_ADDR":       "$remote_addr",
		"REMOTE_PORT":       "$remote_port",
		"SERVER_ADDR":       "$server_addr",
		"SERVER_PORT":       "$server_port",
		"SERVER_NAME":       "$server_name",
		"REDIRECT_STATUS":   "200",
		"PATH_INFO":         "$fastcgi_path_info",
		"PATH_TRANSLATED":   "$document_root$fastcgi_path_info",
	}
}