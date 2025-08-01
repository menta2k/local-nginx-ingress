package docker

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/menta2k/local-nginx-ingress/pkg/errors"
)

// Provider represents the Docker provider for nginx ingress
type Provider struct {
	client        *client.Client
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Configuration
	nginxConfigPath string
	nginxBinary     string
	reloadCommand   []string
	templatePath    string
	
	// State management
	mu              sync.RWMutex
	containers      []*ContainerData
	lastConfig      *NginxConfig
	
	// Snippet management
	snippetManager  *SnippetManager
	
	// FastCGI parameter management
	fastcgiManager  *FastCGIParameterManager
	
	// Event handling
	eventChan       <-chan events.Message
	errorChan       <-chan error
	
	// Callbacks
	onConfigChange  func(*NginxConfig)
	onError         func(error)
	
	// Error handling
	errorHandler    *errors.ErrorHandler
}

// Config represents provider configuration
type Config struct {
	NginxConfigPath string
	NginxBinary     string
	ReloadCommand   []string
	SnippetCacheDir string
	TemplatePath    string // Path to nginx configuration template
	
	// Callbacks
	OnConfigChange func(*NginxConfig)
	OnError        func(error)
}

// NewProvider creates a new Docker provider
func NewProvider(dockerClient *client.Client, config Config) (*Provider, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set defaults
	if config.NginxConfigPath == "" {
		config.NginxConfigPath = "/etc/nginx/conf.d/docker-ingress.conf"
	}
	if config.NginxBinary == "" {
		config.NginxBinary = "nginx"
	}
	if len(config.ReloadCommand) == 0 {
		config.ReloadCommand = []string{"nginx", "-s", "reload"}
	}
	if config.SnippetCacheDir == "" {
		config.SnippetCacheDir = "/tmp/nginx-ingress-snippets"
	}
	if config.TemplatePath == "" {
		config.TemplatePath = "templates/nginx.conf.tmpl"
	}
	
	// Create error handler for provider operations
	errorHandler := errors.NewErrorHandler()
	errorHandler.SetExitOnCritical(false) // Allow graceful recovery
	errorHandler.SetRetryConfig(3, 5*time.Second)
	
	provider := &Provider{
		client:          dockerClient,
		ctx:             ctx,
		cancel:          cancel,
		nginxConfigPath: config.NginxConfigPath,
		nginxBinary:     config.NginxBinary,
		reloadCommand:   config.ReloadCommand,
		templatePath:    config.TemplatePath,
		onConfigChange:  config.OnConfigChange,
		onError:         config.OnError,
		snippetManager:  NewSnippetManager(dockerClient, config.SnippetCacheDir),
		fastcgiManager:  NewFastCGIParameterManager(dockerClient, config.SnippetCacheDir),
		errorHandler:    errorHandler,
	}
	
	return provider, nil
}

// Start starts the provider and begins monitoring Docker events
func (p *Provider) Start() error {
	defer errors.Recover("docker-provider")
	
	log.Println("Starting Docker nginx-ingress provider...")
	
	// Initial configuration load with retry
	if err := p.errorHandler.HandleWithRetry(func() error {
		return p.loadConfiguration()
	}, "provider", "loading initial configuration"); err != nil {
		p.errorHandler.Critical("Failed to load initial configuration after retries", err, "provider")
		return fmt.Errorf("failed to load initial configuration: %w", err)
	}
	
	// Start event monitoring with retry
	if err := p.errorHandler.HandleWithRetry(func() error {
		return p.startEventMonitoring()
	}, "provider", "starting event monitoring"); err != nil {
		p.errorHandler.Critical("Failed to start event monitoring after retries", err, "provider")
		return fmt.Errorf("failed to start event monitoring: %w", err)
	}
	
	// Start event processing loop
	go p.processEvents()
	
	log.Println("Docker nginx-ingress provider started successfully")
	p.errorHandler.Info("Docker provider started successfully", "provider")
	return nil
}

// Stop stops the provider
func (p *Provider) Stop() error {
	defer errors.Recover("docker-provider")
	
	log.Println("Stopping Docker nginx-ingress provider...")
	p.cancel()
	p.errorHandler.Info("Docker provider stopped successfully", "provider")
	return nil
}

