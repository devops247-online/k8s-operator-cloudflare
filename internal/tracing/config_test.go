package tracing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, "k8s-operator-cloudflare", config.ServiceName)
	assert.Equal(t, "0.1.0", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "console", config.Exporter.Type)
	assert.True(t, config.Exporter.Insecure)
	assert.Equal(t, 10*time.Second, config.Exporter.Timeout)
	assert.Equal(t, "parent_based", config.Sampling.Type)
	assert.Equal(t, 1.0, config.Sampling.Ratio)
	assert.NotNil(t, config.ResourceAttributes)
}

func TestNewEnvironmentConfig(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantEnabled bool
		wantType    string
		wantRatio   float64
	}{
		{
			name:        "development environment",
			environment: "development",
			wantEnabled: true,
			wantType:    "console",
			wantRatio:   1.0,
		},
		{
			name:        "dev environment",
			environment: "dev",
			wantEnabled: true,
			wantType:    "console",
			wantRatio:   1.0,
		},
		{
			name:        "staging environment",
			environment: "staging",
			wantEnabled: true,
			wantType:    "otlp",
			wantRatio:   0.5,
		},
		{
			name:        "production environment",
			environment: "production",
			wantEnabled: true,
			wantType:    "otlp",
			wantRatio:   0.1,
		},
		{
			name:        "prod environment",
			environment: "prod",
			wantEnabled: true,
			wantType:    "otlp",
			wantRatio:   0.1,
		},
		{
			name:        "unknown environment",
			environment: "unknown",
			wantEnabled: false,
			wantType:    "console",
			wantRatio:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewEnvironmentConfig(tt.environment)
			assert.Equal(t, tt.wantEnabled, config.Enabled)
			assert.Equal(t, tt.wantType, config.Exporter.Type)
			assert.Equal(t, tt.wantRatio, config.Sampling.Ratio)
			assert.Equal(t, tt.environment, config.Environment)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid disabled config",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid enabled config",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type:    "console",
					Timeout: 10 * time.Second,
				},
				Sampling: SamplingConfig{
					Type:  "always",
					Ratio: 1.0,
				},
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			config: Config{
				Enabled:     true,
				ServiceName: "",
			},
			wantErr: true,
		},
		{
			name: "invalid exporter",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid sampling",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type:    "console",
					Timeout: 10 * time.Second,
				},
				Sampling: SamplingConfig{
					Type: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigIsEnabled(t *testing.T) {
	config := Config{Enabled: true}
	assert.True(t, config.IsEnabled())

	config.Enabled = false
	assert.False(t, config.IsEnabled())
}

func TestConfigGetServiceName(t *testing.T) {
	config := Config{ServiceName: "custom-service"}
	assert.Equal(t, "custom-service", config.GetServiceName())

	config.ServiceName = ""
	assert.Equal(t, "k8s-operator-cloudflare", config.GetServiceName())
}

func TestConfigGetServiceVersion(t *testing.T) {
	config := Config{ServiceVersion: "1.2.3"}
	assert.Equal(t, "1.2.3", config.GetServiceVersion())

	config.ServiceVersion = ""
	assert.Equal(t, "unknown", config.GetServiceVersion())
}

func TestConfigString(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Exporter: ExporterConfig{
			Type: "console",
		},
	}

	str := config.String()
	assert.Contains(t, str, "enabled=true")
	assert.Contains(t, str, "service=test-service")
	assert.Contains(t, str, "version=1.0.0")
	assert.Contains(t, str, "environment=test")
	assert.Contains(t, str, "exporter=console")
}

func TestExporterConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		exporter ExporterConfig
		wantErr  bool
	}{
		{
			name: "valid console exporter",
			exporter: ExporterConfig{
				Type:    "console",
				Timeout: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid otlp exporter",
			exporter: ExporterConfig{
				Type:     "otlp",
				Endpoint: "http://localhost:4318/v1/traces",
				Timeout:  10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid jaeger exporter",
			exporter: ExporterConfig{
				Type:     "jaeger",
				Endpoint: "http://localhost:14268/api/traces",
				Timeout:  10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid none exporter",
			exporter: ExporterConfig{
				Type:    "none",
				Timeout: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid exporter type",
			exporter: ExporterConfig{
				Type: "invalid",
			},
			wantErr: true,
		},
		{
			name: "otlp without endpoint",
			exporter: ExporterConfig{
				Type: "otlp",
			},
			wantErr: true,
		},
		{
			name: "jaeger without endpoint",
			exporter: ExporterConfig{
				Type: "jaeger",
			},
			wantErr: true,
		},
		{
			name: "invalid compression",
			exporter: ExporterConfig{
				Type:        "console",
				Compression: "invalid",
				Timeout:     10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "valid gzip compression",
			exporter: ExporterConfig{
				Type:        "console",
				Compression: "gzip",
				Timeout:     10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero timeout gets default",
			exporter: ExporterConfig{
				Type:    "console",
				Timeout: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.exporter.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.exporter.Timeout == 0 {
					// Should have been set to default
					assert.Equal(t, 10*time.Second, tt.exporter.Timeout)
				}
			}
		})
	}
}

func TestSamplingConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		sampling SamplingConfig
		wantErr  bool
	}{
		{
			name: "valid always sampling",
			sampling: SamplingConfig{
				Type: "always",
			},
			wantErr: false,
		},
		{
			name: "valid never sampling",
			sampling: SamplingConfig{
				Type: "never",
			},
			wantErr: false,
		},
		{
			name: "valid trace_id_ratio sampling",
			sampling: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: 0.5,
			},
			wantErr: false,
		},
		{
			name: "valid parent_based sampling",
			sampling: SamplingConfig{
				Type: "parent_based",
			},
			wantErr: false,
		},
		{
			name: "invalid sampling type",
			sampling: SamplingConfig{
				Type: "invalid",
			},
			wantErr: true,
		},
		{
			name: "trace_id_ratio with negative ratio",
			sampling: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: -0.1,
			},
			wantErr: true,
		},
		{
			name: "trace_id_ratio with ratio > 1.0",
			sampling: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: 1.1,
			},
			wantErr: true,
		},
		{
			name: "trace_id_ratio with ratio = 0.0",
			sampling: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: 0.0,
			},
			wantErr: false,
		},
		{
			name: "trace_id_ratio with ratio = 1.0",
			sampling: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: 1.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sampling.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigWithResourceAttributes(t *testing.T) {
	config := NewDefaultConfig()
	config.ResourceAttributes = map[string]string{
		"deployment.environment": "test",
		"service.namespace":      "default",
	}

	assert.Equal(t, "test", config.ResourceAttributes["deployment.environment"])
	assert.Equal(t, "default", config.ResourceAttributes["service.namespace"])
}

func TestExporterConfigWithHeaders(t *testing.T) {
	exporter := ExporterConfig{
		Type:     "otlp",
		Endpoint: "http://localhost:4318/v1/traces",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"X-API-Key":     "key456",
		},
		Timeout: 15 * time.Second,
	}

	err := exporter.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Bearer token123", exporter.Headers["Authorization"])
	assert.Equal(t, "key456", exporter.Headers["X-API-Key"])
}
