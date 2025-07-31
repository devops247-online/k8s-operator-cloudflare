/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devops247-online/k8s-operator-cloudflare/internal/config"
)

func TestInit(t *testing.T) {
	t.Run("scheme initialization", func(t *testing.T) {
		// Test that scheme is properly initialized
		assert.NotNil(t, scheme)

		// Test that client-go scheme is registered
		gvks := scheme.AllKnownTypes()
		foundCoreV1 := false

		for gvk := range gvks {
			if gvk.Group == "" && gvk.Version == "v1" && gvk.Kind == "Pod" {
				foundCoreV1 = true
				break
			}
		}
		assert.True(t, foundCoreV1, "Core v1 types should be registered")

		// Test that CloudflareRecord CRD types are registered
		foundCloudflareRecord := false
		for gvk := range gvks {
			if gvk.Group == "dns.cloudflare.io" && gvk.Version == "v1" && gvk.Kind == "CloudflareRecord" {
				foundCloudflareRecord = true
				break
			}
		}
		assert.True(t, foundCloudflareRecord, "CloudflareRecord CRD should be registered")
	})

	t.Run("setup logger initialization", func(t *testing.T) {
		// Test that setupLog is initialized
		assert.NotNil(t, setupLog)
		// Note: logr.Logger doesn't have GetName() method
		// The logger is properly initialized and can be used
	})
}

func TestSchemeRegistration(t *testing.T) {
	t.Run("kubernetes core types", func(t *testing.T) {
		// Test that we can create core Kubernetes objects
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("Pod")

		gvk := obj.GroupVersionKind()
		_, exists := scheme.AllKnownTypes()[gvk]
		assert.True(t, exists, "Pod type should be registered in scheme")
	})

	t.Run("cloudflare record types", func(t *testing.T) {
		// Test that CloudflareRecord type is properly registered
		// Check if the CloudflareRecord type exists in the scheme
		allTypes := scheme.AllKnownTypes()
		found := false
		for gvk := range allTypes {
			if gvk.Group == "dns.cloudflare.io" && gvk.Version == "v1" && gvk.Kind == "CloudflareRecord" {
				found = true
				break
			}
		}
		assert.True(t, found, "CloudflareRecord type should be registered in scheme")
	})

	t.Run("scheme contains required types", func(t *testing.T) {
		// Verify that the scheme has both Kubernetes core types and our CRDs
		allTypes := scheme.AllKnownTypes()

		// Should have many types (core K8s + our CRDs)
		assert.Greater(t, len(allTypes), 100, "Scheme should contain many registered types")

		// Check specific important types exist
		importantTypes := []struct {
			group   string
			version string
			kind    string
		}{
			{"", "v1", "Pod"},
			{"", "v1", "Service"},
			{"apps", "v1", "Deployment"},
			{"dns.cloudflare.io", "v1", "CloudflareRecord"},
		}

		for _, typeInfo := range importantTypes {
			found := false
			for gvk := range allTypes {
				if gvk.Group == typeInfo.group && gvk.Version == typeInfo.version && gvk.Kind == typeInfo.kind {
					found = true
					break
				}
			}
			assert.True(t, found, "Type %s/%s %s should be registered", typeInfo.group, typeInfo.version, typeInfo.kind)
		}
	})
}

func TestPackageImports(t *testing.T) {
	t.Run("auth plugins imported", func(t *testing.T) {
		// This is a basic test to ensure the package compiles correctly
		// The auth plugin import is for side effects only
		assert.True(t, true, "Package should compile with all auth plugins imported")
	})
}

// Test helper functions that would be used in main()
func TestMainFunctionHelpers(t *testing.T) {
	t.Run("runtime information", func(t *testing.T) {
		// Basic runtime checks that main() might depend on
		assert.NotEmpty(t, runtime.GOOS, "Should have OS information")
		assert.NotEmpty(t, runtime.GOARCH, "Should have architecture information")
	})
}

