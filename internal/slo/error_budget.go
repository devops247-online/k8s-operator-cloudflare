package slo

import (
	"fmt"
	"sync"
	"time"
)

// ErrorBudgetManager manages error budget calculations and policies
type ErrorBudgetManager struct {
	config *ErrorBudgetConfig

	// Historical data for trend analysis
	history map[string][]ErrorBudgetSnapshot

	// Mutex to protect concurrent access
	mu sync.RWMutex
}

// ErrorBudgetSnapshot represents a point-in-time error budget state
type ErrorBudgetSnapshot struct {
	Timestamp       time.Time
	Remaining       float64
	ConsumedPercent float64
	BurnRate        float64
	SLOName         string
	Window          string
}

// NewErrorBudgetManager creates a new error budget manager
func NewErrorBudgetManager(config *ErrorBudgetConfig) *ErrorBudgetManager {
	return &ErrorBudgetManager{
		config:  config,
		history: make(map[string][]ErrorBudgetSnapshot),
	}
}

// CalculateErrorBudget calculates the error budget for an SLO
func (ebm *ErrorBudgetManager) CalculateErrorBudget(sloName string, actual, target float64) *ErrorBudgetCalculation {
	calc := &ErrorBudgetCalculation{
		SLOName:   sloName,
		Actual:    actual,
		Target:    target,
		Timestamp: time.Now(),
	}

	// Calculate total error budget (1 - target)
	calc.TotalBudget = 1.0 - target

	// Calculate consumed budget
	if actual >= target {
		calc.ConsumedBudget = 0.0
	} else {
		calc.ConsumedBudget = target - actual
	}

	// Calculate remaining budget
	calc.RemainingBudget = calc.TotalBudget - calc.ConsumedBudget

	// Calculate percentages
	if calc.TotalBudget > 0 {
		calc.RemainingPercent = calc.RemainingBudget / calc.TotalBudget
		calc.ConsumedPercent = calc.ConsumedBudget / calc.TotalBudget
	} else {
		calc.RemainingPercent = 1.0
		calc.ConsumedPercent = 0.0
	}

	// Ensure percentages are within bounds
	calc.RemainingPercent = clamp(calc.RemainingPercent, 0.0, 1.0)
	calc.ConsumedPercent = clamp(calc.ConsumedPercent, 0.0, 1.0)

	return calc
}

// ErrorBudgetCalculation holds the results of an error budget calculation
type ErrorBudgetCalculation struct {
	SLOName          string
	Actual           float64
	Target           float64
	TotalBudget      float64
	ConsumedBudget   float64
	RemainingBudget  float64
	RemainingPercent float64
	ConsumedPercent  float64
	Timestamp        time.Time
}

// RecordSnapshot records an error budget snapshot for historical tracking
func (ebm *ErrorBudgetManager) RecordSnapshot(snapshot ErrorBudgetSnapshot) {
	ebm.mu.Lock()
	defer ebm.mu.Unlock()

	key := fmt.Sprintf("%s_%s", snapshot.SLOName, snapshot.Window)

	// Initialize slice if needed
	if _, exists := ebm.history[key]; !exists {
		ebm.history[key] = make([]ErrorBudgetSnapshot, 0, 1000)
	}

	// Add snapshot
	ebm.history[key] = append(ebm.history[key], snapshot)

	// Trim old snapshots (keep last 1000)
	if len(ebm.history[key]) > 1000 {
		ebm.history[key] = ebm.history[key][len(ebm.history[key])-1000:]
	}
}

// CalculateBurnRate calculates the error budget burn rate over a time period
func (ebm *ErrorBudgetManager) CalculateBurnRate(sloName, window string, period time.Duration) (float64, error) {
	ebm.mu.RLock()
	defer ebm.mu.RUnlock()

	key := fmt.Sprintf("%s_%s", sloName, window)
	snapshots, exists := ebm.history[key]
	if !exists || len(snapshots) < 2 {
		return 0, fmt.Errorf("insufficient historical data for burn rate calculation")
	}

	// Find snapshots at the start and end of the period
	now := time.Now()
	periodStart := now.Add(-period)

	var startSnapshot, endSnapshot *ErrorBudgetSnapshot

	// Find the closest snapshots to our period boundaries
	for i := range snapshots {
		if snapshots[i].Timestamp.After(periodStart) && startSnapshot == nil {
			if i > 0 {
				startSnapshot = &snapshots[i-1]
			} else {
				startSnapshot = &snapshots[i]
			}
		}
		endSnapshot = &snapshots[i]
	}

	if startSnapshot == nil || endSnapshot == nil {
		return 0, fmt.Errorf("could not find snapshots for period")
	}

	// Calculate burn rate
	timeDiff := endSnapshot.Timestamp.Sub(startSnapshot.Timestamp)
	if timeDiff <= 0 {
		return 0, fmt.Errorf("invalid time difference for burn rate calculation")
	}

	budgetConsumed := startSnapshot.ConsumedPercent - endSnapshot.ConsumedPercent
	if budgetConsumed < 0 {
		budgetConsumed = -budgetConsumed // Absolute value
	}

	// Burn rate = budget consumed per hour
	hoursElapsed := timeDiff.Hours()
	if hoursElapsed > 0 {
		return budgetConsumed / hoursElapsed, nil
	}

	return 0, nil
}

