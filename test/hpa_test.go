package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/engine"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestHPATemplate(t *testing.T) {
	chartPath := filepath.Join("..", "charts", "cloudflare-dns-operator")

	// Load the chart
	chart, err := loader.Load(chartPath)
	assert.NoError(t, err)

	// Test values with HPA enabled - include necessary defaults
	values := map[string]interface{}{
		"nameOverride":     "",
		"fullnameOverride": "",
		"crds": map[string]interface{}{
			"install": true,
		},
		"horizontalPodAutoscaler": map[string]interface{}{
			"enabled":                           true,
			"minReplicas":                       2,
			"maxReplicas":                       20,
			"targetCPUUtilizationPercentage":    70,
			"targetMemoryUtilizationPercentage": 80,
			"customMetrics": map[string]interface{}{
				"enabled": true,
				"metrics": map[string]interface{}{
					"queueDepth": map[string]interface{}{
						"enabled":     true,
						"targetValue": 15,
					},
					"reconcileRate": map[string]interface{}{
						"enabled":     true,
						"targetValue": 8,
					},
					"errorRate": map[string]interface{}{
						"enabled":     true,
						"targetValue": 3,
					},
				},
			},
			"behavior": map[string]interface{}{
				"scaleUp": map[string]interface{}{
					"stabilizationWindowSeconds": 120,
					"selectPolicy":               "Max",
					"policies": []map[string]interface{}{
						{
							"type":          "Percent",
							"value":         50,
							"periodSeconds": 60,
						},
					},
				},
				"scaleDown": map[string]interface{}{
					"stabilizationWindowSeconds": 300,
					"selectPolicy":               "Min",
					"policies": []map[string]interface{}{
						{
							"type":          "Pods",
							"value":         1,
							"periodSeconds": 60,
						},
					},
				},
			},
		},
	}

	// Render the template
	engine := engine.Engine{}
	renderedTemplates, err := engine.Render(chart, values)
	assert.NoError(t, err)

	// Find the HPA template
	hpaTemplate, exists := renderedTemplates["cloudflare-dns-operator/templates/hpa.yaml"]
	assert.True(t, exists, "HPA template should exist when enabled")

	// Parse the HPA manifest
	var hpa autoscalingv2.HorizontalPodAutoscaler
	err = yaml.Unmarshal([]byte(hpaTemplate), &hpa)
	assert.NoError(t, err)

	// Verify HPA configuration
	assert.Equal(t, "cloudflare-dns-operator", hpa.Name)
	assert.Equal(t, int32(2), hpa.Spec.MinReplicas)
	assert.Equal(t, int32(20), hpa.Spec.MaxReplicas)

	// Verify target reference
	assert.Equal(t, "apps/v1", hpa.Spec.ScaleTargetRef.APIVersion)
	assert.Equal(t, "Deployment", hpa.Spec.ScaleTargetRef.Kind)
	assert.Equal(t, "cloudflare-dns-operator", hpa.Spec.ScaleTargetRef.Name)

	// Verify metrics
	assert.Len(t, hpa.Spec.Metrics, 5) // CPU, Memory, and 3 custom metrics

	// Check CPU metric
	cpuMetric := hpa.Spec.Metrics[0]
	assert.Equal(t, autoscalingv2.ResourceMetricSourceType, cpuMetric.Type)
	assert.Equal(t, "cpu", cpuMetric.Resource.Name)
	assert.Equal(t, int32(70), *cpuMetric.Resource.Target.AverageUtilization)

	// Check Memory metric
	memoryMetric := hpa.Spec.Metrics[1]
	assert.Equal(t, autoscalingv2.ResourceMetricSourceType, memoryMetric.Type)
	assert.Equal(t, "memory", memoryMetric.Resource.Name)
	assert.Equal(t, int32(80), *memoryMetric.Resource.Target.AverageUtilization)

	// Check custom metrics
	queueDepthMetric := hpa.Spec.Metrics[2]
	assert.Equal(t, autoscalingv2.PodsMetricSourceType, queueDepthMetric.Type)
	assert.Equal(t, "cloudflare_operator_queue_depth", queueDepthMetric.Pods.Metric.Name)

	// Verify behavior configuration
	assert.NotNil(t, hpa.Spec.Behavior)
	assert.NotNil(t, hpa.Spec.Behavior.ScaleUp)
	assert.NotNil(t, hpa.Spec.Behavior.ScaleDown)

	assert.Equal(t, int32(120), *hpa.Spec.Behavior.ScaleUp.StabilizationWindowSeconds)
	assert.Equal(t, autoscalingv2.MaxChangePolicySelect, *hpa.Spec.Behavior.ScaleUp.SelectPolicy)

	assert.Equal(t, int32(300), *hpa.Spec.Behavior.ScaleDown.StabilizationWindowSeconds)
	assert.Equal(t, autoscalingv2.MinChangePolicySelect, *hpa.Spec.Behavior.ScaleDown.SelectPolicy)
}

