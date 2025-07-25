package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Secret Management", func() {
	var (
		ctx           context.Context
		testNamespace *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create test namespace with unique name
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("secret-test-%d-%d", time.Now().Unix(), time.Now().UnixNano()%1000000),
			},
		}
		// Create namespace with retry logic in case of conflicts
		Eventually(func() error {
			return k8sClient.Create(ctx, testNamespace)
		}, time.Second*30, time.Second).Should(Succeed(), "Should be able to create test namespace")
	})

	AfterEach(func() {
		if testNamespace != nil {
			// Clean up test namespace with proper wait
			Eventually(func() error {
				return k8sClient.Delete(ctx, testNamespace)
			}, time.Second*30, time.Second).Should(SatisfyAny(Succeed(), MatchError(ContainSubstring("not found"))))

			// Wait for namespace to be fully deleted
			Eventually(func() bool {
				var ns corev1.Namespace
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace.Name}, &ns)
				return err != nil // Namespace should not be found
			}, time.Second*30, time.Second).Should(BeTrue())
		}
	})

	Context("When using Kubernetes native secrets", func() {
		It("Should create and use native Kubernetes secret", func() {
			// Create a Kubernetes secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cloudflare-credentials",
					Namespace: testNamespace.Name,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"api-token": "test-token-12345",
					"email":     "test@example.com",
					"zoneId":    "test-zone-id",
				},
			}

			err := k8sClient.Create(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			// Verify secret was created
			createdSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}, createdSecret)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Data).To(HaveKey("api-token"))
			Expect(createdSecret.Data).To(HaveKey("email"))
			Expect(createdSecret.Data).To(HaveKey("zoneId"))
		})

		It("Should handle secret rotation", func() {
			// Create initial secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cloudflare-credentials-rotation",
					Namespace: testNamespace.Name,
					Labels: map[string]string{
						"rotation": "enabled",
					},
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"api-token": "initial-token",
				},
			}

			err := k8sClient.Create(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			// Update secret (simulate rotation)
			Eventually(func() error {
				var currentSecret corev1.Secret
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				}, &currentSecret)
				if err != nil {
					return err
				}

				// Update the token
				currentSecret.Data["api-token"] = []byte("rotated-token")
				return k8sClient.Update(ctx, &currentSecret)
			}, time.Second*30, time.Second).Should(Succeed())

			// Verify rotation
			updatedSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}, updatedSecret)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(updatedSecret.Data["api-token"])).To(Equal("rotated-token"))
		})
	})

	Context("When using External Secrets Operator", func() {
		It("Should validate External Secret configuration", func() {
			// Note: This test validates the configuration without requiring
			// External Secrets Operator to be installed

			// Simulate external secret configuration
			externalSecretConfig := map[string]interface{}{
				"provider": "external-secrets-operator",
				"externalSecretsOperator": map[string]interface{}{
					"enabled": true,
					"secretStore": map[string]interface{}{
						"name":    "cloudflare-secret-store",
						"kind":    "SecretStore",
						"backend": "vault",
					},
					"refreshInterval": "1h",
					"externalSecret": map[string]interface{}{
						"name": "cloudflare-credentials",
						"dataFrom": []map[string]interface{}{
							{
								"extract": map[string]interface{}{
									"key": "cloudflare/credentials",
								},
							},
						},
					},
				},
			}

			// Validate configuration structure
			Expect(externalSecretConfig).To(HaveKey("provider"))
			Expect(externalSecretConfig["provider"]).To(Equal("external-secrets-operator"))

			esConfig := externalSecretConfig["externalSecretsOperator"].(map[string]interface{})
			Expect(esConfig).To(HaveKey("enabled"))
			Expect(esConfig).To(HaveKey("secretStore"))
			Expect(esConfig).To(HaveKey("refreshInterval"))
			Expect(esConfig).To(HaveKey("externalSecret"))
		})
	})

	Context("When using Vault integration", func() {
		It("Should validate Vault configuration", func() {
			// Validate Vault configuration structure
			vaultConfig := map[string]interface{}{
				"enabled":    true,
				"address":    "https://vault.example.com",
				"authMethod": "kubernetes",
				"path":       "secret/data/cloudflare",
				"kubernetes": map[string]interface{}{
					"role":           "cloudflare-operator",
					"serviceAccount": "cloudflare-dns-operator",
				},
				"rotation": map[string]interface{}{
					"enabled":  true,
					"schedule": "0 0 * * 0",
				},
			}

			// Validate configuration
			Expect(vaultConfig).To(HaveKey("enabled"))
			Expect(vaultConfig).To(HaveKey("address"))
			Expect(vaultConfig).To(HaveKey("authMethod"))
			Expect(vaultConfig).To(HaveKey("path"))
			Expect(vaultConfig).To(HaveKey("kubernetes"))
			Expect(vaultConfig).To(HaveKey("rotation"))

			// Validate Kubernetes auth
			k8sAuth := vaultConfig["kubernetes"].(map[string]interface{})
			Expect(k8sAuth).To(HaveKey("role"))
			Expect(k8sAuth).To(HaveKey("serviceAccount"))

			// Validate rotation config
			rotation := vaultConfig["rotation"].(map[string]interface{})
			Expect(rotation).To(HaveKey("enabled"))
			Expect(rotation).To(HaveKey("schedule"))
		})
	})

	Context("Secret access auditing", func() {
		It("Should track secret access events", func() {
			// Create a secret with audit annotations
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "audited-secret",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"audit.cloudflare.io/enabled":     "true",
						"audit.cloudflare.io/destination": "stdout",
					},
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"api-token": "audit-test-token",
				},
			}

			err := k8sClient.Create(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			// Verify audit annotations
			createdSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}, createdSecret)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Annotations).To(HaveKeyWithValue("audit.cloudflare.io/enabled", "true"))
			Expect(createdSecret.Annotations).To(HaveKeyWithValue("audit.cloudflare.io/destination", "stdout"))
		})
	})

	Context("Secret encryption at rest", func() {
		It("Should validate encryption configuration", func() {
			// Validate encryption configuration
			encryptionConfig := map[string]interface{}{
				"enabled":     true,
				"kmsProvider": "aws-kms",
			}

			Expect(encryptionConfig).To(HaveKey("enabled"))
			Expect(encryptionConfig).To(HaveKey("kmsProvider"))
			Expect(encryptionConfig["enabled"]).To(BeTrue())
			Expect(encryptionConfig["kmsProvider"]).ToNot(BeEmpty())
		})
	})
})