// PredictExhaustion predicts when the error budget will be exhausted
func (ebm *ErrorBudgetManager) PredictExhaustion(sloName, window string) (time.Time, error) {
	// Get current burn rate
	burnRate, err := ebm.CalculateBurnRate(sloName, window, 1*time.Hour)
	if err != nil {
		return time.Time{}, fmt.Errorf("calculating burn rate: %w", err)
	}

	if burnRate <= 0 {
		return time.Time{}, fmt.Errorf("burn rate is zero or negative")
	}

	// Get latest snapshot
	ebm.mu.RLock()
	key := fmt.Sprintf("%s_%s", sloName, window)
	snapshots, exists := ebm.history[key]
	if !exists || len(snapshots) == 0 {
		ebm.mu.RUnlock()
		return time.Time{}, fmt.Errorf("no historical data available")
	}
	latestSnapshot := snapshots[len(snapshots)-1]
	ebm.mu.RUnlock()

	remainingBudget := 1.0 - latestSnapshot.ConsumedPercent

	if remainingBudget <= 0 {
		return latestSnapshot.Timestamp, nil // Already exhausted
	}

	// Calculate hours until exhaustion
	hoursUntilExhaustion := remainingBudget / burnRate

	return time.Now().Add(time.Duration(hoursUntilExhaustion) * time.Hour), nil
}

// CheckAlertThresholds checks if any alert thresholds have been exceeded
func (ebm *ErrorBudgetManager) CheckAlertThresholds(calc *ErrorBudgetCalculation) []AlertThreshold {
	var triggered []AlertThreshold

	// Check warning threshold
	if calc.ConsumedPercent >= ebm.config.AlertThresholds.Warning {
		triggered = append(triggered, AlertThreshold{
			Name:      "warning",
			Threshold: ebm.config.AlertThresholds.Warning,
			Severity:  "warning",
			Message:   fmt.Sprintf("Error budget warning: %.1f%% consumed", calc.ConsumedPercent*100),
		})
	}

	// Check critical threshold
	if calc.ConsumedPercent >= ebm.config.AlertThresholds.Critical {
		triggered = append(triggered, AlertThreshold{
			Name:      "critical",
			Threshold: ebm.config.AlertThresholds.Critical,
			Severity:  "critical",
			Message:   fmt.Sprintf("Error budget critical: %.1f%% consumed", calc.ConsumedPercent*100),
		})
	}

	return triggered
}

// AlertThreshold represents a triggered alert threshold
type AlertThreshold struct {
	Name      string
	Threshold float64
	Severity  string
	Message   string
}

// GetRecommendedAction returns the recommended action based on error budget consumption
func (ebm *ErrorBudgetManager) GetRecommendedAction(consumedPercent float64) string {
	// Sort policies by threshold in descending order to find the highest applicable threshold
	var applicableAction string
	var highestThreshold float64 = -1

	for _, policy := range ebm.config.Policies {
		if consumedPercent >= policy.Threshold && policy.Threshold > highestThreshold {
			applicableAction = policy.Action
			highestThreshold = policy.Threshold
		}
	}

	if applicableAction != "" {
		return applicableAction
	}
	return "continue_normal_operations"
}

