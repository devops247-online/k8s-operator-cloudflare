package slo

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestNewRecordingRulesGenerator(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	assert.NotNil(t, generator)
	assert.Equal(t, config, generator.config)
}

func TestGenerateRecordingRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	rules := generator.GenerateRecordingRules()

	assert.NotNil(t, rules)
	assert.Len(t, rules.Groups, 4) // SLI, SLO burn rate, Error budget, Multi-window

	// Check group names
	groupNames := make([]string, len(rules.Groups))
	for i, group := range rules.Groups {
		groupNames[i] = group.Name
	}

	expectedGroups := []string{
		"cloudflare-dns-operator.sli",
		"cloudflare-dns-operator.slo.burn_rate",
		"cloudflare-dns-operator.slo.error_budget",
		"cloudflare-dns-operator.slo.multi_window",
	}

	for _, expected := range expectedGroups {
		assert.Contains(t, groupNames, expected)
	}
}

func TestGenerateSLIRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	sliGroup := generator.generateSLIRules()

	assert.Equal(t, "cloudflare-dns-operator.sli", sliGroup.Name)
	assert.Equal(t, "30s", sliGroup.Interval)
	assert.NotEmpty(t, sliGroup.Rules)

	// Check that we have rules for all SLI types and windows
	sliTypes := []string{"availability", "success_rate", "latency_p95", "throughput"}
	windows := []string{"5m", "30m", "1h"}

	for _, sliType := range sliTypes {
		for _, window := range windows {
			found := false
			for _, rule := range sliGroup.Rules {
				if strings.Contains(rule.Record, sliType) &&
					rule.Labels["window"] == window {
					found = true
					break
				}
			}
			assert.True(t, found, "Missing rule for %s with window %s", sliType, window)
		}
	}

	// Verify rule structure
	for _, rule := range sliGroup.Rules {
		assert.NotEmpty(t, rule.Record, "Rule record should not be empty")
		assert.NotEmpty(t, rule.Expr, "Rule expression should not be empty")
		assert.Contains(t, rule.Record, "sli:", "SLI rule should start with 'sli:'")

		// Check labels
		assert.NotEmpty(t, rule.Labels["sli_type"], "SLI type label should not be empty")
		assert.NotEmpty(t, rule.Labels["window"], "Window label should not be empty")
	}
}

func TestGenerateSLOBurnRateRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	burnRateGroup := generator.generateSLOBurnRateRules()

	assert.Equal(t, "cloudflare-dns-operator.slo.burn_rate", burnRateGroup.Name)
	assert.Equal(t, "30s", burnRateGroup.Interval)
	assert.NotEmpty(t, burnRateGroup.Rules)

	// Check that we have burn rate rules for all SLO types
	sloTypes := []string{"availability", "success_rate", "latency_p95", "throughput"}
	windows := []string{"5m", "30m", "1h", "6h", "24h"}

	for _, sloType := range sloTypes {
		for _, window := range windows {
			found := false
			for _, rule := range burnRateGroup.Rules {
				if strings.Contains(rule.Record, sloType) &&
					rule.Labels["window"] == window {
					found = true
					break
				}
			}
			assert.True(t, found, "Missing burn rate rule for %s with window %s", sloType, window)
		}
	}

	// Verify rule structure
	for _, rule := range burnRateGroup.Rules {
		assert.NotEmpty(t, rule.Record, "Rule record should not be empty")
		assert.NotEmpty(t, rule.Expr, "Rule expression should not be empty")
		assert.Contains(t, rule.Record, "slo:burn_rate:", "Burn rate rule should contain 'slo:burn_rate:'")

		// Check labels
		assert.NotEmpty(t, rule.Labels["slo_type"], "SLO type label should not be empty")
		assert.NotEmpty(t, rule.Labels["window"], "Window label should not be empty")
		assert.NotEmpty(t, rule.Labels["target"], "Target label should not be empty")
	}
}

func TestGenerateErrorBudgetRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	errorBudgetGroup := generator.generateErrorBudgetRules()

	assert.Equal(t, "cloudflare-dns-operator.slo.error_budget", errorBudgetGroup.Name)
	assert.Equal(t, "1m", errorBudgetGroup.Interval)
	assert.NotEmpty(t, errorBudgetGroup.Rules)

	// Check for error budget remaining rules
	remainingRules := 0
	burnRateRules := 0

	for _, rule := range errorBudgetGroup.Rules {
		assert.NotEmpty(t, rule.Record, "Rule record should not be empty")
		assert.NotEmpty(t, rule.Expr, "Rule expression should not be empty")
		assert.Contains(t, rule.Record, "slo:error_budget", "Error budget rule should contain 'slo:error_budget'")

		// Check labels
		assert.NotEmpty(t, rule.Labels["slo_type"], "SLO type label should not be empty")
		assert.NotEmpty(t, rule.Labels["window"], "Window label should not be empty")
		assert.NotEmpty(t, rule.Labels["budget_type"], "Budget type label should not be empty")

		switch rule.Labels["budget_type"] {
		case "remaining":
			remainingRules++
		case "burn_rate":
			burnRateRules++
		}
	}

	assert.Greater(t, remainingRules, 0, "Should have error budget remaining rules")
	assert.Greater(t, burnRateRules, 0, "Should have error budget burn rate rules")
}

func TestGenerateMultiWindowRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	multiWindowGroup := generator.generateMultiWindowRules()

	assert.Equal(t, "cloudflare-dns-operator.slo.multi_window", multiWindowGroup.Name)
	assert.Equal(t, "30s", multiWindowGroup.Interval)
	assert.NotEmpty(t, multiWindowGroup.Rules)

	pageAlerts := 0
	ticketAlerts := 0

	for _, rule := range multiWindowGroup.Rules {
		assert.NotEmpty(t, rule.Record, "Rule record should not be empty")
		assert.NotEmpty(t, rule.Expr, "Rule expression should not be empty")
		assert.Contains(t, rule.Record, "slo:multi_window_burn_rate:", //nolint:lll
			"Multi-window rule should contain 'slo:multi_window_burn_rate:'")

		// Check labels
		assert.NotEmpty(t, rule.Labels["slo_type"], "SLO type label should not be empty")
		assert.NotEmpty(t, rule.Labels["short_window"], "Short window label should not be empty")
		assert.NotEmpty(t, rule.Labels["long_window"], "Long window label should not be empty")
		assert.NotEmpty(t, rule.Labels["severity"], "Severity label should not be empty")
		assert.NotEmpty(t, rule.Labels["threshold"], "Threshold label should not be empty")

		switch rule.Labels["severity"] {
		case "page":
			pageAlerts++
		case "ticket":
			ticketAlerts++
		}
	}

	assert.Greater(t, pageAlerts, 0, "Should have page alert rules")
	assert.Greater(t, ticketAlerts, 0, "Should have ticket alert rules")
}

func TestGenerateYAML(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	yamlOutput, err := generator.GenerateYAML()

	assert.NoError(t, err)
	assert.NotEmpty(t, yamlOutput)

	// Verify it's valid YAML
	var parsed PrometheusRulesSpec
	err = yaml.Unmarshal([]byte(yamlOutput), &parsed)
	assert.NoError(t, err)

	// Verify structure
	assert.Len(t, parsed.Groups, 4)

	// Check that the YAML contains expected content
	assert.Contains(t, yamlOutput, "groups:")
	assert.Contains(t, yamlOutput, "cloudflare-dns-operator.sli")
	assert.Contains(t, yamlOutput, "sli:availability:rate5m")
	assert.Contains(t, yamlOutput, "slo:burn_rate:")
}

func TestGeneratePrometheusRuleCRD(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	crdYAML, err := generator.GeneratePrometheusRuleCRD("test-slo-rules", "monitoring")

	assert.NoError(t, err)
	assert.NotEmpty(t, crdYAML)

	// Verify it's valid CRD YAML
	var crd map[string]interface{}
	err = yaml.Unmarshal([]byte(crdYAML), &crd)
	assert.NoError(t, err)

	// Check CRD structure
	assert.Equal(t, "monitoring.coreos.com/v1", crd["apiVersion"])
	assert.Equal(t, "PrometheusRule", crd["kind"])

	metadata, ok := crd["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test-slo-rules", metadata["name"])
	assert.Equal(t, "monitoring", metadata["namespace"])

	labels, ok := metadata["labels"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "cloudflare-dns-operator", labels["app.kubernetes.io/name"])
	assert.Equal(t, "slo-monitoring", labels["app.kubernetes.io/component"])

	// Check spec exists
	spec, ok := crd["spec"].(map[string]interface{})
	assert.True(t, ok)

	groups, ok := spec["groups"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, groups, 4)
}

