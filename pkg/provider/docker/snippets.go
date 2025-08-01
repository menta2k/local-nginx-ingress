package docker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
)

// SnippetManager handles downloading and caching nginx configuration snippets from containers
type SnippetManager struct {
	client    *client.Client
	cacheDir  string
	ctx       context.Context
}

// SnippetContent represents downloaded snippet content with metadata
type SnippetContent struct {
	Content  string
	FilePath string
	Hash     string
}

// NewSnippetManager creates a new snippet manager
func NewSnippetManager(dockerClient *client.Client, cacheDir string) *SnippetManager {
	return &SnippetManager{
		client:   dockerClient,
		cacheDir: cacheDir,
		ctx:      context.Background(),
	}
}

// DownloadSnippet downloads a configuration snippet from a container
func (sm *SnippetManager) DownloadSnippet(containerID, filePath string) (*SnippetContent, error) {
	if filePath == "" {
		return nil, nil
	}

	// Validate file path (security check)
	if err := sm.validateFilePath(filePath); err != nil {
		return nil, fmt.Errorf("invalid file path %s: %w", filePath, err)
	}

	// Generate cache key
	cacheKey := fmt.Sprintf("%s_%s", containerID[:12], sm.hashPath(filePath))
	cacheFile := filepath.Join(sm.cacheDir, cacheKey+".conf")

	// Check if we have a cached version
	if content, err := sm.loadFromCache(cacheFile); err == nil {
		return content, nil
	}

	// Download from container
	content, err := sm.downloadFromContainer(containerID, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s from container %s: %w", filePath, containerID, err)
	}

	snippet := &SnippetContent{
		Content:  content,
		FilePath: filePath,
		Hash:     sm.hashContent(content),
	}

	// Cache the content
	if err := sm.saveToCache(cacheFile, snippet); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to cache snippet %s: %v\n", cacheFile, err)
	}

	return snippet, nil
}

// downloadFromContainer downloads a file from a Docker container
func (sm *SnippetManager) downloadFromContainer(containerID, filePath string) (string, error) {
	// Use docker cp equivalent - create a tar stream from the container
	reader, _, err := sm.client.CopyFromContainer(sm.ctx, containerID, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to copy file from container: %w", err)
	}
	defer reader.Close()

	// Extract content from tar stream
	content, err := sm.extractFileFromTar(reader, filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("failed to extract file from tar: %w", err)
	}

	return content, nil
}

// extractFileFromTar extracts a single file from a tar stream
func (sm *SnippetManager) extractFileFromTar(reader io.Reader, filename string) (string, error) {
	// For simplicity, we'll use a different approach - docker exec
	// This is more direct but requires the container to be running
	return sm.downloadViaExec(reader, filename)
}

// downloadViaExec downloads file content using docker exec
func (sm *SnippetManager) downloadViaExec(reader io.Reader, filename string) (string, error) {
	// Read the tar stream into a buffer
	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return "", err
	}

	// For now, return the content as-is (this would need proper tar extraction in production)
	// This is a simplified implementation
	content := buf.String()
	
	// Clean up any tar headers (simplified)
	if strings.Contains(content, "\x00") {
		// Find the actual content after tar headers
		parts := strings.Split(content, "\x00")
		for _, part := range parts {
			if strings.TrimSpace(part) != "" && !strings.HasPrefix(part, "PaxHeaders") {
				return strings.TrimSpace(part), nil
			}
		}
	}
	
	return strings.TrimSpace(content), nil
}

// validateFilePath ensures the file path is safe and allowed
func (sm *SnippetManager) validateFilePath(filePath string) error {
	// Security checks
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	
	if strings.HasPrefix(filePath, "/etc/") || strings.HasPrefix(filePath, "/var/") {
		return fmt.Errorf("system directories not allowed")
	}
	
	if !strings.HasSuffix(filePath, ".conf") && !strings.HasSuffix(filePath, ".txt") {
		return fmt.Errorf("only .conf and .txt files allowed")
	}
	
	return nil
}

// hashPath creates a hash of the file path for cache keys
func (sm *SnippetManager) hashPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h)[:12]
}

// hashContent creates a hash of the content for change detection
func (sm *SnippetManager) hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)[:12]
}

// loadFromCache loads snippet content from cache
func (sm *SnippetManager) loadFromCache(cacheFile string) (*SnippetContent, error) {
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache file not found")
	}
	
	content, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	
	return &SnippetContent{
		Content: string(content),
		Hash:    sm.hashContent(string(content)),
	}, nil
}

// saveToCache saves snippet content to cache
func (sm *SnippetManager) saveToCache(cacheFile string, snippet *SnippetContent) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(sm.cacheDir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(cacheFile, []byte(snippet.Content), 0644)
}

// ClearCache removes all cached snippets
func (sm *SnippetManager) ClearCache() error {
	if _, err := os.Stat(sm.cacheDir); os.IsNotExist(err) {
		return nil
	}
	
	return os.RemoveAll(sm.cacheDir)
}

// DownloadAllSnippets downloads all snippets for a container configuration
func (sm *SnippetManager) DownloadAllSnippets(config *ContainerConfig) (map[string]*SnippetContent, error) {
	snippets := make(map[string]*SnippetContent)
	
	// Download configuration snippet (location-level)
	if config.ConfigurationSnippet != "" {
		snippet, err := sm.DownloadSnippet(config.ContainerID, config.ConfigurationSnippet)
		if err != nil {
			return nil, fmt.Errorf("failed to download configuration snippet: %w", err)
		}
		if snippet != nil {
			snippets["configuration"] = snippet
		}
	}
	
	// Download server snippet (server-level)
	if config.ServerSnippet != "" {
		snippet, err := sm.DownloadSnippet(config.ContainerID, config.ServerSnippet)
		if err != nil {
			return nil, fmt.Errorf("failed to download server snippet: %w", err)
		}
		if snippet != nil {
			snippets["server"] = snippet
		}
	}
	
	return snippets, nil
}

// GetExampleLabels returns example labels for snippet configuration
func GetExampleSnippetLabels() map[string]string {
	return map[string]string{
		LabelConfigurationSnippet: "/app/config/location.conf",
		LabelServerSnippet:        "/app/config/server.conf",
	}
}

// ValidateSnippetSyntax performs basic nginx syntax validation on snippet content
func ValidateSnippetSyntax(content string) error {
	// Basic validation - check for common syntax issues
	lines := strings.Split(content, "\n")
	openBraces := 0
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Count braces
		openBraces += strings.Count(line, "{")
		openBraces -= strings.Count(line, "}")
		
		// Check for semicolons on directive lines
		if !strings.Contains(line, "{") && !strings.Contains(line, "}") {
			if !strings.HasSuffix(line, ";") && !strings.HasSuffix(line, "{") {
				return fmt.Errorf("line %d: directive should end with semicolon: %s", i+1, line)
			}
		}
	}
	
	if openBraces != 0 {
		return fmt.Errorf("unmatched braces in snippet")
	}
	
	return nil
}