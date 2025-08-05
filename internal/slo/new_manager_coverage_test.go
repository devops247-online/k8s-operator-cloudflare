package slo

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// Test for NewManager function to achieve 100% coverage
// This test is in a separate file to isolate metric registration
func TestNewManagerCoverageOnly(t *testing.T) {
	// Create a completely fresh registry for this test
	oldDefaultRegisterer := prometheus.DefaultRegisterer
	defer func() {
		prometheus.DefaultRegisterer = oldDefaultRegisterer
	}()

	// Use a fresh registry as default
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	config := DefaultConfig()
	logger := zaptest.NewLogger(t)

	// This calls the original NewManager function which uses DefaultRegisterer
	manager, err := NewManager(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
}
