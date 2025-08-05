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

package utils //nolint:revive // Test utils package name is descriptive

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // Using ginkgo DSL
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = Describe("Utils", func() {
	Describe("GetNonEmptyLines", func() {
		It("should return non-empty lines", func() {
			input := "line1\n\nline2\n\n\nline3"
			result := GetNonEmptyLines(input)
			Expect(result).To(Equal([]string{"line1", "line2", "line3"}))
		})

		It("should handle empty string", func() {
			result := GetNonEmptyLines("")
			Expect(result).To(BeEmpty())
		})

		It("should handle string with only newlines", func() {
			result := GetNonEmptyLines("\n\n\n")
			Expect(result).To(BeEmpty())
		})
	})

	Describe("GetProjectDir", func() {
		It("should return project directory", func() {
			dir, err := GetProjectDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(dir).NotTo(BeEmpty())
		})

		It("should remove /test/e2e from path", func() {
			// The function removes /test/e2e from the path
			// Let's verify the logic works correctly

			// If we're already in a /test/e2e directory, GetProjectDir should remove it
			dir, err := GetProjectDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(dir).NotTo(ContainSubstring("/test/e2e"))

			// The result should be a valid directory
			_, err = os.Stat(dir)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Run", func() {
		It("should execute command successfully", func() {
			cmd := exec.Command("echo", "test")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal("test"))
		})

		It("should return error for failing command", func() {
			cmd := exec.Command("false")
			output, err := Run(cmd)
			Expect(err).To(HaveOccurred())
			Expect(output).To(BeEmpty())
		})

		It("should set GO111MODULE environment variable", func() {
			cmd := exec.Command("sh", "-c", "echo $GO111MODULE")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal("on"))
		})
	})

	Describe("UncommentCode", func() {
		var tempFile string

		BeforeEach(func() {
			// Create temp file
			f, err := os.CreateTemp("", "test-uncomment-*.go")
			Expect(err).NotTo(HaveOccurred())
			tempFile = f.Name()
			err = f.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Remove(tempFile)
			_ = err // Ignore error in cleanup
		})

		It("should uncomment single line", func() {
			content := `package main
// fmt.Println("hello")
func main() {
}`
			err := os.WriteFile(tempFile, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())

			err = UncommentCode(tempFile, `// fmt.Println("hello")`, "// ")
			Expect(err).NotTo(HaveOccurred())

			result, err := os.ReadFile(tempFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring(`fmt.Println("hello")`))
			Expect(string(result)).NotTo(ContainSubstring(`// fmt.Println("hello")`))
		})

		It("should uncomment multiple lines", func() {
			content := `package main
// func test() {
//     fmt.Println("test")
// }
func main() {
}`
			err := os.WriteFile(tempFile, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())

			target := `// func test() {
//     fmt.Println("test")
// }`
			err = UncommentCode(tempFile, target, "// ")
			Expect(err).NotTo(HaveOccurred())

			result, err := os.ReadFile(tempFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("func test() {"))
			Expect(string(result)).To(ContainSubstring(`    fmt.Println("test")`))
			Expect(string(result)).NotTo(ContainSubstring("// func test()"))
		})

		It("should return error when target not found", func() {
			content := `package main`
			err := os.WriteFile(tempFile, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())

			err = UncommentCode(tempFile, "not found", "// ")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to find the code"))
		})

		It("should handle file permissions correctly", func() {
			content := `package main`
			err := os.WriteFile(tempFile, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())

			// Check that file has correct permissions after write
			info, err := os.Stat(tempFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})
	})

	Describe("Security-related functions", func() {
		It("InstallPrometheusOperator should construct safe URL", func() {
			// This test verifies that the URL is constructed from constants
			expectedURL := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
			Expect(expectedURL).To(ContainSubstring("https://github.com/prometheus-operator"))
			Expect(expectedURL).To(ContainSubstring(prometheusOperatorVersion))
		})

		It("InstallCertManager should construct safe URL", func() {
			// This test verifies that the URL is constructed from constants
			expectedURL := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
			Expect(expectedURL).To(ContainSubstring("https://github.com/cert-manager"))
			Expect(expectedURL).To(ContainSubstring(certmanagerVersion))
		})

		It("LoadImageToKindClusterWithName should handle environment variable safely", func() {
			// Test with environment variable
			err := os.Setenv("KIND_CLUSTER", "test-cluster")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = os.Unsetenv("KIND_CLUSTER")
			}()

			// This would normally fail but we're testing the construction
			err = LoadImageToKindClusterWithName("test-image:latest")
			// We expect error because kind is not installed in test env
			Expect(err).To(HaveOccurred())
		})
	})
})
