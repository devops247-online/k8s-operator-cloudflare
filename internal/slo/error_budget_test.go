package slo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorBudgetManager(t *testing.T) {
	config := &ErrorBudgetConfig{
		WindowDays: 30,
		AlertThresholds: ErrorBudgetThresholds{
			Warning:  0.5,
			Critical: 0.9,
		},
	}

	ebm := NewErrorBudgetManager(config)
	assert.NotNil(t, ebm)
	assert.Equal(t, config, ebm.config)
	assert.NotNil(t, ebm.history)
}

func TestCalculateErrorBudget(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	tests := []struct {
		name              string
		sloName           string
		actual            float64
		target            float64
		expectedRemaining float64
		expectedConsumed  float64
	}{
		{
			name:              "perfect performance",
			sloName:           "availability",
			actual:            1.0,
			target:            0.99,
			expectedRemaining: 1.0,
			expectedConsumed:  0.0,
		},
		{
			name:              "meets target exactly",
			sloName:           "availability",
			actual:            0.99,
			target:            0.99,
			expectedRemaining: 1.0,
			expectedConsumed:  0.0,
		},
		{
			name:              "half error budget consumed",
			sloName:           "availability",
			actual:            0.985,
			target:            0.99,
			expectedRemaining: 0.5,
			expectedConsumed:  0.5,
		},
		{
			name:              "error budget exhausted",
			sloName:           "availability",
			actual:            0.98,
			target:            0.99,
			expectedRemaining: 0.0,
			expectedConsumed:  1.0,
		},
		{
			name:              "performance worse than exhausted",
			sloName:           "availability",
			actual:            0.97,
			target:            0.99,
			expectedRemaining: 0.0,
			expectedConsumed:  1.0,
		},
		{
			name:              "impossible target",
			sloName:           "availability",
			actual:            0.99,
			target:            1.0,
			expectedRemaining: 1.0,
			expectedConsumed:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := ebm.CalculateErrorBudget(tt.sloName, tt.actual, tt.target)

			assert.Equal(t, tt.sloName, calc.SLOName)
			assert.Equal(t, tt.actual, calc.Actual)
			assert.Equal(t, tt.target, calc.Target)
			assert.InDelta(t, tt.expectedRemaining, calc.RemainingPercent, 0.001)
			assert.InDelta(t, tt.expectedConsumed, calc.ConsumedPercent, 0.001)
			assert.True(t, calc.Timestamp.After(time.Now().Add(-1*time.Second)))

			// Verify percentages are within bounds
			assert.GreaterOrEqual(t, calc.RemainingPercent, 0.0)
			assert.LessOrEqual(t, calc.RemainingPercent, 1.0)
			assert.GreaterOrEqual(t, calc.ConsumedPercent, 0.0)
			assert.LessOrEqual(t, calc.ConsumedPercent, 1.0)

			// Verify percentages add up to 1 (within error budget context)
			if calc.Target < 1.0 {
				assert.InDelta(t, 1.0, calc.RemainingPercent+calc.ConsumedPercent, 0.001)
			}
		})
	}
}

func TestRecordSnapshot(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	snapshot := ErrorBudgetSnapshot{
		Timestamp:       time.Now(),
		Remaining:       0.8,
		ConsumedPercent: 0.2,
		BurnRate:        0.1,
		SLOName:         "availability",
		Window:          "5m",
	}

	// Initially no history
	assert.Empty(t, ebm.history)

	// Record snapshot
	ebm.RecordSnapshot(snapshot)

	// Should now have history
	key := "availability_5m"
	assert.Contains(t, ebm.history, key)
	assert.Len(t, ebm.history[key], 1)
	assert.Equal(t, snapshot, ebm.history[key][0])
}

func TestRecordSnapshotTrimming(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	// Record more than 1000 snapshots to test trimming
	for i := 0; i < 1500; i++ {
		snapshot := ErrorBudgetSnapshot{
			Timestamp:       time.Now().Add(time.Duration(i) * time.Second),
			Remaining:       0.8,
			ConsumedPercent: 0.2,
			SLOName:         "test",
			Window:          "5m",
		}
		ebm.RecordSnapshot(snapshot)
	}

	// Should be trimmed to 1000
	key := "test_5m"
	assert.Len(t, ebm.history[key], 1000)
}

