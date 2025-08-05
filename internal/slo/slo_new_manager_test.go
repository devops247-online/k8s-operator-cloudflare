package slo

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Separate test file for NewManager to avoid metric conflicts
func TestNewManagerGlobalRegistry(t *testing.T) {
	// Use a custom registry to avoid conflicts
	registry := prometheus.NewRegistry()

	config := DefaultConfig()
	logger := zaptest.NewLogger(t)

	manager, err := NewManagerWithRegistry(config, logger, registry)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.calculator)
	assert.NotNil(t, manager.logger)
}