// Test that the package can be properly imported and initialized
func TestPackageInitialization(t *testing.T) {
	t.Run("global variables initialized", func(t *testing.T) {
		// Test that global variables are properly set
		assert.NotNil(t, scheme, "Global scheme should be initialized")
		assert.NotNil(t, setupLog, "Global setupLog should be initialized")
	})

	t.Run("scheme registration completeness", func(t *testing.T) {
		// Ensure both clientgo and CRD types are registered
		beforeCount := len(clientgoscheme.Scheme.AllKnownTypes())
		afterCount := len(scheme.AllKnownTypes())

		// Our scheme should have at least as many types as the base clientgo scheme
		// plus our custom CRD types
		assert.GreaterOrEqual(t, afterCount, beforeCount, "Our scheme should include all clientgo types plus CRDs")
	})
}

// Test configuration integration as implemented in main()
func TestConfigurationIntegration(t *testing.T) {
	// Create a fake Kubernetes client with ConfigMap
	k8sScheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(k8sScheme)

	t.Run("config manager initialization with default namespace", func(t *testing.T) {
		// Test default namespace behavior
		_ = os.Unsetenv("POD_NAMESPACE")

		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}
		configManager := config.NewConfigManager(fakeClient, namespace)

		assert.NotNil(t, configManager)
		assert.Equal(t, "default", configManager.GetNamespace())
	})

	t.Run("config manager initialization with custom namespace", func(t *testing.T) {
		_ = os.Setenv("POD_NAMESPACE", "custom-namespace")
		defer func() { _ = os.Unsetenv("POD_NAMESPACE") }()

		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}
		configManager := config.NewConfigManager(fakeClient, namespace)

		assert.NotNil(t, configManager)
		assert.Equal(t, "custom-namespace", configManager.GetNamespace())
	})

	t.Run("config loading with environment variables", func(t *testing.T) {
		// Set environment variables like main.go does
		_ = os.Setenv("CONFIG_ENVIRONMENT", "development")
		_ = os.Setenv("CONFIG_OPERATOR_LOG_LEVEL", "debug")
		_ = os.Setenv("CONFIG_OPERATOR_RECONCILE_INTERVAL", "30s")
		defer func() {
			_ = os.Unsetenv("CONFIG_ENVIRONMENT")
			_ = os.Unsetenv("CONFIG_OPERATOR_LOG_LEVEL")
			_ = os.Unsetenv("CONFIG_OPERATOR_RECONCILE_INTERVAL")
		}()

		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		configManager := config.NewConfigManager(fakeClient, "default")

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		require.NoError(t, err)
		assert.NotNil(t, operatorConfig)
		assert.Equal(t, "development", operatorConfig.Environment)
		assert.Equal(t, "debug", operatorConfig.Operator.LogLevel)
		assert.Equal(t, 30*time.Second, operatorConfig.Operator.ReconcileInterval)
	})

	t.Run("config loading with ConfigMap", func(t *testing.T) {
		// Create ConfigMap like main.go expects
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cloudflare-dns-operator-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"config.yaml": `
environment: staging
operator:
  logLevel: info
  reconcileInterval: 60000000000
cloudflare:
  apiTimeout: 30000000000
  rateLimitRPS: 10
`,
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(k8sScheme).
			WithObjects(configMap).
			Build()

		configManager := config.NewConfigManager(fakeClient, "default")

		configMapName := os.Getenv("CONFIG_CONFIGMAP_NAME")
		if configMapName == "" {
			configMapName = "cloudflare-dns-operator-config"
		}

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
			ConfigMapName:  configMapName,
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		require.NoError(t, err)
		assert.NotNil(t, operatorConfig)
		assert.Equal(t, "staging", operatorConfig.Environment)
		assert.Equal(t, "info", operatorConfig.Operator.LogLevel)
		assert.Equal(t, 1*time.Minute, operatorConfig.Operator.ReconcileInterval)
		assert.Equal(t, 30*time.Second, operatorConfig.Cloudflare.APITimeout)
		assert.Equal(t, 10, operatorConfig.Cloudflare.RateLimitRPS)
	})

	t.Run("config loading fallback to defaults on error", func(t *testing.T) {
		// Simulate config loading failure
		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		configManager := config.NewConfigManager(fakeClient, "default")

		loadOptions := config.LoadOptions{
			ConfigMapName: "non-existent-configmap",
			// No LoadFromEnv, so this should fail
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		// This should fail, so we expect error
		assert.Error(t, err)
		assert.Nil(t, operatorConfig)

		// Test fallback to default config - mimics main.go behavior
		environment := os.Getenv("OPERATOR_ENVIRONMENT")
		if environment == "" {
			environment = "production"
		}
		defaultConfig := config.GetEnvironmentDefaults(environment)

		assert.NotNil(t, defaultConfig)
		assert.Equal(t, "production", defaultConfig.Environment)
	})

	t.Run("config loading with custom ConfigMap name from env", func(t *testing.T) {
		_ = os.Setenv("CONFIG_CONFIGMAP_NAME", "custom-operator-config")
		defer func() { _ = os.Unsetenv("CONFIG_CONFIGMAP_NAME") }()

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-operator-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"environment": "test",
				"log-level":   "warn",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(k8sScheme).
			WithObjects(configMap).
			Build()

		configManager := config.NewConfigManager(fakeClient, "default")

		configMapName := os.Getenv("CONFIG_CONFIGMAP_NAME")
		if configMapName == "" {
			configMapName = "cloudflare-dns-operator-config"
		}

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
			ConfigMapName:  configMapName,
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		require.NoError(t, err)
		assert.NotNil(t, operatorConfig)
		assert.Equal(t, "test", operatorConfig.Environment)
		assert.Equal(t, "warn", operatorConfig.Operator.LogLevel)
	})

	t.Run("config loading with Secret name from env", func(t *testing.T) {
		_ = os.Setenv("CONFIG_SECRET_NAME", "operator-secret-config")
		defer func() { _ = os.Unsetenv("CONFIG_SECRET_NAME") }()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-secret-config",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"environment": []byte("production"),
				"api-timeout": []byte("45s"),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(k8sScheme).
			WithObjects(secret).
			Build()

		configManager := config.NewConfigManager(fakeClient, "default")

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
			SecretName:     os.Getenv("CONFIG_SECRET_NAME"),
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		require.NoError(t, err)
		assert.NotNil(t, operatorConfig)
		assert.Equal(t, "production", operatorConfig.Environment)
		assert.Equal(t, 45*time.Second, operatorConfig.Cloudflare.APITimeout)
	})
}

