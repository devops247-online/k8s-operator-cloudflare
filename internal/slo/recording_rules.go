package slo

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RecordingRuleGroup represents a Prometheus recording rule group
type RecordingRuleGroup struct {
	Name     string          `yaml:"name"`
	Interval string          `yaml:"interval,omitempty"`
	Rules    []RecordingRule `yaml:"rules"`
}

// RecordingRule represents a single Prometheus recording rule
type RecordingRule struct {
	Record string            `yaml:"record"`
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// PrometheusRulesSpec represents the structure expected by PrometheusRule CRD
type PrometheusRulesSpec struct {
	Groups []RecordingRuleGroup `yaml:"groups"`
}

// RecordingRulesGenerator generates Prometheus recording rules for SLO monitoring
type RecordingRulesGenerator struct {
	config *Config
}

// NewRecordingRulesGenerator creates a new recording rules generator
func NewRecordingRulesGenerator(config *Config) *RecordingRulesGenerator {
	return &RecordingRulesGenerator{
		config: config,
	}
}

// GenerateRecordingRules generates all recording rules for SLO monitoring
func (g *RecordingRulesGenerator) GenerateRecordingRules() *PrometheusRulesSpec {
	return &PrometheusRulesSpec{
		Groups: []RecordingRuleGroup{
			g.generateSLIRules(),
			g.generateSLOBurnRateRules(),
			g.generateErrorBudgetRules(),
			g.generateMultiWindowRules(),
		},
	}
}

// generateSLIRules generates SLI recording rules
func (g *RecordingRulesGenerator) generateSLIRules() RecordingRuleGroup {
	rules := []RecordingRule{
		// Availability SLI
		{
			Record: "sli:availability:rate5m",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code!~"5.."}[5m])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[5m]))
			)`,
			Labels: map[string]string{
				"sli_type": "availability",
				"window":   "5m",
			},
		},
		{
			Record: "sli:availability:rate30m",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code!~"5.."}[30m])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[30m]))
			)`,
			Labels: map[string]string{
				"sli_type": "availability",
				"window":   "30m",
			},
		},
		{
			Record: "sli:availability:rate1h",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code!~"5.."}[1h])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[1h]))
			)`,
			Labels: map[string]string{
				"sli_type": "availability",
				"window":   "1h",
			},
		},
		// Success Rate SLI (includes client errors as failures)
		{
			Record: "sli:success_rate:rate5m",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code=~"2.."}[5m])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[5m]))
			)`,
			Labels: map[string]string{
				"sli_type": "success_rate",
				"window":   "5m",
			},
		},
		{
			Record: "sli:success_rate:rate30m",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code=~"2.."}[30m])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[30m]))
			)`,
			Labels: map[string]string{
				"sli_type": "success_rate",
				"window":   "30m",
			},
		},
		{
			Record: "sli:success_rate:rate1h",
			Expr: `(
				sum(rate(http_requests_total{job="cloudflare-dns-operator",code=~"2.."}[1h])) /
				sum(rate(http_requests_total{job="cloudflare-dns-operator"}[1h]))
			)`,
			Labels: map[string]string{
				"sli_type": "success_rate",
				"window":   "1h",
			},
		},
		// Latency SLI (P95)
		{
			Record: "sli:latency_p95:5m",
			Expr: `histogram_quantile(0.95,
				sum(rate(http_request_duration_seconds_bucket{job="cloudflare-dns-operator"}[5m])) by (le)
			)`,
			Labels: map[string]string{
				"sli_type": "latency_p95",
				"window":   "5m",
			},
		},
		{
			Record: "sli:latency_p95:30m",
			Expr: `histogram_quantile(0.95,
				sum(rate(http_request_duration_seconds_bucket{job="cloudflare-dns-operator"}[30m])) by (le)
			)`,
			Labels: map[string]string{
				"sli_type": "latency_p95",
				"window":   "30m",
			},
		},
		{
			Record: "sli:latency_p95:1h",
			Expr: `histogram_quantile(0.95,
				sum(rate(http_request_duration_seconds_bucket{job="cloudflare-dns-operator"}[1h])) by (le)
			)`,
			Labels: map[string]string{
				"sli_type": "latency_p95",
				"window":   "1h",
			},
		},
		// Throughput SLI
		{
			Record: "sli:throughput:rate5m",
			Expr:   `sum(rate(http_requests_total{job="cloudflare-dns-operator"}[5m]))`,
			Labels: map[string]string{
				"sli_type": "throughput",
				"window":   "5m",
			},
		},
		{
			Record: "sli:throughput:rate30m",
			Expr:   `sum(rate(http_requests_total{job="cloudflare-dns-operator"}[30m]))`,
			Labels: map[string]string{
				"sli_type": "throughput",
				"window":   "30m",
			},
		},
		{
			Record: "sli:throughput:rate1h",
			Expr:   `sum(rate(http_requests_total{job="cloudflare-dns-operator"}[1h]))`,
			Labels: map[string]string{
				"sli_type": "throughput",
				"window":   "1h",
			},
		},
	}

	return RecordingRuleGroup{
		Name:     "cloudflare-dns-operator.sli",
		Interval: "30s",
		Rules:    rules,
	}
}

// generateSLOBurnRateRules generates SLO burn rate recording rules
func (g *RecordingRulesGenerator) generateSLOBurnRateRules() RecordingRuleGroup {
	rules := []RecordingRule{}

	// Generate burn rate rules for each SLO target
	sloTargets := map[string]float64{
		"availability": g.config.Targets.Availability / 100.0,
		"success_rate": g.config.Targets.SuccessRate / 100.0,
		"latency_p95":  g.config.Targets.LatencyP95.Seconds(),
		"throughput":   g.config.Targets.ThroughputMin,
	}

	windows := []string{"5m", "30m", "1h", "6h", "24h"}

	for sloName, target := range sloTargets {
		for _, window := range windows {
			switch sloName {
			case "latency_p95":
				// For latency, we calculate the percentage of requests that meet the SLO
				rule := RecordingRule{
					Record: fmt.Sprintf("slo:burn_rate:%s:%s", sloName, window),
					Expr: fmt.Sprintf(`(
						1 - (
							sum(rate(http_request_duration_seconds_bucket{job="cloudflare-dns-operator",le="%.3f"}[%s])) /
							sum(rate(http_request_duration_seconds_bucket{job="cloudflare-dns-operator",le="+Inf"}[%s]))
						)
					) / (1 - 0.95)`, target, window, window),
					Labels: map[string]string{
						"slo_type": sloName,
						"window":   window,
						"target":   fmt.Sprintf("%.3f", target),
					},
				}
				rules = append(rules, rule)
			case "throughput":
				// For throughput, we check if we're meeting the minimum requirement
				rule := RecordingRule{
					Record: fmt.Sprintf("slo:burn_rate:%s:%s", sloName, window),
					Expr: fmt.Sprintf(`(
						clamp_max(1 - (sli:throughput:rate%s / %.2f), 1)
					)`, window, target),
					Labels: map[string]string{
						"slo_type": sloName,
						"window":   window,
						"target":   fmt.Sprintf("%.2f", target),
					},
				}
				rules = append(rules, rule)
			default:
				// For availability and success rate
				rule := RecordingRule{
					Record: fmt.Sprintf("slo:burn_rate:%s:%s", sloName, window),
					Expr: fmt.Sprintf(`(
						1 - (sli:%s:rate%s / %.4f)
					) / (1 - %.4f)`, sloName, window, target, target),
					Labels: map[string]string{
						"slo_type": sloName,
						"window":   window,
						"target":   fmt.Sprintf("%.4f", target),
					},
				}
				rules = append(rules, rule)
			}
		}
	}

	return RecordingRuleGroup{
		Name:     "cloudflare-dns-operator.slo.burn_rate",
		Interval: "30s",
		Rules:    rules,
	}
}

// generateErrorBudgetRules generates error budget recording rules
func (g *RecordingRulesGenerator) generateErrorBudgetRules() RecordingRuleGroup {
	rules := []RecordingRule{
		// Error budget remaining (30 days)
		{
			Record: "slo:error_budget_remaining:availability:30d",
			Expr: fmt.Sprintf(`(
				1 - (
					(1 - avg_over_time(sli:availability:rate1h[30d])) / (1 - %.4f)
				)
			)`, g.config.Targets.Availability/100.0),
			Labels: map[string]string{
				"slo_type":    "availability",
				"window":      "30d",
				"target":      fmt.Sprintf("%.4f", g.config.Targets.Availability/100.0),
				"budget_type": "remaining",
			},
		},
		{
			Record: "slo:error_budget_remaining:success_rate:30d",
			Expr: fmt.Sprintf(`(
				1 - (
					(1 - avg_over_time(sli:success_rate:rate1h[30d])) / (1 - %.4f)
				)
			)`, g.config.Targets.SuccessRate/100.0),
			Labels: map[string]string{
				"slo_type":    "success_rate",
				"window":      "30d",
				"target":      fmt.Sprintf("%.4f", g.config.Targets.SuccessRate/100.0),
				"budget_type": "remaining",
			},
		},
		// Error budget burn rate over different periods
		{
			Record: "slo:error_budget_burn_rate:availability:1h",
			Expr: `(
				slo:burn_rate:availability:1h * 24 * 30
			)`,
			Labels: map[string]string{
				"slo_type":    "availability",
				"window":      "1h",
				"budget_type": "burn_rate",
			},
		},
		{
			Record: "slo:error_budget_burn_rate:availability:6h",
			Expr: `(
				slo:burn_rate:availability:6h * 4 * 30
			)`,
			Labels: map[string]string{
				"slo_type":    "availability",
				"window":      "6h",
				"budget_type": "burn_rate",
			},
		},
		{
			Record: "slo:error_budget_burn_rate:success_rate:1h",
			Expr: `(
				slo:burn_rate:success_rate:1h * 24 * 30
			)`,
			Labels: map[string]string{
				"slo_type":    "success_rate",
				"window":      "1h",
				"budget_type": "burn_rate",
			},
		},
		{
			Record: "slo:error_budget_burn_rate:success_rate:6h",
			Expr: `(
				slo:burn_rate:success_rate:6h * 4 * 30
			)`,
			Labels: map[string]string{
				"slo_type":    "success_rate",
				"window":      "6h",
				"budget_type": "burn_rate",
			},
		},
	}

	return RecordingRuleGroup{
		Name:     "cloudflare-dns-operator.slo.error_budget",
		Interval: "1m",
		Rules:    rules,
	}
}

// generateMultiWindowRules generates multi-window burn rate rules for alerting
func (g *RecordingRulesGenerator) generateMultiWindowRules() RecordingRuleGroup {
	rules := []RecordingRule{}

	// Multi-window burn rate rules for page alerts (fast burn)
	pageAlerts := []struct {
		shortWindow string
		longWindow  string
		threshold   float64
	}{
		{"5m", "1h", 14.4}, // 2% budget in 1 hour
		{"30m", "6h", 6.0}, // 5% budget in 6 hours
	}

	// Multi-window burn rate rules for ticket alerts (slow burn)
	ticketAlerts := []struct {
		shortWindow string
		longWindow  string
		threshold   float64
	}{
		{"2h", "1d", 3.0}, // 10% budget in 1 day
		{"6h", "3d", 1.0}, // 25% budget in 3 days
	}

	sloTypes := []string{"availability", "success_rate"}

	// Generate page alert rules
	for _, alert := range pageAlerts {
		for _, sloType := range sloTypes {
			rule := RecordingRule{
				Record: fmt.Sprintf("slo:multi_window_burn_rate:%s:%s_%s", sloType, alert.shortWindow, alert.longWindow),
				Expr: fmt.Sprintf(`(
					slo:burn_rate:%s:%s > (%.1f * 0.25)
					and
					slo:burn_rate:%s:%s > (%.1f * 0.25)
				)`, sloType, alert.shortWindow, alert.threshold, sloType, alert.longWindow, alert.threshold),
				Labels: map[string]string{
					"slo_type":     sloType,
					"short_window": alert.shortWindow,
					"long_window":  alert.longWindow,
					"severity":     "page",
					"threshold":    fmt.Sprintf("%.1f", alert.threshold),
				},
			}
			rules = append(rules, rule)
		}
	}

	// Generate ticket alert rules
	for _, alert := range ticketAlerts {
		for _, sloType := range sloTypes {
			rule := RecordingRule{
				Record: fmt.Sprintf("slo:multi_window_burn_rate:%s:%s_%s", sloType, alert.shortWindow, alert.longWindow),
				Expr: fmt.Sprintf(`(
					slo:burn_rate:%s:%s > (%.1f * 0.1)
					and
					slo:burn_rate:%s:%s > (%.1f * 0.1)
				)`, sloType, alert.shortWindow, alert.threshold, sloType, alert.longWindow, alert.threshold),
				Labels: map[string]string{
					"slo_type":     sloType,
					"short_window": alert.shortWindow,
					"long_window":  alert.longWindow,
					"severity":     "ticket",
					"threshold":    fmt.Sprintf("%.1f", alert.threshold),
				},
			}
			rules = append(rules, rule)
		}
	}

	return RecordingRuleGroup{
		Name:     "cloudflare-dns-operator.slo.multi_window",
		Interval: "30s",
		Rules:    rules,
	}
}

// GenerateYAML generates YAML output for the recording rules
func (g *RecordingRulesGenerator) GenerateYAML() (string, error) {
	rules := g.GenerateRecordingRules()

	yamlData, err := yaml.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recording rules to YAML: %w", err)
	}

	return string(yamlData), nil
}

// GeneratePrometheusRuleCRD generates a PrometheusRule CRD YAML
func (g *RecordingRulesGenerator) GeneratePrometheusRuleCRD(name, namespace string) (string, error) {
	rules := g.GenerateRecordingRules()

	crd := map[string]interface{}{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "PrometheusRule",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				"app.kubernetes.io/name":      "cloudflare-dns-operator",
				"app.kubernetes.io/component": "slo-monitoring",
				"prometheus":                  "kube-prometheus",
				"role":                        "alert-rules",
			},
		},
		"spec": rules,
	}

	yamlData, err := yaml.Marshal(crd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PrometheusRule CRD to YAML: %w", err)
	}

	return string(yamlData), nil
}

// ValidateRecordingRules validates the generated recording rules
func (g *RecordingRulesGenerator) ValidateRecordingRules() []string {
	var issues []string
	rules := g.GenerateRecordingRules()

	for _, group := range rules.Groups {
		// Validate group name
		if group.Name == "" {
			issues = append(issues, "empty group name found")
		}

		// Validate rules
		for _, rule := range group.Rules {
			// Check for empty record name
			if rule.Record == "" {
				issues = append(issues, fmt.Sprintf("empty record name in group %s", group.Name))
			}

			// Check for empty expression
			if rule.Expr == "" {
				issues = append(issues, fmt.Sprintf("empty expression for rule %s", rule.Record))
			}

			// Check for valid metric naming
			if !strings.Contains(rule.Record, ":") {
				issues = append(issues, fmt.Sprintf("rule %s doesn't follow naming convention (missing colon)", rule.Record))
			}

			// Check for reasonable interval
			if group.Interval != "" {
				if interval, err := time.ParseDuration(group.Interval); err != nil {
					issues = append(issues, fmt.Sprintf("invalid interval %s in group %s", group.Interval, group.Name))
				} else if interval < 10*time.Second {
					issues = append(issues, fmt.Sprintf("interval %s in group %s is too short", group.Interval, group.Name))
				}
			}
		}
	}

	return issues
}

// GetRecordingRuleNames returns all recording rule names for testing
func (g *RecordingRulesGenerator) GetRecordingRuleNames() []string {
	var names []string
	rules := g.GenerateRecordingRules()

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			names = append(names, rule.Record)
		}
	}

	return names
}
