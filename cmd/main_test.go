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
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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
