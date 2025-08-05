package slo

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Helper function to create a manager with separate registry for tests
func newTestManager(t *testing.T, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	return NewManagerWithRegistry(config, logger, registry)
}

// Helper function that preserves nil config for testing
func newTestManagerWithNilConfig(_ *testing.T, config *Config, logger *zap.Logger) (*Manager, error) {
	registry := prometheus.NewRegistry()
	return NewManagerWithRegistry(config, logger, registry)
}

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	manager, err := newTestManager(t, config)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.calculator)
	assert.NotNil(t, manager.logger)
	assert.NotNil(t, manager.lastCalculation)
	assert.NotNil(t, manager.errorBudgets)
	assert.NotNil(t, manager.stopCh)
	assert.NotNil(t, manager.doneCh)
}

func TestNewManagerWithDefaultRegistry(t *testing.T) {
	// Use a custom registry to avoid conflicts with other tests
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

func TestNewManagerWithInvalidConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test with nil config using separate registry
	_, err := newTestManagerWithNilConfig(t, nil, logger)
	assert.Error(t, err)

	// Test with nil logger
	config := DefaultConfig()
	_, err = newTestManagerWithNilConfig(t, config, nil)
	assert.Error(t, err)

	// Test with disabled config
	disabledConfig := DefaultConfig()
	disabledConfig.Enabled = false
	manager, err := newTestManager(t, disabledConfig)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.False(t, manager.config.Enabled)
}

func TestManagerStart(t *testing.T) {
	manager, err := newTestManager(t, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = manager.Start(ctx)
	assert.NoError(t, err)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	manager.Stop()
}

func TestManagerStartDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	ctx := context.Background()
	err = manager.Start(ctx)
	assert.NoError(t, err)

	manager.Stop()
}

func TestManagerStop(t *testing.T) {

	config := DefaultConfig()
	config.Windows = []TimeWindow{
		{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true},
	}

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = manager.Start(ctx)
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan bool, 1)
	go func() {
		manager.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() took too long")
	}
}

func TestUpdateSingleErrorBudget(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Test updating error budget
	manager.updateSingleErrorBudget("availability", 0.998, 0.999, "5m")

	// Check that the error budget was stored
	manager.mu.RLock()
	state, exists := manager.errorBudgets["availability_5m"]
	manager.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, "availability", state.SLOName)
	assert.GreaterOrEqual(t, state.CurrentRemaining, 0.0)
	assert.LessOrEqual(t, state.CurrentRemaining, 1.0)
	assert.True(t, state.LastUpdated.After(time.Now().Add(-1*time.Second)))
}

func TestUpdateErrorBudgets(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	values := &SLIValues{
		Availability: 0.998,
		SuccessRate:  0.992,
		LatencyP95:   35 * time.Second,
		Window:       "5m",
	}

	manager.updateErrorBudgets(values)

	// Check that all error budgets were updated
	manager.mu.RLock()
	availabilityState := manager.errorBudgets["availability_5m"]
	successRateState := manager.errorBudgets["success_rate_5m"]
	latencyState := manager.errorBudgets["latency_p95_5m"]
	manager.mu.RUnlock()

	assert.NotNil(t, availabilityState)
	assert.NotNil(t, successRateState)
	assert.NotNil(t, latencyState)

	assert.Equal(t, "availability", availabilityState.SLOName)
	assert.Equal(t, "success_rate", successRateState.SLOName)
	assert.Equal(t, "latency_p95", latencyState.SLOName)
}

func TestCheckErrorBudgetPolicies(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Test with state that should trigger policies
	state := &ErrorBudgetState{
		SLOName:         "test",
		ConsumedPercent: 0.6, // Above warning threshold (0.5)
	}

	manager.checkErrorBudgetPolicies(state)

	// Should have triggered the first policy
	assert.Contains(t, state.PolicyTriggered, "notify")

	// Test with state that should trigger critical policy
	state.ConsumedPercent = 0.95
	manager.checkErrorBudgetPolicies(state)

	// Should have triggered both policies
	assert.Contains(t, state.PolicyTriggered, "freeze_deployments")
}

func TestGetSLIValues(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Initially should not exist
	_, exists := manager.GetSLIValues("5m")
	assert.False(t, exists)

	// Add some test values
	testValues := &SLIValues{
		Availability: 0.999,
		SuccessRate:  0.995,
		Window:       "5m",
		Timestamp:    time.Now(),
	}

	manager.mu.Lock()
	manager.lastCalculation["5m"] = testValues
	manager.mu.Unlock()

	// Should now exist
	values, exists := manager.GetSLIValues("5m")
	assert.True(t, exists)
	assert.Equal(t, testValues, values)
}

func TestGetErrorBudgetState(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Initially should not exist
	_, exists := manager.GetErrorBudgetState("availability", "5m")
	assert.False(t, exists)

	// Add some test state
	testState := &ErrorBudgetState{
		SLOName:          "availability",
		CurrentRemaining: 0.8,
		ConsumedPercent:  0.2,
		LastUpdated:      time.Now(),
	}

	manager.mu.Lock()
	manager.errorBudgets["availability_5m"] = testState
	manager.mu.Unlock()

	// Should now exist
	state, exists := manager.GetErrorBudgetState("availability", "5m")
	assert.True(t, exists)
	assert.Equal(t, testState, state)
}