// loadConfiguration loads the current container configuration
func (p *Provider) loadConfiguration() error {
	defer errors.Recover("docker-provider")
	
	containers, err := ListContainers(p.ctx, p.client)
	if err != nil {
		p.errorHandler.Error("Failed to list containers", err, "provider")
		return fmt.Errorf("failed to list containers: %w", err)
	}
	
	p.mu.Lock()
	p.containers = containers
	p.mu.Unlock()
	
	return p.updateNginxConfig()
}

// startEventMonitoring starts monitoring Docker events
func (p *Provider) startEventMonitoring() error {
	// Create event filters for container events
	eventFilters := filters.NewArgs()
	eventFilters.Add("type", "container")
	eventFilters.Add("event", "start")
	eventFilters.Add("event", "stop")
	eventFilters.Add("event", "die")
	eventFilters.Add("event", "destroy")
	
	// Start listening for events
	eventChan, errorChan := p.client.Events(p.ctx, events.ListOptions{
		Filters: eventFilters,
	})
	
	p.eventChan = eventChan
	p.errorChan = errorChan
	
	return nil
}

// processEvents processes Docker events
func (p *Provider) processEvents() {
	defer errors.Recover("docker-provider")
	
	log.Println("Starting Docker event processing...")
	
	for {
		select {
		case event := <-p.eventChan:
			if err := p.handleDockerEvent(event); err != nil {
				p.errorHandler.Warning("Error handling Docker event", err, "provider")
				if p.onError != nil {
					p.onError(err)
				}
			}
			
		case err := <-p.errorChan:
			if err != nil {
				p.errorHandler.Error("Docker event stream error", err, "provider")
				if p.onError != nil {
					p.onError(err)
				}
				
				// Try to restart event monitoring with retry
				time.Sleep(5 * time.Second)
				if err := p.errorHandler.HandleWithRetry(func() error {
					return p.startEventMonitoring()
				}, "provider", "restarting event monitoring"); err != nil {
					p.errorHandler.Critical("Failed to restart event monitoring after retries", err, "provider")
					return // Exit event processing loop on critical failure
				}
			}
			
		case <-p.ctx.Done():
			log.Println("Stopping Docker event processing...")
			p.errorHandler.Info("Docker event processing stopped", "provider")
			return
		}
	}
}

// handleDockerEvent handles a single Docker event
func (p *Provider) handleDockerEvent(event events.Message) error {
	defer errors.Recover("docker-provider")
	
	containerID := event.Actor.ID
	containerName := event.Actor.Attributes["name"]
	action := string(event.Action)
	
	log.Printf("Handling Docker event: %s for container %s (%s)", action, containerName, containerID[:12])
	
	// Check if container has nginx labels
	switch action {
	case "start":
		// Container started - check if it has nginx ingress labels
		containerJSON, err := p.client.ContainerInspect(p.ctx, containerID)
		if err != nil {
			if errdefs.IsNotFound(err) {
				p.errorHandler.Warning("Container not found during start event", err, "provider")
				return nil
			}
			inspectErr := fmt.Errorf("failed to inspect container %s: %w", containerID, err)
			p.errorHandler.Error("Failed to inspect container", inspectErr, "provider")
			return inspectErr
		}
		
		if hasNginxLabels(containerJSON.Config.Labels) {
			log.Printf("Container %s has nginx ingress labels, reloading configuration", containerName)
			return p.loadConfiguration()
		}
		
	case "stop", "die", "destroy":
		// Container stopped/removed - check if we need to update config
		p.mu.RLock()
		needsUpdate := false
		for _, container := range p.containers {
			if container.Config.ContainerID == containerID {
				needsUpdate = true
				break
			}
		}
		p.mu.RUnlock()
		
		if needsUpdate {
			log.Printf("Container %s with nginx ingress labels stopped, reloading configuration", containerName)
			return p.loadConfiguration()
		}
	}
	
	return nil
}

