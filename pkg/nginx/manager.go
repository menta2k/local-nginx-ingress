package nginx

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/menta2k/local-nginx-ingress/pkg/errors"
)

// Manager manages the nginx process lifecycle
type Manager struct {
	binaryPath   string
	configPath   string
	pidFilePath  string
	cmd          *exec.Cmd
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
	errorHandler *errors.ErrorHandler
}

// Config represents nginx manager configuration
type Config struct {
	BinaryPath  string // Path to nginx binary
	ConfigPath  string // Path to main nginx.conf
	PidFilePath string // Path to nginx.pid file
}

// NewManager creates a new nginx manager
func NewManager(config Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set defaults
	if config.BinaryPath == "" {
		config.BinaryPath = "nginx"
	}
	if config.ConfigPath == "" {
		config.ConfigPath = "/etc/nginx/nginx.conf"
	}
	if config.PidFilePath == "" {
		config.PidFilePath = "/var/run/nginx.pid"
	}
	
	// Create error handler for nginx operations
	errorHandler := errors.NewErrorHandler()
	errorHandler.SetExitOnCritical(false) // Allow graceful recovery
	errorHandler.SetRetryConfig(2, 3*time.Second)
	
	return &Manager{
		binaryPath:   config.BinaryPath,
		configPath:   config.ConfigPath,
		pidFilePath:  config.PidFilePath,
		ctx:          ctx,
		cancel:       cancel,
		stopChan:     make(chan struct{}, 1),
		errorHandler: errorHandler,
	}
}

// Start starts the nginx process
func (m *Manager) Start() error {
	defer errors.Recover("nginx-manager")
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		err := fmt.Errorf("nginx is already running")
		m.errorHandler.Warning("Attempted to start nginx when already running", err, "nginx")
		return err
	}
	
	// Test configuration first with retry
	if err := m.errorHandler.HandleWithRetry(func() error {
		return m.testConfig()
	}, "nginx", "testing nginx configuration"); err != nil {
		m.errorHandler.Error("Nginx configuration test failed after retries", err, "nginx")
		return fmt.Errorf("nginx configuration test failed: %w", err)
	}
	
	// Create nginx command
	m.cmd = exec.CommandContext(m.ctx, m.binaryPath, "-g", "daemon off;")
	
	// Set up process group to handle child processes
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	// Forward stdout/stderr to our logs
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr
	
	log.Printf("Starting nginx process: %s -g 'daemon off;'", m.binaryPath)
	
	// Start process with retry
	if err := m.errorHandler.HandleWithRetry(func() error {
		return m.cmd.Start()
	}, "nginx", "starting nginx process"); err != nil {
		m.errorHandler.Critical("Failed to start nginx after retries", err, "nginx")
		return fmt.Errorf("failed to start nginx: %w", err)
	}
	
	m.running = true
	
	// Monitor the process in a goroutine
	go m.monitor()
	
	log.Printf("✅ Nginx started successfully with PID %d", m.cmd.Process.Pid)
	return nil
}

// Stop stops the nginx process gracefully
func (m *Manager) Stop() error {
	defer errors.Recover("nginx-manager")
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		m.errorHandler.Info("Nginx stop requested but already stopped", "nginx")
		return nil
	}
	
	log.Println("Stopping nginx process...")
	
	// Cancel context to stop the process
	m.cancel()
	
	// Try graceful shutdown first
	if m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(syscall.SIGQUIT); err != nil {
			m.errorHandler.Warning("Failed to send SIGQUIT to nginx", err, "nginx")
			
			// Force kill if graceful shutdown fails
			if err := m.cmd.Process.Kill(); err != nil {
				m.errorHandler.Error("Failed to kill nginx process", err, "nginx")
				return err
			}
		}
		
		// Wait for process to exit with timeout
		done := make(chan error, 1)
		go func() {
			defer errors.Recover("nginx-stop-wait")
			done <- m.cmd.Wait()
		}()
		
		select {
		case err := <-done:
			if err != nil {
				m.errorHandler.Warning("Nginx process exited with error", err, "nginx")
			} else {
				log.Println("✅ Nginx stopped gracefully")
			}
		case <-time.After(10 * time.Second):
			m.errorHandler.Warning("Timeout waiting for nginx to stop, force killing", nil, "nginx")
			if err := m.cmd.Process.Kill(); err != nil {
				m.errorHandler.Error("Failed to force kill nginx", err, "nginx")
				return err
			}
		}
	}
	
	m.running = false
	m.cmd = nil
	
	// Signal stop channel
	select {
	case m.stopChan <- struct{}{}:
	default:
	}
	
	return nil
}

