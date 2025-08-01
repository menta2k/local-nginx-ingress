package errors

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// StructuredError represents a structured error with context  
type StructuredError struct {
	Message   string
	Cause     error
	Severity  ErrorSeverity
	Component string
	Context   map[string]interface{}
	Timestamp time.Time
	Stack     string
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// ErrorHandler manages error handling and recovery
type ErrorHandler struct {
	exitOnCritical    bool
	retryAttempts     int
	retryDelay       time.Duration
	circuitBreaker   *CircuitBreaker
	errorThreshold   int // Number of errors before triggering circuit breaker
	errorCount       int // Current error count
	lastResetTime    time.Time
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() *ErrorHandler {
	eh := &ErrorHandler{
		exitOnCritical:  true,
		retryAttempts:   3,
		retryDelay:     5 * time.Second,
		errorThreshold: 10, // Allow 10 errors before circuit breaking
		lastResetTime:  time.Now(),
	}
	eh.circuitBreaker = NewCircuitBreaker(3, 30*time.Second) // 3 failures, 30s timeout
	return eh
}

// SetExitOnCritical configures whether to exit on critical errors
func (eh *ErrorHandler) SetExitOnCritical(exit bool) {
	eh.exitOnCritical = exit
}

// SetRetryConfig configures retry behavior
func (eh *ErrorHandler) SetRetryConfig(attempts int, delay time.Duration) {
	eh.retryAttempts = attempts
	eh.retryDelay = delay
}

// SetErrorThreshold configures error threshold for degraded mode detection
func (eh *ErrorHandler) SetErrorThreshold(threshold int) {
	eh.errorThreshold = threshold
}

// GetErrorCount returns the current error count
func (eh *ErrorHandler) GetErrorCount() int {
	return eh.errorCount
}

// IsInDegradedMode returns true if the system is in degraded mode
func (eh *ErrorHandler) IsInDegradedMode() bool {
	return eh.errorCount > eh.errorThreshold/2
}

// GetCircuitBreakerState returns the current circuit breaker state
func (eh *ErrorHandler) GetCircuitBreakerState() CircuitState {
	if eh.circuitBreaker != nil {
		return eh.circuitBreaker.GetState()
	}
	return Closed
}

// NewError creates a new structured error
func (eh *ErrorHandler) NewError(message string, cause error, severity ErrorSeverity, component string) *StructuredError {
	// Get stack trace
	stack := make([]byte, 4096)
	length := runtime.Stack(stack, false)
	
	return &StructuredError{
		Message:   message,
		Cause:     cause,
		Severity:  severity,
		Component: component,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Stack:     string(stack[:length]),
	}
}

// Handle processes an error according to its severity
func (eh *ErrorHandler) Handle(err *StructuredError) {
	// Log the error
	eh.logError(err)
	
	// Increment error count for tracking
	eh.errorCount++
	
	// Reset error count periodically (every 5 minutes)
	if time.Since(eh.lastResetTime) > 5*time.Minute {
		eh.errorCount = 0
		eh.lastResetTime = time.Now()
		eh.circuitBreaker.Reset() // Reset circuit breaker periodically
	}
	
	// Take action based on severity
	switch err.Severity {
	case SeverityInfo:
		// Just log, no action needed
	case SeverityWarning:
		// Log warning, continue execution
		// Check if we're getting too many warnings
		if eh.errorCount > eh.errorThreshold {
			log.Printf("⚠️ High warning count (%d), consider investigating", eh.errorCount)
		}
	case SeverityError:
		// Log error, may affect functionality but continue
		// Consider degraded mode if too many errors
		if eh.errorCount > eh.errorThreshold/2 {
			log.Printf("❌ High error count (%d), system may be in degraded state", eh.errorCount)
		}
	case SeverityCritical:
		// Log critical error, may exit application
		if eh.exitOnCritical {
			log.Printf("💥 Critical error encountered, shutting down gracefully...")
			os.Exit(1)
		} else {
			log.Printf("💥 Critical error encountered but continuing due to graceful recovery mode")
		}
	}
}

// HandleWithRetry attempts to retry a function on error with circuit breaker protection
func (eh *ErrorHandler) HandleWithRetry(operation func() error, component string, description string) error {
	// Use circuit breaker to protect against cascading failures
	returnErr := eh.circuitBreaker.Execute(func() error {
		var lastErr error
		
		for attempt := 0; attempt <= eh.retryAttempts; attempt++ {
			if attempt > 0 {
				// Exponential backoff with jitter
				backoffDelay := time.Duration(attempt*attempt) * eh.retryDelay
				if backoffDelay > 30*time.Second {
					backoffDelay = 30 * time.Second
				}
				log.Printf("🔄 Retrying %s (attempt %d/%d) after %v...", description, attempt, eh.retryAttempts, backoffDelay)
				time.Sleep(backoffDelay)
			}
			
			if err := operation(); err != nil {
				lastErr = err
				structuredErr := eh.NewError(
					fmt.Sprintf("Failed %s (attempt %d/%d)", description, attempt+1, eh.retryAttempts+1),
					err,
					SeverityWarning,
					component,
				)
				eh.logError(structuredErr)
				continue
			}
			
			// Success
			if attempt > 0 {
				log.Printf("✅ %s succeeded after %d retries", description, attempt)
			}
			return nil
		}
		
		// All attempts failed
		return lastErr
	})
	
	if returnErr != nil {
		// Increment error count for potential circuit breaking at higher level
		eh.errorCount++
		
		finalErr := eh.NewError(
			fmt.Sprintf("Failed %s after %d attempts", description, eh.retryAttempts+1),
			returnErr,
			SeverityError,
			component,
		)
		eh.Handle(finalErr)
		return finalErr
	}
	
	// Reset error count on success
	eh.errorCount = 0
	return nil
}

// Recover handles panic recovery
func (eh *ErrorHandler) Recover(component string) {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = fmt.Errorf("panic: %s", x)
		case error:
			err = fmt.Errorf("panic: %w", x)
		default:
			err = fmt.Errorf("panic: %v", x)
		}
		
		structuredErr := eh.NewError(
			"Panic recovered",
			err,
			SeverityCritical,
			component,
		)
		eh.Handle(structuredErr)
	}
}

