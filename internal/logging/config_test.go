package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantErr  bool
		wantJSON bool
	}{
		{
			name: "default config",
			config: Config{
				Level:       "info",
				Format:      "json",
				Development: false,
			},
			wantErr:  false,
			wantJSON: true,
		},
		{
			name: "console format",
			config: Config{
				Level:       "debug",
				Format:      "console",
				Development: true,
			},
			wantErr:  false,
			wantJSON: false,
		},
		{
			name: "invalid level",
			config: Config{
				Level:  "invalid",
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: Config{
				Level:  "info",
				Format: "invalid",
			},
			wantErr: true,
		},
		{
			name: "with sampling",
			config: Config{
				Level:  "info",
				Format: "json",
				Sampling: SamplingConfig{
					Enabled:    true,
					Initial:    100,
					Thereafter: 100,
				},
			},
			wantErr:  false,
			wantJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			level := tt.config.GetLevel()
			assert.NotEqual(t, zapcore.InvalidLevel, level)

			isJSON := tt.config.IsJSON()
			assert.Equal(t, tt.wantJSON, isJSON)
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	config := NewDefaultConfig()

	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.False(t, config.Development)
	assert.False(t, config.Sampling.Enabled)
	assert.Len(t, config.Outputs, 1)
	assert.Contains(t, config.Outputs, "stdout")
}

func TestConfigEnvironmentDefaults(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantLevel   string
		wantDev     bool
	}{
		{
			name:        "development environment",
			environment: "development",
			wantLevel:   "debug",
			wantDev:     true,
		},
		{
			name:        "production environment",
			environment: "production",
			wantLevel:   "info",
			wantDev:     false,
		},
		{
			name:        "unknown environment defaults to production",
			environment: "unknown",
			wantLevel:   "info",
			wantDev:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewEnvironmentConfig(tt.environment)
			assert.Equal(t, tt.wantLevel, config.Level)
			assert.Equal(t, tt.wantDev, config.Development)
		})
	}
}

func TestSamplingConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  SamplingConfig
		wantErr bool
	}{
		{
			name: "valid sampling config",
			config: SamplingConfig{
				Enabled:    true,
				Initial:    100,
				Thereafter: 100,
			},
			wantErr: false,
		},
		{
			name: "disabled sampling",
			config: SamplingConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "invalid initial value",
			config: SamplingConfig{
				Enabled:    true,
				Initial:    -1,
				Thereafter: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid thereafter value",
			config: SamplingConfig{
				Enabled:    true,
				Initial:    100,
				Thereafter: -1,
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

func TestConfigWithCustomOutputs(t *testing.T) {
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"stdout", "/var/log/operator.log"},
	}

	err := config.Validate()
	assert.NoError(t, err)
	assert.Len(t, config.Outputs, 2)
	assert.Contains(t, config.Outputs, "stdout")
	assert.Contains(t, config.Outputs, "/var/log/operator.log")
}

func TestConfigStringer(t *testing.T) {
	config := Config{
		Level:       "info",
		Format:      "json",
		Development: false,
	}

	str := config.String()
	assert.Contains(t, str, "level=info")
	assert.Contains(t, str, "format=json")
	assert.Contains(t, str, "development=false")
}