func TestValidateRecordingRules(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	issues := generator.ValidateRecordingRules()

	// Should have no validation issues
	assert.Empty(t, issues, "Recording rules should pass validation: %v", issues)
}

func TestValidateRecordingRulesDetailed(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	// Test by validating rules with actual implementation
	issues := generator.ValidateRecordingRules()

	// The default generated rules should be valid
	assert.Empty(t, issues, "Default recording rules should be valid: %v", issues)

	// Now test with a custom generator that has empty windows to trigger some validation paths
	emptyWindowsConfig := &Config{
		Enabled: true,
		Targets: SLOTargets{
			Availability:  99.9,
			SuccessRate:   99.5,
			LatencyP95:    30 * time.Second,
			ThroughputMin: 10.0,
		},
		Windows: []TimeWindow{}, // Empty windows
	}

	emptyGenerator := NewRecordingRulesGenerator(emptyWindowsConfig)
	emptyIssues := emptyGenerator.ValidateRecordingRules()

	// With empty windows, there might be fewer rules but should still be valid
	// The slice will be empty (length 0) but not nil
	assert.Equal(t, 0, len(emptyIssues))
}

func TestGetRecordingRuleNames(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	names := generator.GetRecordingRuleNames()

	assert.NotEmpty(t, names)

	// Check for expected rule names
	expectedPatterns := []string{
		"sli:availability:rate5m",
		"sli:success_rate:rate5m",
		"sli:latency_p95:5m",
		"sli:throughput:rate5m",
		"slo:burn_rate:availability:5m",
		"slo:error_budget_remaining:availability:30d",
		"slo:multi_window_burn_rate:availability:5m_1h",
	}

	for _, pattern := range expectedPatterns {
		found := false
		for _, name := range names {
			if strings.Contains(name, pattern) || name == pattern {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find rule matching pattern: %s", pattern)
	}

	// Verify all names follow the convention
	for _, name := range names {
		assert.Contains(t, name, ":", "Rule name should contain colon: %s", name)
		assert.True(t, strings.HasPrefix(name, "sli:") || strings.HasPrefix(name, "slo:"),
			"Rule name should start with 'sli:' or 'slo:': %s", name)
	}
}

func TestRecordingRuleStructures(t *testing.T) {
	t.Run("RecordingRule", func(t *testing.T) {
		rule := RecordingRule{
			Record: "test:metric:rate5m",
			Expr:   "rate(test_metric[5m])",
			Labels: map[string]string{
				"service": "test",
				"window":  "5m",
			},
		}

		assert.Equal(t, "test:metric:rate5m", rule.Record)
		assert.Equal(t, "rate(test_metric[5m])", rule.Expr)
		assert.Len(t, rule.Labels, 2)
		assert.Equal(t, "test", rule.Labels["service"])
		assert.Equal(t, "5m", rule.Labels["window"])
	})

	t.Run("RecordingRuleGroup", func(t *testing.T) {
		group := RecordingRuleGroup{
			Name:     "test.group",
			Interval: "30s",
			Rules: []RecordingRule{
				{Record: "test:rule1", Expr: "up"},
				{Record: "test:rule2", Expr: "up{job=\"test\"}"},
			},
		}

		assert.Equal(t, "test.group", group.Name)
		assert.Equal(t, "30s", group.Interval)
		assert.Len(t, group.Rules, 2)
	})

	t.Run("PrometheusRulesSpec", func(t *testing.T) {
		spec := PrometheusRulesSpec{
			Groups: []RecordingRuleGroup{
				{Name: "group1", Rules: []RecordingRule{{Record: "rule1", Expr: "up"}}},
				{Name: "group2", Rules: []RecordingRule{{Record: "rule2", Expr: "up"}}},
			},
		}

		assert.Len(t, spec.Groups, 2)
		assert.Equal(t, "group1", spec.Groups[0].Name)
		assert.Equal(t, "group2", spec.Groups[1].Name)
	})
}

func TestRecordingRulesWithCustomConfig(t *testing.T) {
	// Test with custom SLO targets
	config := &Config{
		Enabled: true,
		Targets: SLOTargets{
			Availability:  99.9, // Higher availability target
			SuccessRate:   99.5, // Lower success rate target
			LatencyP95:    500 * time.Millisecond,
			ThroughputMin: 50.0,
		},
		Windows: []TimeWindow{
			{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true},
			{Name: "1h", Duration: 1 * time.Hour, IsShortTerm: false},
		},
	}

	generator := NewRecordingRulesGenerator(config)
	rules := generator.GenerateRecordingRules()

	assert.NotNil(t, rules)
	assert.NotEmpty(t, rules.Groups)

	// Verify that the custom targets are reflected in the rules
	burnRateGroup := rules.Groups[1] // SLO burn rate group
	assert.Equal(t, "cloudflare-dns-operator.slo.burn_rate", burnRateGroup.Name)

	found := false
	for _, rule := range burnRateGroup.Rules {
		if strings.Contains(rule.Record, "availability") {
			// Should reference the custom availability target (0.999)
			assert.Contains(t, rule.Labels["target"], "0.999")
			found = true
			break
		}
	}
	assert.True(t, found, "Should find availability rule with custom target")
}

// Integration test
func TestRecordingRulesIntegration(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	// Generate rules
	rules := generator.GenerateRecordingRules()
	assert.NotNil(t, rules)

	// Validate rules
	issues := generator.ValidateRecordingRules()
	assert.Empty(t, issues)

	// Generate YAML
	yamlOutput, err := generator.GenerateYAML()
	assert.NoError(t, err)
	assert.NotEmpty(t, yamlOutput)

	// Generate CRD
	crdYAML, err := generator.GeneratePrometheusRuleCRD("integration-test", "default")
	assert.NoError(t, err)
	assert.NotEmpty(t, crdYAML)

	// Get rule names
	names := generator.GetRecordingRuleNames()
	assert.NotEmpty(t, names)

	t.Logf("Generated %d recording rules across %d groups", len(names), len(rules.Groups))
}

func TestGenerateYAMLError(t *testing.T) {
	// Create a config that might cause issues
	config := &Config{
		Enabled: true,
		Targets: SLOTargets{
			Availability:  99.9,
			SuccessRate:   99.5,
			LatencyP95:    30 * time.Second,
			ThroughputMin: 10.0,
		},
		Windows: []TimeWindow{}, // Empty windows to test edge case
	}

	generator := NewRecordingRulesGenerator(config)

	// Should still work with empty windows
	yamlOutput, err := generator.GenerateYAML()
	assert.NoError(t, err)
	assert.NotEmpty(t, yamlOutput)
}

func TestValidateRecordingRulesEdgeCases(t *testing.T) {
	// Test with valid configuration
	validConfig := DefaultConfig()
	validGenerator := NewRecordingRulesGenerator(validConfig)

	validIssues := validGenerator.ValidateRecordingRules()

	// Should have no issues with valid config
	assert.Empty(t, validIssues)

	// Test that empty windows produces rules but without window-specific rules
	generator := &RecordingRulesGenerator{
		config: &Config{
			Windows: []TimeWindow{}, // Empty windows
		},
	}

	rules := generator.GenerateRecordingRules()

	// Should still have groups (error budget and multi-window)
	// but no window-specific rules in the windows group
	assert.NotEmpty(t, rules.Groups)

	// Find the windows group
	var windowsGroup *RecordingRuleGroup
	for i := range rules.Groups {
		if rules.Groups[i].Name == "cloudflare-dns-operator.slo.windows" {
			windowsGroup = &rules.Groups[i]
			break
		}
	}

	// Windows group should have no rules if no windows configured
	if windowsGroup != nil {
		assert.Empty(t, windowsGroup.Rules)
	}
}

func TestGeneratePrometheusRuleCRDEdgeCases(t *testing.T) {
	config := DefaultConfig()
	generator := NewRecordingRulesGenerator(config)

	t.Run("empty name", func(t *testing.T) {
		crdYAML, err := generator.GeneratePrometheusRuleCRD("", "default")
		assert.NoError(t, err)
		assert.NotEmpty(t, crdYAML)
		assert.Contains(t, crdYAML, `name: ""`)
	})

	t.Run("empty namespace", func(t *testing.T) {
		crdYAML, err := generator.GeneratePrometheusRuleCRD("test", "")
		assert.NoError(t, err)
		assert.NotEmpty(t, crdYAML)
		assert.Contains(t, crdYAML, `namespace: ""`)
	})

	t.Run("special characters in name", func(t *testing.T) {
		crdYAML, err := generator.GeneratePrometheusRuleCRD("test-rule_123", "monitoring-ns")
		assert.NoError(t, err)
		assert.NotEmpty(t, crdYAML)
		assert.Contains(t, crdYAML, "test-rule_123")
		assert.Contains(t, crdYAML, "monitoring-ns")
	})
}