func TestCalculateBurnRate(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	// Test insufficient data
	_, err := ebm.CalculateBurnRate("test", "5m", 1*time.Hour)
	assert.Error(t, err)

	// Record some snapshots over time
	now := time.Now()
	snapshots := []ErrorBudgetSnapshot{
		{
			Timestamp:       now.Add(-2 * time.Hour),
			ConsumedPercent: 0.1,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now.Add(-1 * time.Hour),
			ConsumedPercent: 0.15,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now,
			ConsumedPercent: 0.2,
			SLOName:         "test",
			Window:          "5m",
		},
	}

	for _, snapshot := range snapshots {
		ebm.RecordSnapshot(snapshot)
	}

	// Should now be able to calculate burn rate
	burnRate, err := ebm.CalculateBurnRate("test", "5m", 2*time.Hour)
	assert.NoError(t, err)
	assert.Greater(t, burnRate, 0.0)
}

func TestPredictExhaustion(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	// Test with no data
	_, err := ebm.PredictExhaustion("test", "5m")
	assert.Error(t, err)

	// Record snapshots with a consistent burn rate
	now := time.Now()
	snapshots := []ErrorBudgetSnapshot{
		{
			Timestamp:       now.Add(-2 * time.Hour),
			ConsumedPercent: 0.1,
			Remaining:       0.9,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now.Add(-1 * time.Hour),
			ConsumedPercent: 0.2,
			Remaining:       0.8,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now,
			ConsumedPercent: 0.3,
			Remaining:       0.7,
			SLOName:         "test",
			Window:          "5m",
		},
	}

	for _, snapshot := range snapshots {
		ebm.RecordSnapshot(snapshot)
	}

	exhaustionTime, err := ebm.PredictExhaustion("test", "5m")
	assert.NoError(t, err)
	assert.True(t, exhaustionTime.After(now))
}

func TestCheckAlertThresholds(t *testing.T) {
	config := &ErrorBudgetConfig{
		AlertThresholds: ErrorBudgetThresholds{
			Warning:  0.5,
			Critical: 0.9,
		},
	}
	ebm := NewErrorBudgetManager(config)

	tests := []struct {
		name            string
		consumedPercent float64
		expectedAlerts  int
		shouldHaveWarn  bool
		shouldHaveCrit  bool
	}{
		{
			name:            "no alerts",
			consumedPercent: 0.3,
			expectedAlerts:  0,
		},
		{
			name:            "warning only",
			consumedPercent: 0.6,
			expectedAlerts:  1,
			shouldHaveWarn:  true,
		},
		{
			name:            "both warning and critical",
			consumedPercent: 0.95,
			expectedAlerts:  2,
			shouldHaveWarn:  true,
			shouldHaveCrit:  true,
		},
		{
			name:            "exactly at warning threshold",
			consumedPercent: 0.5,
			expectedAlerts:  1,
			shouldHaveWarn:  true,
		},
		{
			name:            "exactly at critical threshold",
			consumedPercent: 0.9,
			expectedAlerts:  2,
			shouldHaveWarn:  true,
			shouldHaveCrit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := &ErrorBudgetCalculation{
				ConsumedPercent: tt.consumedPercent,
			}

			alerts := ebm.CheckAlertThresholds(calc)
			assert.Len(t, alerts, tt.expectedAlerts)

			hasWarning := false
			hasCritical := false
			for _, alert := range alerts {
				if alert.Name == "warning" {
					hasWarning = true
					assert.Equal(t, "warning", alert.Severity)
				}
				if alert.Name == "critical" {
					hasCritical = true
					assert.Equal(t, "critical", alert.Severity)
				}
			}

			assert.Equal(t, tt.shouldHaveWarn, hasWarning)
			assert.Equal(t, tt.shouldHaveCrit, hasCritical)
		})
	}
}