// updateNginxConfig generates and applies new nginx configuration
func (p *Provider) updateNginxConfig() error {
	defer errors.Recover("docker-provider")
	
	p.mu.RLock()
	containers := make([]*ContainerData, len(p.containers))
	copy(containers, p.containers)
	p.mu.RUnlock()
	
	// Filter only enabled containers
	enabledContainers := FilterEnabledContainers(containers)
	
	log.Printf("Generating nginx configuration for %d containers", len(enabledContainers))
	
	// Generate nginx configuration with snippet support
	config, err := GenerateNginxConfig(enabledContainers, p.snippetManager, p.fastcgiManager)
	if err != nil {
		generateErr := fmt.Errorf("failed to generate nginx config: %w", err)
		p.errorHandler.Error("Failed to generate nginx configuration", generateErr, "provider")
		return generateErr
	}
	
	// Validate configuration
	if err := ValidateNginxConfig(config); err != nil {
		validateErr := fmt.Errorf("invalid nginx config: %w", err)
		p.errorHandler.Error("Invalid nginx configuration generated", validateErr, "provider")
		return validateErr
	}
	
	// Check if configuration changed
	if p.configEquals(config, p.lastConfig) {
		log.Println("Configuration unchanged, skipping update")
		p.errorHandler.Info("Configuration unchanged, skipping update", "provider")
		return nil
	}
	
	// Write configuration to file with retry
	if err := p.errorHandler.HandleWithRetry(func() error {
		return p.writeConfigFile(config)
	}, "provider", "writing nginx configuration file"); err != nil {
		p.errorHandler.Error("Failed to write config file after retries", err, "provider")
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	// Test nginx configuration with retry
	if err := p.errorHandler.HandleWithRetry(func() error {
		return p.testNginxConfig()
	}, "provider", "testing nginx configuration"); err != nil {
		p.errorHandler.Error("Nginx configuration test failed after retries", err, "provider")
		return fmt.Errorf("nginx config test failed: %w", err)
	}
	
	// Reload nginx with retry
	if err := p.errorHandler.HandleWithRetry(func() error {
		return p.reloadNginx()
	}, "provider", "reloading nginx"); err != nil {
		p.errorHandler.Error("Failed to reload nginx after retries", err, "provider")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}
	
	p.mu.Lock()
	p.lastConfig = config
	p.mu.Unlock()
	
	log.Println("Nginx configuration updated successfully")
	p.errorHandler.Info("Nginx configuration updated successfully", "provider")
	
	// Notify callback
	if p.onConfigChange != nil {
		p.onConfigChange(config)
	}
	
	return nil
}

// writeConfigFile writes the nginx configuration to file
func (p *Provider) writeConfigFile(config *NginxConfig) error {
	content, err := RenderNginxConfig(config, p.templatePath)
	if err != nil {
		return err
	}
	
	// Ensure directory exists
	dir := filepath.Dir(p.nginxConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Write to temporary file first
	tempFile := p.nginxConfigPath + ".tmp"
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	
	// Atomic move
	if err := os.Rename(tempFile, p.nginxConfigPath); err != nil {
		os.Remove(tempFile) // cleanup
		return fmt.Errorf("failed to move config file: %w", err)
	}
	
	log.Printf("Nginx configuration written to %s", p.nginxConfigPath)
	return nil
}

// testNginxConfig tests the nginx configuration
func (p *Provider) testNginxConfig() error {
	cmd := exec.Command(p.nginxBinary, "-t")
	if output, err := cmd.CombinedOutput(); err != nil {
		testErr := fmt.Errorf("nginx config test failed: %s", string(output))
		p.errorHandler.Warning("Nginx configuration test failed", testErr, "provider")
		return testErr
	}
	return nil
}

// reloadNginx reloads the nginx configuration
func (p *Provider) reloadNginx() error {
	cmd := exec.Command(p.reloadCommand[0], p.reloadCommand[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		reloadErr := fmt.Errorf("nginx reload failed: %s", string(output))
		p.errorHandler.Warning("Nginx reload failed", reloadErr, "provider")
		return reloadErr
	}
	log.Println("Nginx reloaded successfully")
	p.errorHandler.Info("Nginx reloaded successfully", "provider")
	return nil
}

// configEquals compares two nginx configurations for equality
func (p *Provider) configEquals(a, b *NginxConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	// Simple comparison - in production you might want more sophisticated comparison
	aStr, _ := RenderNginxConfig(a, p.templatePath)
	bStr, _ := RenderNginxConfig(b, p.templatePath)
	
	return aStr == bStr
}

// GetContainers returns current containers with nginx ingress configuration
func (p *Provider) GetContainers() []*ContainerData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	containers := make([]*ContainerData, len(p.containers))
	copy(containers, p.containers)
	return containers
}

// GetCurrentConfig returns the current nginx configuration
func (p *Provider) GetCurrentConfig() *NginxConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastConfig
}