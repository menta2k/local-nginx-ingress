package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/menta2k/local-nginx-ingress/pkg/errors"
	"github.com/menta2k/local-nginx-ingress/pkg/health"
	"github.com/menta2k/local-nginx-ingress/pkg/nginx"
	provider "github.com/menta2k/local-nginx-ingress/pkg/provider/docker"
	"github.com/menta2k/local-nginx-ingress/pkg/safe"
)

func main() {
	// Set up panic recovery
	defer errors.Recover("main")
	
	// Configure error handler for graceful degradation instead of immediate exit
	errorHandler := errors.NewErrorHandler()
	errorHandler.SetExitOnCritical(false) // Allow graceful recovery
	errorHandler.SetRetryConfig(3, 5*time.Second)
	
	ctx := context.Background()
	pool := safe.NewPool(ctx)

	log.Println("🐳 Starting Local Nginx Ingress Controller...")

	// Initialize health monitor
	healthMonitor := health.NewHealthMonitor()
	if err := healthMonitor.Start(); err != nil {
		errors.Warning("Failed to start health monitor", err, "health")
	}
	defer func() {
		if err := healthMonitor.Stop(); err != nil {
			errors.Warning("Error stopping health monitor", err, "health")
		}
	}()

	// Create necessary directories with retry
	if err := errorHandler.HandleWithRetry(func() error {
		return nginx.CreateDefaultDirectories()
	}, "startup", "creating necessary directories"); err != nil {
		errors.Critical("Failed to create directories after retries", err, "startup")
		return
	}

	// Generate default SSL certificate with retry
	if err := errorHandler.HandleWithRetry(func() error {
		return nginx.GenerateDefaultSSLCert()
	}, "startup", "generating SSL certificate"); err != nil {
		errors.Warning("Failed to generate SSL certificate, continuing without it", err, "startup")
		// Continue without SSL - not critical for basic functionality
	}

	// Create nginx manager
	nginxManager := nginx.NewManager(nginx.Config{
		BinaryPath:  getEnvOrDefault("NGINX_BINARY", "nginx"),
		ConfigPath:  "/etc/nginx/nginx.conf",
		PidFilePath: "/var/run/nginx.pid",
	})

	log.Println("🔍 Testing nginx configuration...")
	
	// Create Docker client with retry
	var cli *client.Client
	if err := errorHandler.HandleWithRetry(func() error {
		var err error
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
		return err
	}, "docker", "creating Docker client"); err != nil {
		errors.Critical("Failed to create Docker client after retries", err, "docker")
		return
	}
	defer func() {
		if cli != nil {
			if err := cli.Close(); err != nil {
				errors.Warning("Failed to close Docker client", err, "docker")
			}
		}
	}()

	// Test Docker connection with retry
	if err := errorHandler.HandleWithRetry(func() error {
		_, err := cli.Info(ctx)
		return err
	}, "docker", "testing Docker connection"); err != nil {
		errors.Critical("Failed to connect to Docker after retries", err, "docker")
		return
	}
	log.Printf("✅ Docker socket is accessible")

	// Register health checks
	healthMonitor.RegisterComponent("docker", func() error {
		_, err := cli.Info(ctx)
		return err
	}, 30*time.Second)

	healthMonitor.RegisterComponent("nginx", func() error {
		if !nginxManager.IsRunning() {
			return fmt.Errorf("nginx process is not running")
		}
		return nil
	}, 15*time.Second)

	// Create custom onConfigChange callback that uses nginx manager
	onConfigChangeWithReload := func(config *provider.NginxConfig) {
		log.Printf("📝 Nginx configuration updated with %d upstreams and %d servers",
			len(config.Upstreams), len(config.Servers))

		for _, server := range config.Servers {
			log.Printf("   • Server: %s (%d locations)", server.ServerName, len(server.Locations))
		}

		// Reload nginx using manager with error handling
		if nginxManager.IsRunning() {
			if err := errorHandler.HandleWithRetry(func() error {
				return nginxManager.Reload()
			}, "nginx", "reloading nginx configuration"); err != nil {
				errors.ErrorMsg("Failed to reload nginx configuration after retries", err, "nginx")
			}
		}
	}

	// Create provider configuration
	providerConfig := provider.Config{
		NginxConfigPath: getEnvOrDefault("NGINX_CONFIG_PATH", "/etc/nginx/conf.d/docker-ingress.conf"),
		NginxBinary:     getEnvOrDefault("NGINX_BINARY", "nginx"),
		ReloadCommand:   []string{"nginx", "-s", "reload"}, // Still used for config testing
		SnippetCacheDir: getEnvOrDefault("SNIPPET_CACHE_DIR", "/tmp/nginx-ingress-snippets"),
		OnConfigChange:  onConfigChangeWithReload,
		OnError:         onProviderError,
	}

	// Create Docker provider
	dockerProvider, err := provider.NewProvider(cli, providerConfig)
	if err != nil {
		errors.Critical("Failed to create Docker provider", err, "provider")
		return
	}

	log.Println("✅ Nginx configuration is valid")

	// Display configuration
	log.Println("📋 Configuration:")
	log.Printf("   • Nginx config: %s", providerConfig.NginxConfigPath)
	log.Printf("   • Nginx binary: %s", providerConfig.NginxBinary)
	log.Printf("   • Docker socket: %s", getEnvOrDefault("DOCKER_HOST", "unix:///var/run/docker.sock"))

	// Start nginx process with retry
	log.Println("🚀 Starting nginx process...")
	if err := errorHandler.HandleWithRetry(func() error {
		return nginxManager.Start()
	}, "nginx", "starting nginx process"); err != nil {
		errors.Critical("Failed to start nginx after retries", err, "nginx")
		return
	}

	// Start provider in a goroutine with error handling
	pool.GoCtx(func(ctx context.Context) {
		defer errors.Recover("provider")
		
		if err := dockerProvider.Start(); err != nil {
			errors.ErrorMsg("Docker provider encountered an error", err, "provider")
			// Try to restart the provider after a delay
			time.Sleep(10 * time.Second)
			log.Println("🔄 Attempting to restart Docker provider...")
			if restartErr := dockerProvider.Start(); restartErr != nil {
				errors.Critical("Failed to restart Docker provider", restartErr, "provider")
			}
		}
	})

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("✅ Local Nginx Ingress Controller started")
	log.Println("🔍 Monitoring Docker containers with labels starting with 'nginx.ingress'")
	log.Println("📡 Press Ctrl+C to stop")
	fmt.Println()

	// Display initial container status
	containers := dockerProvider.GetContainers()
	displayContainerStatus(containers)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println()
	log.Println("🛑 Shutting down gracefully...")

	// Stop nginx gracefully
	if err := nginxManager.Stop(); err != nil {
		errors.Warning("Error stopping nginx", err, "nginx")
	} else {
		log.Println("✅ Nginx stopped successfully")
	}

	// Stop provider gracefully
	if err := dockerProvider.Stop(); err != nil {
		errors.Warning("Error stopping Docker provider", err, "provider")
	} else {
		log.Println("✅ Docker provider stopped successfully")
	}

	// Stop goroutine pool
	pool.Stop()

	log.Println("👋 Local Nginx Ingress Controller stopped")
}


// onProviderError is called when provider encounters an error
func onProviderError(err error) {
	errors.ErrorMsg("Provider encountered an error", err, "provider")
}

// displayContainerStatus displays current container configurations
func displayContainerStatus(containers []*provider.ContainerData) {
	enabledCount := 0
	for _, container := range containers {
		if container.Config.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		log.Println("ℹ️  No containers with nginx ingress labels found")
		log.Println("   Add labels like 'nginx.ingress.enable=true' and 'nginx.ingress.host=example.com' to your containers")
		return
	}

	log.Printf("📊 Found %d containers with nginx ingress enabled:", enabledCount)
	for _, container := range containers {
		if container.Config.Enabled {
			log.Printf("   • %s -> %s:%d%s (%s)",
				container.Config.Host,
				container.IPAddress,
				container.Config.Port,
				container.Config.Path,
				container.Config.ContainerName)
		}
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}