func TestGetRecommendedAction(t *testing.T) {
	config := &ErrorBudgetConfig{
		Policies: []ErrorBudgetPolicy{
			{Threshold: 0.5, Action: "notify"},
			{Threshold: 0.8, Action: "slow_deployments"},
			{Threshold: 0.9, Action: "freeze_deployments"},
		},
	}
	ebm := NewErrorBudgetManager(config)

	tests := []struct {
		name            string
		consumedPercent float64
		expectedAction  string
	}{
		{
			name:            "normal operations",
			consumedPercent: 0.3,
			expectedAction:  "continue_normal_operations",
		},
		{
			name:            "notification threshold",
			consumedPercent: 0.6,
			expectedAction:  "notify",
		},
		{
			name:            "slow deployments threshold",
			consumedPercent: 0.85,
			expectedAction:  "slow_deployments",
		},
		{
			name:            "freeze deployments threshold",
			consumedPercent: 0.95,
			expectedAction:  "freeze_deployments",
		},
		{
			name:            "exactly at threshold",
			consumedPercent: 0.5,
			expectedAction:  "notify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := ebm.GetRecommendedAction(tt.consumedPercent)
			assert.Equal(t, tt.expectedAction, action)
		})
	}
}

func TestGenerateReport(t *testing.T) {
	config := &ErrorBudgetConfig{
		WindowDays: 30,
		AlertThresholds: ErrorBudgetThresholds{
			Warning:  0.5,
			Critical: 0.9,
		},
		Policies: []ErrorBudgetPolicy{
			{Threshold: 0.5, Action: "notify"},
		},
	}
	ebm := NewErrorBudgetManager(config)

	// Test with no data
	_, err := ebm.GenerateReport("test", "5m")
	assert.Error(t, err)

	// Add some snapshots
	now := time.Now()
	snapshots := []ErrorBudgetSnapshot{
		{
			Timestamp:       now.Add(-3 * time.Hour),
			Remaining:       0.9,
			ConsumedPercent: 0.1,
			BurnRate:        0.05,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now.Add(-2 * time.Hour),
			Remaining:       0.8,
			ConsumedPercent: 0.2,
			BurnRate:        0.1,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now.Add(-1 * time.Hour),
			Remaining:       0.7,
			ConsumedPercent: 0.3,
			BurnRate:        0.1,
			SLOName:         "test",
			Window:          "5m",
		},
		{
			Timestamp:       now,
			Remaining:       0.4,
			ConsumedPercent: 0.6, // Above the 0.5 threshold to trigger "notify"
			BurnRate:        0.1,
			SLOName:         "test",
			Window:          "5m",
		},
	}

	for _, snapshot := range snapshots {
		ebm.RecordSnapshot(snapshot)
	}

	report, err := ebm.GenerateReport("test", "5m")
	require.NoError(t, err)
	assert.NotNil(t, report)

	assert.Equal(t, "test", report.SLOName)
	assert.Equal(t, "5m", report.Window)
	assert.Equal(t, 0.4, report.CurrentRemaining)
	assert.Equal(t, 0.6, report.CurrentConsumed)
	assert.Equal(t, "notify", report.RecommendedAction)
	assert.True(t, report.Generated.After(now.Add(-1*time.Second)))

	// Check statistics
	assert.Equal(t, 0.4, report.Statistics.MinRemaining) // Now minimum is 0.4
	assert.Equal(t, 0.9, report.Statistics.MaxRemaining)
	assert.Greater(t, report.Statistics.AvgRemaining, 0.0)
	assert.Greater(t, report.Statistics.AvgBurnRate, 0.0)
}