// GenerateReport generates an error budget report
func (ebm *ErrorBudgetManager) GenerateReport(sloName, window string) (*ErrorBudgetReport, error) {
	ebm.mu.RLock()
	key := fmt.Sprintf("%s_%s", sloName, window)
	snapshots, exists := ebm.history[key]
	if !exists || len(snapshots) == 0 {
		ebm.mu.RUnlock()
		return nil, fmt.Errorf("no data available for report")
	}

	// Create a copy of snapshots to avoid holding the lock too long
	snapshotsCopy := make([]ErrorBudgetSnapshot, len(snapshots))
	copy(snapshotsCopy, snapshots)
	ebm.mu.RUnlock()

	report := &ErrorBudgetReport{
		SLOName:   sloName,
		Window:    window,
		Generated: time.Now(),
	}

	// Get latest state
	latest := snapshotsCopy[len(snapshotsCopy)-1]
	report.CurrentRemaining = latest.Remaining
	report.CurrentConsumed = latest.ConsumedPercent

	// Calculate burn rate
	windowDuration := time.Duration(ebm.config.WindowDays) * 24 * time.Hour
	burnRate, err := ebm.CalculateBurnRate(sloName, window, windowDuration)
	if err == nil {
		report.BurnRate = burnRate
	}

	// Predict exhaustion
	exhaustionTime, err := ebm.PredictExhaustion(sloName, window)
	if err == nil {
		report.PredictedExhaustion = &exhaustionTime
	}

	// Get recommended action
	report.RecommendedAction = ebm.GetRecommendedAction(latest.ConsumedPercent)

	// Calculate statistics over the window
	report.Statistics = ebm.calculateStatistics(snapshotsCopy)

	return report, nil
}

// ErrorBudgetReport contains a comprehensive error budget report
type ErrorBudgetReport struct {
	SLOName             string
	Window              string
	Generated           time.Time
	CurrentRemaining    float64
	CurrentConsumed     float64
	BurnRate            float64
	PredictedExhaustion *time.Time
	RecommendedAction   string
	Statistics          ErrorBudgetStatistics
}

// ErrorBudgetStatistics contains statistical data about error budget
type ErrorBudgetStatistics struct {
	MinRemaining     float64
	MaxRemaining     float64
	AvgRemaining     float64
	MinBurnRate      float64
	MaxBurnRate      float64
	AvgBurnRate      float64
	TotalViolations  int
	LongestViolation time.Duration
}

// calculateStatistics calculates statistics from snapshots
func (ebm *ErrorBudgetManager) calculateStatistics(snapshots []ErrorBudgetSnapshot) ErrorBudgetStatistics {
	if len(snapshots) == 0 {
		return ErrorBudgetStatistics{}
	}

	stats := ErrorBudgetStatistics{
		MinRemaining: 1.0,
		MaxRemaining: 0.0,
		MinBurnRate:  1e9,
		MaxBurnRate:  0.0,
	}

	var (
		totalRemaining float64
		totalBurnRate  float64
		burnRateCount  int
		inViolation    bool
		violationStart time.Time
	)

	for _, snapshot := range snapshots {
		// Update remaining stats
		if snapshot.Remaining < stats.MinRemaining {
			stats.MinRemaining = snapshot.Remaining
		}
		if snapshot.Remaining > stats.MaxRemaining {
			stats.MaxRemaining = snapshot.Remaining
		}
		totalRemaining += snapshot.Remaining

		// Update burn rate stats
		if snapshot.BurnRate > 0 {
			if snapshot.BurnRate < stats.MinBurnRate {
				stats.MinBurnRate = snapshot.BurnRate
			}
			if snapshot.BurnRate > stats.MaxBurnRate {
				stats.MaxBurnRate = snapshot.BurnRate
			}
			totalBurnRate += snapshot.BurnRate
			burnRateCount++
		}

		// Track violations (when consumed > warning threshold)
		if snapshot.ConsumedPercent >= ebm.config.AlertThresholds.Warning {
			if !inViolation {
				inViolation = true
				violationStart = snapshot.Timestamp
				stats.TotalViolations++
			}
		} else {
			if inViolation {
				violationDuration := snapshot.Timestamp.Sub(violationStart)
				if violationDuration > stats.LongestViolation {
					stats.LongestViolation = violationDuration
				}
				inViolation = false
			}
		}
	}

	// Calculate averages
	if len(snapshots) > 0 {
		stats.AvgRemaining = totalRemaining / float64(len(snapshots))
	}
	if burnRateCount > 0 {
		stats.AvgBurnRate = totalBurnRate / float64(burnRateCount)
	}

	// If still in violation, calculate duration to now
	if inViolation {
		violationDuration := time.Since(violationStart)
		if violationDuration > stats.LongestViolation {
			stats.LongestViolation = violationDuration
		}
	}

	return stats
}

// clamp ensures a value is within min and max bounds
func clamp(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}