func TestGetAllErrorBudgets(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Initially should be empty
	budgets := manager.GetAllErrorBudgets()
	assert.Empty(t, budgets)

	// Add some test states
	testState1 := &ErrorBudgetState{
		SLOName:          "availability",
		CurrentRemaining: 0.8,
	}
	testState2 := &ErrorBudgetState{
		SLOName:          "success_rate",
		CurrentRemaining: 0.9,
	}

	manager.mu.Lock()
	manager.errorBudgets["availability_5m"] = testState1
	manager.errorBudgets["success_rate_5m"] = testState2
	manager.mu.Unlock()

	// Should return copies of all states
	budgets = manager.GetAllErrorBudgets()
	assert.Len(t, budgets, 2)
	assert.Contains(t, budgets, "availability_5m")
	assert.Contains(t, budgets, "success_rate_5m")

	// Verify they are copies (modifying returned map shouldn't affect original)
	delete(budgets, "availability_5m")

	originalBudgets := manager.GetAllErrorBudgets()
	assert.Len(t, originalBudgets, 2)
}

func TestCalculateForWindow(t *testing.T) {

	config := DefaultConfig()

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	ctx := context.Background()
	window := TimeWindow{
		Name:        "5m",
		Duration:    5 * time.Minute,
		IsShortTerm: true,
	}

	// Should not panic and should update internal state
	manager.calculateForWindow(ctx, window)

	// Check if values were calculated and stored
	values, exists := manager.GetSLIValues("5m")
	assert.True(t, exists)
	assert.Equal(t, "5m", values.Window)

	// Check if error budgets were updated
	budgets := manager.GetAllErrorBudgets()
	assert.NotEmpty(t, budgets)
}

// Test ErrorBudgetState struct
func TestErrorBudgetState(t *testing.T) {
	state := &ErrorBudgetState{
		SLOName:          "test_slo",
		CurrentRemaining: 0.75,
		ConsumedPercent:  0.25,
		LastUpdated:      time.Now(),
		PolicyTriggered:  []string{"notify"},
		BurnRate:         0.1,
		TimeToExhaustion: 10 * time.Hour,
	}

	assert.Equal(t, "test_slo", state.SLOName)
	assert.Equal(t, 0.75, state.CurrentRemaining)
	assert.Equal(t, 0.25, state.ConsumedPercent)
	assert.Contains(t, state.PolicyTriggered, "notify")
	assert.Equal(t, 0.1, state.BurnRate)
	assert.Equal(t, 10*time.Hour, state.TimeToExhaustion)
}

// Concurrent access tests
func TestManagerConcurrentAccess(t *testing.T) {

	config := DefaultConfig()
	config.Windows = []TimeWindow{
		{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true},
	}

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	// Start multiple goroutines accessing the manager concurrently
	done := make(chan bool)
	errors := make(chan error, 20)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				manager.GetSLIValues("5m")
				manager.GetErrorBudgetState("availability", "5m")
				manager.GetAllErrorBudgets()
			}
		}()
	}

	// Writers
	for i := 0; i < 5; i++ {
		go func(_ int) {
			defer func() { done <- true }()
			for j := 0; j < 50; j++ {
				values := &SLIValues{
					Availability: 0.999,
					Window:       "5m",
					Timestamp:    time.Now(),
				}
				manager.updateErrorBudgets(values)
			}
		}(i)
	}

	// Window calculators
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			ctx := context.Background()
			window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}
			for j := 0; j < 20; j++ {
				manager.calculateForWindow(ctx, window)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Concurrent access error: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}
}

// Benchmark tests
func BenchmarkManagerCalculateForWindow(b *testing.B) {
	config := DefaultConfig()
	logger := zap.NewNop()
	registry := prometheus.NewRegistry()

	manager, err := NewManagerWithRegistry(config, logger, registry)
	require.NoError(b, err)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.calculateForWindow(ctx, window)
	}
}

func BenchmarkManagerGetSLIValues(b *testing.B) {
	config := DefaultConfig()
	logger := zap.NewNop()
	registry := prometheus.NewRegistry()

	manager, err := NewManagerWithRegistry(config, logger, registry)
	require.NoError(b, err)

	// Pre-populate with some values
	testValues := &SLIValues{Window: "5m", Timestamp: time.Now()}
	manager.mu.Lock()
	manager.lastCalculation["5m"] = testValues
	manager.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetSLIValues("5m")
	}
}

// Integration tests
func TestManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultConfig()
	config.Windows = []TimeWindow{
		{Name: "1m", Duration: 1 * time.Minute, IsShortTerm: true},
	}

	manager, err := newTestManager(t, config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the manager
	err = manager.Start(ctx)
	require.NoError(t, err)

	// Let it run for a bit
	time.Sleep(2 * time.Second)

	// Check that calculations were performed
	values, exists := manager.GetSLIValues("1m")
	assert.True(t, exists)
	assert.NotNil(t, values)

	budgets := manager.GetAllErrorBudgets()
	assert.NotEmpty(t, budgets)

	manager.Stop()
}

// Edge case tests
func TestManagerEdgeCases(t *testing.T) {

	t.Run("empty windows configuration", func(t *testing.T) {
		config := DefaultConfig()
		config.Windows = []TimeWindow{}

		manager, err := newTestManager(t, config)
		require.NoError(t, err)

		ctx := context.Background()
		err = manager.Start(ctx)
		assert.NoError(t, err)

		manager.Stop()
	})

	t.Run("single window configuration", func(t *testing.T) {
		config := DefaultConfig()
		config.Windows = []TimeWindow{
			{Name: "test", Duration: 1 * time.Second, IsShortTerm: true},
		}

		manager, err := newTestManager(t, config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = manager.Start(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		manager.Stop()
	})

	t.Run("very short window intervals", func(t *testing.T) {
		config := DefaultConfig()
		config.Windows = []TimeWindow{
			{Name: "100ms", Duration: 100 * time.Millisecond, IsShortTerm: true},
		}

		manager, err := newTestManager(t, config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err = manager.Start(ctx)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)
		manager.Stop()
	})
}