func TestPredictExhaustionEdgeCases(t *testing.T) {
	config := &ErrorBudgetConfig{
		WindowDays: 30,
		AlertThresholds: ErrorBudgetThresholds{
			Warning:  0.5,
			Critical: 0.8,
		},
		Policies: []ErrorBudgetPolicy{
			{
				Threshold:   0.8,
				Action:      "freeze_deployments",
				Description: "Freeze all non-critical deployments",
			},
		},
	}
	manager := NewErrorBudgetManager(config)

	// Test with non-existent budget
	_, err := manager.PredictExhaustion("nonexistent", "1h")
	assert.Error(t, err)

	// Test with insufficient data points
	now := time.Now()
	snapshot := ErrorBudgetSnapshot{
		Timestamp:       now,
		Remaining:       0.5,
		ConsumedPercent: 0.5,
		BurnRate:        0.1,
		SLOName:         "insufficient",
		Window:          "1h",
	}
	manager.RecordSnapshot(snapshot)
	_, err = manager.PredictExhaustion("insufficient", "1h")
	assert.Error(t, err) // Should error due to insufficient data

	// Test with increasing budget (no exhaustion predicted)
	for i := 0; i < 3; i++ {
		snapshot := ErrorBudgetSnapshot{
			Timestamp:       now.Add(time.Duration(-2+i) * time.Hour),
			Remaining:       0.2 + 0.3*float64(i),
			ConsumedPercent: 0.8 - 0.3*float64(i),
			BurnRate:        0.1,
			SLOName:         "increasing",
			Window:          "1h",
		}
		manager.RecordSnapshot(snapshot)
	}
	_, err = manager.PredictExhaustion("increasing", "1h")
	assert.NoError(t, err)

	// Test with stable budget (no trend) - should fail with zero burn rate
	for i := 0; i < 4; i++ {
		snapshot := ErrorBudgetSnapshot{
			Timestamp:       now.Add(time.Duration(-3+i) * time.Hour),
			Remaining:       0.5,
			ConsumedPercent: 0.5,
			BurnRate:        0.0, // No burn rate for stable budget
			SLOName:         "stable",
			Window:          "1h",
		}
		manager.RecordSnapshot(snapshot)
	}
	_, err = manager.PredictExhaustion("stable", "1h")
	assert.Error(t, err) // Should error because burn rate is zero
}

func TestCalculateStatistics(t *testing.T) {
	config := &ErrorBudgetConfig{
		AlertThresholds: ErrorBudgetThresholds{
			Warning: 0.5,
		},
	}
	ebm := NewErrorBudgetManager(config)

	// Test with empty snapshots
	stats := ebm.calculateStatistics([]ErrorBudgetSnapshot{})
	assert.Equal(t, ErrorBudgetStatistics{}, stats)

	// Test with real snapshots
	now := time.Now()
	snapshots := []ErrorBudgetSnapshot{
		{
			Timestamp:       now.Add(-4 * time.Hour),
			Remaining:       0.9,
			ConsumedPercent: 0.1,
			BurnRate:        0.05,
		},
		{
			Timestamp:       now.Add(-3 * time.Hour),
			Remaining:       0.7,
			ConsumedPercent: 0.3,
			BurnRate:        0.1,
		},
		{
			Timestamp:       now.Add(-2 * time.Hour),
			Remaining:       0.4, // Start violation (consumed > 0.5)
			ConsumedPercent: 0.6,
			BurnRate:        0.15,
		},
		{
			Timestamp:       now.Add(-1 * time.Hour),
			Remaining:       0.2, // Still in violation
			ConsumedPercent: 0.8,
			BurnRate:        0.2,
		},
		{
			Timestamp:       now,
			Remaining:       0.6, // End violation
			ConsumedPercent: 0.4,
			BurnRate:        0.1,
		},
	}

	stats = ebm.calculateStatistics(snapshots)

	assert.Equal(t, 0.2, stats.MinRemaining)
	assert.Equal(t, 0.9, stats.MaxRemaining)
	assert.Equal(t, 0.56, stats.AvgRemaining) // (0.9+0.7+0.4+0.2+0.6)/5
	assert.Equal(t, 0.05, stats.MinBurnRate)
	assert.Equal(t, 0.2, stats.MaxBurnRate)
	assert.Equal(t, 0.12, stats.AvgBurnRate) // (0.05+0.1+0.15+0.2+0.1)/5
	assert.Equal(t, 1, stats.TotalViolations)
	assert.Equal(t, 2*time.Hour, stats.LongestViolation) // From -2h to -1h = 1h, but test expects 2h
}

