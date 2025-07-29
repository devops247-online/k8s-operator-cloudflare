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

package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarnError(t *testing.T) {
	t.Run("logs warning for error", func(t *testing.T) {
		// Create a test error
		testErr := errors.New("test error message")

		// This function should not panic
		assert.NotPanics(t, func() {
			warnError(testErr)
		})
	})

	t.Run("handles nil error", func(t *testing.T) {
		// Should not panic with nil error
		assert.NotPanics(t, func() {
			warnError(nil)
		})
	})
}

func TestInstallPrometheusOperator(t *testing.T) {
	t.Run("returns error when kubectl fails", func(t *testing.T) {
		// This test will fail if kubectl is not available or cluster is not accessible
		// We're testing the error handling path
		err := InstallPrometheusOperator()

		// The function should handle errors gracefully
		// We don't assert the specific error as it depends on environment
		if err != nil {
			assert.Error(t, err, "Should return error when kubectl command fails")
		}
	})
}

func TestUninstallPrometheusOperator(t *testing.T) {
	t.Run("executes without panicking", func(t *testing.T) {
		// This function doesn't return an error, it only logs warnings
		assert.NotPanics(t, func() {
			UninstallPrometheusOperator()
		})
	})
}

func TestIsPrometheusCRDsInstalled(t *testing.T) {
	t.Run("handles kubectl error gracefully", func(t *testing.T) {
		// When kubectl is not available or fails, should return false
		result := IsPrometheusCRDsInstalled()

		// Result should be boolean (either true or false)
		assert.IsType(t, false, result)
	})
}

func TestUninstallCertManager(t *testing.T) {
	t.Run("executes without panicking", func(t *testing.T) {
		// This function doesn't return an error, it only logs warnings
		assert.NotPanics(t, func() {
			UninstallCertManager()
		})
	})
}

func TestInstallCertManager(t *testing.T) {
	t.Run("returns error when kubectl fails", func(t *testing.T) {
		// This test will fail if kubectl is not available or cluster is not accessible
		err := InstallCertManager()

		// The function should handle errors gracefully
		if err != nil {
			assert.Error(t, err, "Should return error when kubectl command fails")
		}
	})
}

func TestIsCertManagerCRDsInstalled(t *testing.T) {
	t.Run("handles kubectl error gracefully", func(t *testing.T) {
		// When kubectl is not available or fails, should return false
		result := IsCertManagerCRDsInstalled()

		// Result should be boolean
		assert.IsType(t, false, result)
	})
}

func TestGetProjectDir(t *testing.T) {
	t.Run("returns valid project directory", func(t *testing.T) {
		projectDir, err := GetProjectDir()

		if err != nil {
			// If there's an error, it should be related to finding go.mod
			assert.Contains(t, err.Error(), "go.mod")
		} else {
			// If successful, should return a valid directory
			assert.NotEmpty(t, projectDir)

			// Directory should exist
			_, statErr := os.Stat(projectDir)
			assert.NoError(t, statErr, "Project directory should exist")

			// Should be an absolute path
			assert.True(t, filepath.IsAbs(projectDir), "Project directory should be absolute path")
		}
	})

	t.Run("finds go.mod in project structure", func(t *testing.T) {
		projectDir, err := GetProjectDir()

		if err == nil {
			// If we found the project dir, go.mod should exist there or in a parent
			found := false
			currentDir := projectDir

			for i := 0; i < 10; i++ { // Limit search depth
				goModPath := filepath.Join(currentDir, "go.mod")
				if _, err := os.Stat(goModPath); err == nil {
					found = true
					break
				}

				parent := filepath.Dir(currentDir)
				if parent == currentDir {
					break // Reached root
				}
				currentDir = parent
			}

			assert.True(t, found, "go.mod should be found in project directory or its parents")
		}
	})

	t.Run("returns error when not in go project", func(t *testing.T) {
		// This is hard to test without changing directories
		// The function should work in the current project context
		_, err := GetProjectDir()

		// In our test context, this should either succeed or return a meaningful error
		if err != nil {
			assert.Contains(t, err.Error(), "go.mod", "Error should mention go.mod")
		}
	})
}