// Reload reloads the nginx configuration
func (m *Manager) Reload() error {
	defer errors.Recover("nginx-manager")
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.running {
		err := fmt.Errorf("nginx is not running")
		m.errorHandler.Warning("Attempted to reload nginx when not running", err, "nginx")
		return err
	}
	
	// Test configuration first with retry
	if err := m.errorHandler.HandleWithRetry(func() error {
		return m.testConfig()
	}, "nginx", "testing configuration before reload"); err != nil {
		m.errorHandler.Error("Nginx configuration test failed before reload", err, "nginx")
		return fmt.Errorf("nginx configuration test failed: %w", err)
	}
	
	log.Println("Reloading nginx configuration...")
	
	if m.cmd.Process != nil {
		if err := m.errorHandler.HandleWithRetry(func() error {
			return m.cmd.Process.Signal(syscall.SIGHUP)
		}, "nginx", "sending SIGHUP signal for reload"); err != nil {
			m.errorHandler.Error("Failed to send SIGHUP to nginx after retries", err, "nginx")
			return fmt.Errorf("failed to send SIGHUP to nginx: %w", err)
		}
		log.Println("✅ Nginx configuration reloaded")
	}
	
	return nil
}

// IsRunning returns true if nginx is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// testConfig tests the nginx configuration
func (m *Manager) testConfig() error {
	cmd := exec.Command(m.binaryPath, "-t")
	if output, err := cmd.CombinedOutput(); err != nil {
		configErr := fmt.Errorf("configuration test failed: %s", string(output))
		m.errorHandler.Warning("Nginx configuration test failed", configErr, "nginx")
		return configErr
	}
	return nil
}

// monitor monitors the nginx process
func (m *Manager) monitor() {
	defer errors.Recover("nginx-monitor")
	
	if err := m.cmd.Wait(); err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		
		// Only log error if it's not due to context cancellation
		if m.ctx.Err() == nil {
			m.errorHandler.Critical("Nginx process died unexpectedly", err, "nginx")
		} else {
			m.errorHandler.Info("Nginx process stopped due to context cancellation", "nginx")
		}
	}
}

// CreateDefaultDirectories creates necessary directories for nginx
func CreateDefaultDirectories() error {
	defer errors.Recover("nginx-directories")
	
	dirs := []string{
		"/var/log/nginx",
		"/var/cache/nginx",
		"/var/run",
		"/etc/nginx/ssl",
		"/etc/nginx/auth",
		"/etc/nginx/conf.d",
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			dirErr := fmt.Errorf("failed to create directory %s: %w", dir, err)
			errors.ErrorMsg("Failed to create nginx directory", dirErr, "nginx")
			return dirErr
		}
	}
	
	errorHandlerInstance := errors.NewErrorHandler()
	errorHandlerInstance.Info("All nginx directories created successfully", "nginx")
	return nil
}

// GenerateDefaultSSLCert generates a default self-signed SSL certificate
func GenerateDefaultSSLCert() error {
	defer errors.Recover("nginx-ssl")
	
	certPath := "/etc/nginx/ssl/default.crt"
	keyPath := "/etc/nginx/ssl/default.key"
	errorHandlerInstance := errors.NewErrorHandler()
	
	// Skip if certificate already exists
	if _, err := os.Stat(certPath); err == nil {
		errorHandlerInstance.Info("SSL certificate already exists, skipping generation", "nginx")
		return nil
	}
	
	log.Println("📋 Generating default SSL certificate...")
	
	cmd := exec.Command("openssl", "req", "-x509", "-nodes", "-newkey", "rsa:2048", "-days", "365",
		"-keyout", keyPath,
		"-out", certPath,
		"-subj", "/C=US/ST=State/L=City/O=Organization/CN=localhost")
	
	if err := cmd.Run(); err != nil {
		certErr := fmt.Errorf("failed to generate SSL certificate: %w", err)
		errorHandlerInstance.Error("Failed to generate SSL certificate", certErr, "nginx")
		return certErr
	}
	
	// Set proper permissions
	if err := os.Chmod(keyPath, 0600); err != nil {
		permErr := fmt.Errorf("failed to set key file permissions: %w", err)
		errorHandlerInstance.Warning("Failed to set SSL key permissions", permErr, "nginx")
		return permErr
	}
	if err := os.Chmod(certPath, 0644); err != nil {
		permErr := fmt.Errorf("failed to set cert file permissions: %w", err)
		errorHandlerInstance.Warning("Failed to set SSL cert permissions", permErr, "nginx")
		return permErr
	}
	
	log.Println("✅ Default SSL certificate generated")
	errorHandlerInstance.Info("SSL certificate generated successfully", "nginx")
	return nil
}

// GetPid gets the nginx process PID
func (m *Manager) GetPid() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

// WaitForStop waits for the nginx process to stop
func (m *Manager) WaitForStop() {
	<-m.stopChan
}