// logError formats and logs an error
func (eh *ErrorHandler) logError(err *StructuredError) {
	severity := eh.getSeverityEmoji(err.Severity)
	timestamp := err.Timestamp.Format("2006-01-02 15:04:05")
	
	// Basic error information
	log.Printf("%s [%s] %s: %s", severity, timestamp, err.Component, err.Error())
	
	// Add context if available
	if len(err.Context) > 0 {
		log.Printf("   Context: %v", err.Context)
	}
	
	// Add stack trace for critical errors
	if err.Severity == SeverityCritical {
		lines := strings.Split(err.Stack, "\n")
		log.Printf("   Stack trace:")
		for i, line := range lines {
			if i > 10 { // Limit stack trace length
				break
			}
			if strings.TrimSpace(line) != "" {
				log.Printf("     %s", line)
			}
		}
	}
}

// getSeverityEmoji returns an emoji for the error severity
func (eh *ErrorHandler) getSeverityEmoji(severity ErrorSeverity) string {
	switch severity {
	case SeverityInfo:
		return "ℹ️"
	case SeverityWarning:
		return "⚠️"
	case SeverityError:
		return "❌"
	case SeverityCritical:
		return "💥"
	default:
		return "❓"
	}
}

// Convenience functions
func (eh *ErrorHandler) Info(message string, component string) {
	err := eh.NewError(message, nil, SeverityInfo, component)
	eh.Handle(err)
}

func (eh *ErrorHandler) Warning(message string, cause error, component string) {
	err := eh.NewError(message, cause, SeverityWarning, component)
	eh.Handle(err)
}

func (eh *ErrorHandler) Error(message string, cause error, component string) {
	err := eh.NewError(message, cause, SeverityError, component)
	eh.Handle(err)
}

func (eh *ErrorHandler) Critical(message string, cause error, component string) {
	err := eh.NewError(message, cause, SeverityCritical, component)
	eh.Handle(err)
}

// AddContext adds context information to an error
func (e *StructuredError) AddContext(key string, value interface{}) *StructuredError {
	e.Context[key] = value
	return e
}

// Global error handler instance
var DefaultHandler = NewErrorHandler()

// Convenience functions using the default handler
func Handle(err *StructuredError) {
	DefaultHandler.Handle(err)
}

func HandleWithRetry(operation func() error, component string, description string) error {
	return DefaultHandler.HandleWithRetry(operation, component, description)
}

func Info(message string, component string) {
	DefaultHandler.Info(message, component)
}

func Warning(message string, cause error, component string) {
	DefaultHandler.Warning(message, cause, component)
}

func ErrorMsg(message string, cause error, component string) {
	DefaultHandler.Error(message, cause, component)
}

func Critical(message string, cause error, component string) {
	DefaultHandler.Critical(message, cause, component)
}

func Recover(component string) {
	DefaultHandler.Recover(component)
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	failureThreshold int
	timeout          time.Duration
	failureCount     int
	lastFailureTime  time.Time
	state            CircuitState
	mu               sync.RWMutex
}

type CircuitState int

const (
	Closed CircuitState = iota // Normal operation
	Open                       // Circuit is open, failing fast
	HalfOpen                   // Testing if service is back
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		state:           Closed,
	}
}

// Execute runs the operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// Check if circuit should be reset from open to half-open
	if cb.state == Open && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.state = HalfOpen
		cb.failureCount = 0
	}
	
	// Fail fast if circuit is open
	if cb.state == Open {
		return fmt.Errorf("circuit breaker is open")
	}
	
	// Execute the operation
	err := operation()
	
	// Handle result
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		
		// Open circuit if threshold exceeded
		if cb.failureCount >= cb.failureThreshold {
			cb.state = Open
		}
		return err
	}
	
	// Success - reset circuit breaker
	if cb.state == HalfOpen {
		cb.state = Closed
	}
	cb.failureCount = 0
	return nil
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.state = Closed
	cb.failureCount = 0
}