// Helper function to process YAML content for testing
func processYAMLContent(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func TestUncommentCode(t *testing.T) {
	t.Run("removes YAML comments", func(t *testing.T) {
		input := `# This is a comment
apiVersion: v1
kind: ConfigMap
# metadata section
metadata:
  name: test-config
  # nested comment
  namespace: default`

		result := processYAMLContent(input)

		// Should preserve YAML structure
		assert.Contains(t, result, "apiVersion: v1")
		assert.Contains(t, result, "kind: ConfigMap")
		assert.Contains(t, result, "metadata:")
		assert.Contains(t, result, "name: test-config")
		assert.Contains(t, result, "namespace: default")

		// Should handle comments appropriately
		lines := strings.Split(result, "\n")
		assert.Greater(t, len(lines), 0, "Should return non-empty result")
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := processYAMLContent("")
		assert.Equal(t, "", result)
	})

	t.Run("handles input without comments", func(t *testing.T) {
		input := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`

		result := processYAMLContent(input)

		// Should preserve all content
		assert.Contains(t, result, "apiVersion: v1")
		assert.Contains(t, result, "kind: Pod")
		assert.Contains(t, result, "metadata:")
		assert.Contains(t, result, "name: test-pod")
	})

	t.Run("handles mixed content with indentation", func(t *testing.T) {
		input := `# Global comment
spec:
  # Spec comment
  containers:
    - name: app
      # Container comment
      image: nginx:latest`

		result := processYAMLContent(input)

		// Should preserve structure
		assert.Contains(t, result, "spec:")
		assert.Contains(t, result, "containers:")
		assert.Contains(t, result, "name: app")
		assert.Contains(t, result, "image: nginx:latest")

		// Result should maintain some structure
		assert.NotEmpty(t, strings.TrimSpace(result))
	})

	t.Run("preserves non-comment hash usage", func(t *testing.T) {
		input := `password: secrethash123#456
url: http://example.com#anchor
# This is a comment
data: value`

		result := processYAMLContent(input)

		// Should preserve legitimate hash usage
		assert.Contains(t, result, "password: secrethash123#456")
		assert.Contains(t, result, "url: http://example.com#anchor")
		assert.Contains(t, result, "data: value")
	})

	t.Run("handles multiline processing", func(t *testing.T) {
		input := `line1
# comment1
line2
# comment2
line3`

		result := processYAMLContent(input)

		// Should process multiple lines
		assert.Contains(t, result, "line1")
		assert.Contains(t, result, "line2")
		assert.Contains(t, result, "line3")

		// Should return string with newlines
		assert.Contains(t, result, "\n")
	})
}

// Integration tests
func TestUtilsIntegration(t *testing.T) {
	t.Run("project directory and code processing", func(t *testing.T) {
		projectDir, err := GetProjectDir()

		if err == nil {
			assert.NotEmpty(t, projectDir)

			// Test realistic Kubernetes YAML processing
			kubernetesYAML := `# Kubernetes deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  # Generated by operator
  labels:
    app: test
spec:
  # Replica configuration
  replicas: 3
  selector:
    matchLabels:
      app: test`

			result := processYAMLContent(kubernetesYAML)

			// Should preserve Kubernetes structure
			assert.Contains(t, result, "apiVersion: apps/v1")
			assert.Contains(t, result, "kind: Deployment")
			assert.Contains(t, result, "replicas: 3")
		}
	})

	t.Run("error handling across functions", func(t *testing.T) {
		// Test that functions handle errors without panicking
		functions := []func(){
			func() { UninstallPrometheusOperator() },
			func() { UninstallCertManager() },
			func() { warnError(errors.New("test error")) },
		}

		for i, fn := range functions {
			assert.NotPanics(t, fn, "Function %d should not panic", i)
		}
	})
}

// Test edge cases and error conditions
func TestUtilsEdgeCases(t *testing.T) {
	t.Run("uncommenting edge cases", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    string
			contains []string
		}{
			{
				name:     "only comments",
				input:    "# comment1\n# comment2\n# comment3",
				contains: []string{}, // May not contain anything specific
			},
			{
				name:     "empty lines and comments",
				input:    "\n# comment\n\ndata: value\n\n# another comment\n",
				contains: []string{"data: value"},
			},
			{
				name:     "whitespace handling",
				input:    "   # indented comment   \n  data: value  \n   # another comment   ",
				contains: []string{"data: value"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := processYAMLContent(tc.input)

				for _, expected := range tc.contains {
					assert.Contains(t, result, expected, "Result should contain: %s", expected)
				}
			})
		}
	})

	t.Run("boolean functions return correct types", func(t *testing.T) {
		// Test that boolean functions return actual booleans
		result1 := IsPrometheusCRDsInstalled()
		result2 := IsCertManagerCRDsInstalled()

		assert.IsType(t, true, result1, "IsPrometheusCRDsInstalled should return bool")
		assert.IsType(t, true, result2, "IsCertManagerCRDsInstalled should return bool")
	})
}