func TestHPATemplateDisabled(t *testing.T) {
	chartPath := filepath.Join("..", "charts", "cloudflare-dns-operator")

	// Load the chart
	chart, err := loader.Load(chartPath)
	assert.NoError(t, err)

	// Test values with HPA disabled (default)
	values := map[string]interface{}{
		"horizontalPodAutoscaler": map[string]interface{}{
			"enabled": false,
		},
	}

	// Render the template
	engine := engine.Engine{}
	renderedTemplates, err := engine.Render(chart, values)
	assert.NoError(t, err)

	// HPA template should be empty when disabled
	hpaTemplate, exists := renderedTemplates["cloudflare-dns-operator/templates/hpa.yaml"]
	if exists {
		// Template exists but should be empty
		assert.Empty(t, hpaTemplate, "HPA template should be empty when disabled")
	}
}

func TestHPATemplateMinimalConfig(t *testing.T) {
	chartPath := filepath.Join("..", "charts", "cloudflare-dns-operator")

	// Load the chart
	chart, err := loader.Load(chartPath)
	assert.NoError(t, err)

	// Test values with minimal HPA configuration
	values := map[string]interface{}{
		"horizontalPodAutoscaler": map[string]interface{}{
			"enabled":                           true,
			"minReplicas":                       1,
			"maxReplicas":                       5,
			"targetCPUUtilizationPercentage":    nil, // Test without CPU metric
			"targetMemoryUtilizationPercentage": 85,
			"customMetrics": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	// Render the template
	engine := engine.Engine{}
	renderedTemplates, err := engine.Render(chart, values)
	assert.NoError(t, err)

	// Find the HPA template
	hpaTemplate, exists := renderedTemplates["cloudflare-dns-operator/templates/hpa.yaml"]
	assert.True(t, exists, "HPA template should exist when enabled")

	// Parse the HPA manifest
	var hpa autoscalingv2.HorizontalPodAutoscaler
	err = yaml.Unmarshal([]byte(hpaTemplate), &hpa)
	assert.NoError(t, err)

	// Verify minimal configuration
	assert.Equal(t, int32(1), hpa.Spec.MinReplicas)
	assert.Equal(t, int32(5), hpa.Spec.MaxReplicas)

	// Should only have memory metric, no CPU or custom metrics
	assert.Len(t, hpa.Spec.Metrics, 1)

	memoryMetric := hpa.Spec.Metrics[0]
	assert.Equal(t, autoscalingv2.ResourceMetricSourceType, memoryMetric.Type)
	assert.Equal(t, "memory", memoryMetric.Resource.Name)
	assert.Equal(t, int32(85), *memoryMetric.Resource.Target.AverageUtilization)
}

func TestPerformanceConfigurationTemplate(t *testing.T) {
	chartPath := filepath.Join("..", "charts", "cloudflare-dns-operator")

	// Load the chart
	chart, err := loader.Load(chartPath)
	assert.NoError(t, err)

	// Test values with performance configuration
	values := map[string]interface{}{
		"performance": map[string]interface{}{
			"controller": map[string]interface{}{
				"maxConcurrentReconciles": 10,
				"reconcileTimeout":        600,
				"requeueInterval":         600,
				"requeueIntervalOnError":  120,
				"rateLimiter": map[string]interface{}{
					"baseDelay":        10,
					"maxDelay":         600,
					"failureThreshold": 10,
					"bucketSize":       200,
					"fillRate":         20,
				},
				"cache": map[string]interface{}{
					"syncTimeout":    240,
					"resyncPeriod":   1200,
					"metricsEnabled": true,
				},
				"batch": map[string]interface{}{
					"enabled": true,
					"size":    20,
					"timeout": 2000,
					"maxWait": 10000,
				},
			},
			"resources": map[string]interface{}{
				"cpu": map[string]interface{}{
					"profilingEnabled":  true,
					"requestMultiplier": 1.2,
					"limitMultiplier":   2.0,
				},
				"memory": map[string]interface{}{
					"profilingEnabled":  true,
					"requestMultiplier": 1.5,
					"limitMultiplier":   3.0,
					"gcTuning": map[string]interface{}{
						"enabled":  true,
						"gogc":     80,
						"memlimit": 85,
					},
				},
			},
			"monitoring": map[string]interface{}{
				"metricsEnabled":  true,
				"metricsInterval": 15,
				"customMetrics": map[string]interface{}{
					"enabled":        true,
					"exportInterval": 10,
				},
			},
		},
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "100m",
				"memory": "64Mi",
			},
		},
	}

	// Render the template
	engine := engine.Engine{}
	renderedTemplates, err := engine.Render(chart, values)
	assert.NoError(t, err)

	// Find the deployment template
	deploymentTemplate, exists := renderedTemplates["cloudflare-dns-operator/templates/deployment.yaml"]
	assert.True(t, exists, "Deployment template should exist")

	// Parse the deployment manifest
	var deployment appsv1.Deployment
	err = yaml.Unmarshal([]byte(deploymentTemplate), &deployment)
	assert.NoError(t, err)

	// Verify environment variables are set
	container := deployment.Spec.Template.Spec.Containers[0]
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	// Check performance environment variables
	assert.Equal(t, "10", envMap["MAX_CONCURRENT_RECONCILES"])
	assert.Equal(t, "600s", envMap["RECONCILE_TIMEOUT"])
	assert.Equal(t, "600s", envMap["REQUEUE_INTERVAL"])
	assert.Equal(t, "120s", envMap["REQUEUE_INTERVAL_ON_ERROR"])

	// Check rate limiter configuration
	assert.Equal(t, "10ms", envMap["RATE_LIMITER_BASE_DELAY"])
	assert.Equal(t, "600s", envMap["RATE_LIMITER_MAX_DELAY"])
	assert.Equal(t, "10", envMap["RATE_LIMITER_FAILURE_THRESHOLD"])
	assert.Equal(t, "200", envMap["RATE_LIMITER_BUCKET_SIZE"])
	assert.Equal(t, "20", envMap["RATE_LIMITER_FILL_RATE"])

	// Check cache configuration
	assert.Equal(t, "240s", envMap["CACHE_SYNC_TIMEOUT"])
	assert.Equal(t, "1200s", envMap["CACHE_RESYNC_PERIOD"])
	assert.Equal(t, "true", envMap["CACHE_METRICS_ENABLED"])

	// Check batch processing
	assert.Equal(t, "true", envMap["BATCH_PROCESSING_ENABLED"])
	assert.Equal(t, "20", envMap["BATCH_SIZE"])
	assert.Equal(t, "2000ms", envMap["BATCH_TIMEOUT"])
	assert.Equal(t, "10000ms", envMap["BATCH_MAX_WAIT"])

	// Check resource profiling
	assert.Equal(t, "true", envMap["CPU_PROFILING_ENABLED"])
	assert.Equal(t, "true", envMap["MEMORY_PROFILING_ENABLED"])

	// Check GC tuning
	assert.Equal(t, "80", envMap["GOGC"])
	assert.Equal(t, "85%", envMap["GOMEMLIMIT"])

	// Check monitoring
	assert.Equal(t, "true", envMap["PERFORMANCE_METRICS_ENABLED"])
	assert.Equal(t, "15s", envMap["METRICS_INTERVAL"])
	assert.Equal(t, "true", envMap["CUSTOM_METRICS_ENABLED"])
	assert.Equal(t, "10s", envMap["CUSTOM_METRICS_EXPORT_INTERVAL"])
}
