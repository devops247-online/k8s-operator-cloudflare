package health

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GracefulShutdownManager handles graceful shutdown of the operator
type GracefulShutdownManager struct {
	shutdownCallbacks []func(context.Context) error
	shutdownTimeout   time.Duration
	mu                sync.RWMutex
	logger            logr.Logger
}

// NewGracefulShutdownManager creates a new graceful shutdown manager
func NewGracefulShutdownManager(timeout time.Duration) *GracefulShutdownManager {
	return &GracefulShutdownManager{
		shutdownCallbacks: make([]func(context.Context) error, 0),
		shutdownTimeout:   timeout,
		logger:            log.Log.WithName("graceful-shutdown"),
	}
}

// AddShutdownCallback adds a callback function to be called during shutdown
func (gsm *GracefulShutdownManager) AddShutdownCallback(callback func(context.Context) error) {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()
	gsm.shutdownCallbacks = append(gsm.shutdownCallbacks, callback)
}

// WaitForShutdownSignal waits for shutdown signals and executes graceful shutdown
func (gsm *GracefulShutdownManager) WaitForShutdownSignal() {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register channel to receive specific signals
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Wait for signal
	sig := <-sigChan
	gsm.logger.Info("Received shutdown signal", "signal", sig)

	// Execute graceful shutdown
	gsm.executeGracefulShutdown()
}

// executeGracefulShutdown executes all registered shutdown callbacks
func (gsm *GracefulShutdownManager) executeGracefulShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), gsm.shutdownTimeout)
	defer cancel()

	gsm.mu.RLock()
	callbacks := make([]func(context.Context) error, len(gsm.shutdownCallbacks))
	copy(callbacks, gsm.shutdownCallbacks)
	gsm.mu.RUnlock()

	gsm.logger.Info("Starting graceful shutdown", "callbacks", len(callbacks), "timeout", gsm.shutdownTimeout)

	// Execute callbacks in reverse order (LIFO)
	for i := len(callbacks) - 1; i >= 0; i-- {
		callback := callbacks[i]
		if err := callback(ctx); err != nil {
			gsm.logger.Error(err, "Error during shutdown callback", "index", i)
		}
	}

	gsm.logger.Info("Graceful shutdown completed")
}

// DefaultShutdownCallbacks returns default shutdown callbacks for the operator
func DefaultShutdownCallbacks() []func(context.Context) error {
	return []func(context.Context) error{
		// Clean up in-flight reconciliations
		func(_ context.Context) error {
			log.Log.Info("Waiting for in-flight reconciliations to complete")
			// Add logic to wait for reconciliations
			time.Sleep(2 * time.Second) // Placeholder
			return nil
		},

		// Clean up connections
		func(_ context.Context) error {
			log.Log.Info("Closing external connections")
			// Add logic to close Cloudflare API connections
			return nil
		},

		// Final cleanup
		func(_ context.Context) error {
			log.Log.Info("Performing final cleanup")
			return nil
		},
	}
}