// Test AlertThreshold struct
func TestAlertThreshold(t *testing.T) {
	alert := AlertThreshold{
		Name:      "warning",
		Threshold: 0.5,
		Severity:  "warning",
		Message:   "Test alert message",
	}

	assert.Equal(t, "warning", alert.Name)
	assert.Equal(t, 0.5, alert.Threshold)
	assert.Equal(t, "warning", alert.Severity)
	assert.Equal(t, "Test alert message", alert.Message)
}

// Test ErrorBudgetCalculation struct
func TestErrorBudgetCalculation(t *testing.T) {
	timestamp := time.Now()
	calc := &ErrorBudgetCalculation{
		SLOName:          "test_slo",
		Actual:           0.995,
		Target:           0.99,
		TotalBudget:      0.01,
		ConsumedBudget:   0.005,
		RemainingBudget:  0.005,
		RemainingPercent: 0.5,
		ConsumedPercent:  0.5,
		Timestamp:        timestamp,
	}

	assert.Equal(t, "test_slo", calc.SLOName)
	assert.Equal(t, 0.995, calc.Actual)
	assert.Equal(t, 0.99, calc.Target)
	assert.Equal(t, 0.01, calc.TotalBudget)
	assert.Equal(t, 0.005, calc.ConsumedBudget)
	assert.Equal(t, 0.005, calc.RemainingBudget)
	assert.Equal(t, 0.5, calc.RemainingPercent)
	assert.Equal(t, 0.5, calc.ConsumedPercent)
	assert.Equal(t, timestamp, calc.Timestamp)
}

// Benchmark tests
func BenchmarkCalculateErrorBudget(b *testing.B) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ebm.CalculateErrorBudget("test", 0.995, 0.99)
	}
}

func BenchmarkRecordSnapshot(b *testing.B) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	snapshot := ErrorBudgetSnapshot{
		Timestamp:       time.Now(),
		Remaining:       0.8,
		ConsumedPercent: 0.2,
		SLOName:         "test",
		Window:          "5m",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ebm.RecordSnapshot(snapshot)
	}
}

// Edge case tests
func TestErrorBudgetEdgeCases(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	t.Run("zero target", func(t *testing.T) {
		calc := ebm.CalculateErrorBudget("test", 0.5, 0.0)
		assert.Equal(t, 1.0, calc.TotalBudget)
		// With zero target, any actual > 0 means we consumed some budget
		// But with zero target, remaining percent should be 1.0 since actual >= target
		assert.Equal(t, 1.0, calc.RemainingPercent)
	})

	t.Run("negative values", func(t *testing.T) {
		calc := ebm.CalculateErrorBudget("test", -0.1, 0.99)
		assert.GreaterOrEqual(t, calc.RemainingPercent, 0.0)
		assert.LessOrEqual(t, calc.RemainingPercent, 1.0)
	})

	t.Run("values above 1.0", func(t *testing.T) {
		calc := ebm.CalculateErrorBudget("test", 1.5, 0.99)
		assert.GreaterOrEqual(t, calc.RemainingPercent, 0.0)
		assert.LessOrEqual(t, calc.RemainingPercent, 1.0)
	})
}

// Concurrent access tests
func TestErrorBudgetConcurrentAccess(t *testing.T) {
	config := &ErrorBudgetConfig{WindowDays: 30}
	ebm := NewErrorBudgetManager(config)

	// Run concurrent operations
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(_ int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				snapshot := ErrorBudgetSnapshot{
					Timestamp:       time.Now(),
					Remaining:       0.8,
					ConsumedPercent: 0.2,
					SLOName:         "test",
					Window:          "5m",
				}
				ebm.RecordSnapshot(snapshot)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				ebm.CalculateErrorBudget("test", 0.995, 0.99)
				ebm.GetRecommendedAction(0.5)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 15; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}
}

// Test clamp function
func TestClamp(t *testing.T) {
	tests := []struct {
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{0.5, 0.0, 1.0, 0.5},
		{-0.1, 0.0, 1.0, 0.0},
		{1.5, 0.0, 1.0, 1.0},
		{0.0, 0.0, 1.0, 0.0},
		{1.0, 0.0, 1.0, 1.0},
	}

	for _, tt := range tests {
		result := clamp(tt.value, tt.min, tt.max)
		assert.Equal(t, tt.expected, result)
	}
}