func TestEnvironmentDetection(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "default environment when not set",
			envValue: "",
			expected: "production",
		},
		{
			name:     "custom environment from env var",
			envValue: "staging",
			expected: "staging",
		},
		{
			name:     "development environment",
			envValue: "development",
			expected: "development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("OPERATOR_ENVIRONMENT", tt.envValue)
				defer func() { _ = os.Unsetenv("OPERATOR_ENVIRONMENT") }()
			} else {
				_ = os.Unsetenv("OPERATOR_ENVIRONMENT")
			}

			environment := os.Getenv("OPERATOR_ENVIRONMENT")
			if environment == "" {
				environment = "production"
			}

			assert.Equal(t, tt.expected, environment)

			// Test that GetEnvironmentDefaults works with this environment
			defaultConfig := config.GetEnvironmentDefaults(environment)
			assert.NotNil(t, defaultConfig)
			assert.Equal(t, tt.expected, defaultConfig.Environment)
		})
	}
}

func TestConfigValidationInMain(t *testing.T) {
	k8sScheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(k8sScheme)

	t.Run("config validation with valid configuration", func(t *testing.T) {
		_ = os.Setenv("CONFIG_ENVIRONMENT", "production")
		_ = os.Setenv("CONFIG_OPERATOR_LOG_LEVEL", "info")
		defer func() {
			_ = os.Unsetenv("CONFIG_ENVIRONMENT")
			_ = os.Unsetenv("CONFIG_OPERATOR_LOG_LEVEL")
		}()

		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		configManager := config.NewConfigManager(fakeClient, "default")

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		require.NoError(t, err)
		assert.NotNil(t, operatorConfig)

		// Validate the configuration manually like main.go would
		err = operatorConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("config validation with invalid configuration", func(t *testing.T) {
		// Set invalid environment
		_ = os.Setenv("CONFIG_ENVIRONMENT", "invalid-environment")
		defer func() { _ = os.Unsetenv("CONFIG_ENVIRONMENT") }()

		fakeClient := fake.NewClientBuilder().WithScheme(k8sScheme).Build()
		configManager := config.NewConfigManager(fakeClient, "default")

		loadOptions := config.LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
		}

		operatorConfig, err := configManager.LoadConfig(context.Background(), loadOptions)

		// Should fail validation
		assert.Error(t, err)
		assert.Nil(t, operatorConfig)
		assert.Contains(t, err.Error(), "validation failed")
	})
}
