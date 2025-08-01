package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/menta2k/local-nginx-ingress/pkg/errors"
)

// HealthStatus represents the health status of a component
type HealthStatus int

const (
	Healthy HealthStatus = iota
	Degraded
	Unhealthy
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name           string
	Status         HealthStatus
	LastCheckTime  time.Time
	ErrorCount     int
	LastError      error
	CheckInterval  time.Duration
	HealthChecker  func() error
}

// HealthMonitor monitors the health of various system components
type HealthMonitor struct {
	components    map[string]*ComponentHealth
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	errorHandler  *errors.ErrorHandler
	healthServer  *http.Server
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	errorHandler := errors.NewErrorHandler()
	errorHandler.SetExitOnCritical(false)
	errorHandler.SetRetryConfig(2, 2*time.Second)
	
	hm := &HealthMonitor{
		components:   make(map[string]*ComponentHealth),
		ctx:          ctx,
		cancel:       cancel,
		errorHandler: errorHandler,
	}
	
	// Set up health check HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hm.healthHandler)
	mux.HandleFunc("/health/detailed", hm.detailedHealthHandler)
	
	hm.healthServer = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	
	return hm
}

// RegisterComponent registers a component for health monitoring
func (hm *HealthMonitor) RegisterComponent(name string, checker func() error, interval time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	component := &ComponentHealth{
		Name:          name,
		Status:        Healthy,
		CheckInterval: interval,
		HealthChecker: checker,
		LastCheckTime: time.Now(),
	}
	
	hm.components[name] = component
	
	// Start monitoring this component
	go hm.monitorComponent(component)
}

// Start starts the health monitor
func (hm *HealthMonitor) Start() error {
	defer errors.Recover("health-monitor")
	
	// Start health check HTTP server
	go func() {
		defer errors.Recover("health-server")
		
		if err := hm.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			hm.errorHandler.Error("Health server failed", err, "health")
		}
	}()
	
	hm.errorHandler.Info("Health monitor started", "health")
	return nil
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() error {
	defer errors.Recover("health-monitor")
	
	hm.cancel()
	
	// Stop health check server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := hm.healthServer.Shutdown(ctx); err != nil {
		hm.errorHandler.Warning("Health server shutdown error", err, "health")
	}
	
	hm.errorHandler.Info("Health monitor stopped", "health")
	return nil
}

// monitorComponent monitors a single component
func (hm *HealthMonitor) monitorComponent(component *ComponentHealth) {
	defer errors.Recover("health-monitor")
	
	ticker := time.NewTicker(component.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			hm.checkComponent(component)
		case <-hm.ctx.Done():
			return
		}
	}
}

// checkComponent performs a health check on a component
func (hm *HealthMonitor) checkComponent(component *ComponentHealth) {
	defer errors.Recover("health-check")
	
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	err := component.HealthChecker()
	component.LastCheckTime = time.Now()
	
	if err != nil {
		component.ErrorCount++
		component.LastError = err
		
		// Determine status based on error count
		if component.ErrorCount >= 5 {
			component.Status = Unhealthy
		} else if component.ErrorCount >= 2 {
			component.Status = Degraded
		}
		
		hm.errorHandler.Warning(fmt.Sprintf("Health check failed for %s", component.Name), err, "health")
	} else {
		// Reset on success
		if component.ErrorCount > 0 {
			hm.errorHandler.Info(fmt.Sprintf("Health check recovered for %s", component.Name), "health")
		}
		component.ErrorCount = 0
		component.LastError = nil
		component.Status = Healthy
	}
}

// GetComponentHealth returns the health status of a specific component
func (hm *HealthMonitor) GetComponentHealth(name string) (*ComponentHealth, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	component, exists := hm.components[name]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	copy := *component
	return &copy, true
}

// GetOverallHealth returns the overall system health
func (hm *HealthMonitor) GetOverallHealth() HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	overallStatus := Healthy
	
	for _, component := range hm.components {
		if component.Status == Unhealthy {
			return Unhealthy
		}
		if component.Status == Degraded {
			overallStatus = Degraded
		}
	}
	
	return overallStatus
}

// healthHandler handles basic health check requests
func (hm *HealthMonitor) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := hm.GetOverallHealth()
	
	switch status {
	case Healthy:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	case Degraded:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"degraded"}`))
	case Unhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy"}`))
	}
}

// detailedHealthHandler provides detailed health information
func (hm *HealthMonitor) detailedHealthHandler(w http.ResponseWriter, r *http.Request) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	response := `{"overall_status":"`
	switch hm.GetOverallHealth() {
	case Healthy:
		response += "healthy"
	case Degraded:
		response += "degraded"
	case Unhealthy:
		response += "unhealthy"
	}
	response += `","components":[`
	
	first := true
	for name, component := range hm.components {
		if !first {
			response += ","
		}
		first = false
		
		status := "healthy"
		switch component.Status {
		case Degraded:
			status = "degraded"
		case Unhealthy:
			status = "unhealthy"
		}
		
		response += fmt.Sprintf(`{"name":"%s","status":"%s","error_count":%d,"last_check":"%s"}`,
			name, status, component.ErrorCount, component.LastCheckTime.Format(time.RFC3339))
	}
	
	response += `]}`
	
	w.Write([]byte(response))
}

// IsHealthy returns true if the overall system is healthy
func (hm *HealthMonitor) IsHealthy() bool {
	return hm.GetOverallHealth() == Healthy
}

// IsDegraded returns true if the system is in degraded mode
func (hm *HealthMonitor) IsDegraded() bool {
	return hm.GetOverallHealth() == Degraded
}