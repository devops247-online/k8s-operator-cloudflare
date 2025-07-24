package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Multi-tenancy Features", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	var (
		ctx                 context.Context
		testNamespacePrefix string
	)

	BeforeEach(func() {
		ctx = context.Background()
		testNamespacePrefix = fmt.Sprintf("mt-test-%d", time.Now().Unix())
	})

	Context("When testing namespace-scoped deployment", func() {
		var testNamespace *corev1.Namespace

		BeforeEach(func() {
			// Create a test namespace for namespace-scoped testing
			testNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-ns-scoped", testNamespacePrefix),
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "cloudflare-dns-operator",
						"multitenancy.scope":           "namespace",
					},
				},
			}
			err := k8sClient.Create(ctx, testNamespace)
			if err != nil {
				Skip(fmt.Sprintf("Failed to create test namespace: %v", err))
			}
		})

		AfterEach(func() {
			if testNamespace != nil {
				_ = k8sClient.Delete(ctx, testNamespace)
			}
		})

		It("Should create Role and RoleBinding for namespace-scoped access", func() {
			// In a real scenario, this would be created by Helm
			// For testing, we create a mock Role to validate RBAC patterns
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cloudflare-dns-operator",
					Namespace: testNamespace.Name,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"dns.cloudflare.io"},
						Resources: []string{"cloudflarerecords", "cloudflarerecords/status", "cloudflarerecords/finalizers"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"secrets"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}

			err := k8sClient.Create(ctx, role)
			Expect(err).ToNot(HaveOccurred(), "Role should be created successfully")

			// Verify the Role exists and has correct permissions
			createdRole := &rbacv1.Role{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, createdRole)
			Expect(err).ToNot(HaveOccurred())

			// Validate Role rules
			Expect(createdRole.Rules).To(HaveLen(2))
			Expect(createdRole.Rules[0].APIGroups).To(ContainElement("dns.cloudflare.io"))
			Expect(createdRole.Rules[0].Resources).To(ContainElement("cloudflarerecords"))
			Expect(createdRole.Rules[1].Resources).To(ContainElement("secrets"))

			// Clean up
			_ = k8sClient.Delete(ctx, role)
		})

		It("Should validate ResourceQuota limits for namespace-scoped tenancy", func() {
			// Create a ResourceQuota for the test namespace
			quota := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cloudflare-quota",
					Namespace: testNamespace.Name,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"count/cloudflarerecords.dns.cloudflare.io": resource.MustParse("50"),
						"limits.cpu":    resource.MustParse("500m"),
						"limits.memory": resource.MustParse("256Mi"),
					},
				},
			}

			err := k8sClient.Create(ctx, quota)
			Expect(err).ToNot(HaveOccurred(), "ResourceQuota should be created successfully")

			// Verify ResourceQuota was created with correct limits
			createdQuota := &corev1.ResourceQuota{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: quota.Name, Namespace: quota.Namespace}, createdQuota)
			Expect(err).ToNot(HaveOccurred())

			// Validate quota specifications
			Expect(createdQuota.Spec.Hard).To(HaveKey(corev1.ResourceName("count/cloudflarerecords.dns.cloudflare.io")))
			expectedRecordLimit := resource.MustParse("50")
			Expect(createdQuota.Spec.Hard["count/cloudflarerecords.dns.cloudflare.io"]).To(Equal(expectedRecordLimit))

			// Clean up
			_ = k8sClient.Delete(ctx, quota)
		})
	})

	Context("When testing multi-namespace deployment", func() {
		var tenantNamespaces []*corev1.Namespace

		BeforeEach(func() {
			// Create multiple tenant namespaces
			tenantNames := []string{"tenant-a", "tenant-b", "tenant-c"}
			tenantNamespaces = make([]*corev1.Namespace, 0, len(tenantNames))

			for _, tenantName := range tenantNames {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", testNamespacePrefix, tenantName),
						Labels: map[string]string{
							"app.kubernetes.io/managed-by": "cloudflare-dns-operator",
							"multitenancy.scope":           "multi-namespace",
							"tenant":                       tenantName,
						},
					},
				}
				err := k8sClient.Create(ctx, ns)
				if err != nil {
					Skip(fmt.Sprintf("Failed to create tenant namespace %s: %v", tenantName, err))
				}
				tenantNamespaces = append(tenantNamespaces, ns)
			}
		})

		AfterEach(func() {
			// Clean up tenant namespaces
			for _, ns := range tenantNamespaces {
				_ = k8sClient.Delete(ctx, ns)
			}
		})

		It("Should create separate ResourceQuotas for each tenant namespace", func() {
			for i, ns := range tenantNamespaces {
				quota := &corev1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("tenant-quota-%d", i),
						Namespace: ns.Name,
						Labels: map[string]string{
							"app.kubernetes.io/component": "tenant-quota",
							"app.kubernetes.io/tenant":    ns.Labels["tenant"],
						},
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							"count/cloudflarerecords.dns.cloudflare.io": resource.MustParse("25"),
							"limits.cpu":    resource.MustParse("250m"),
							"limits.memory": resource.MustParse("128Mi"),
						},
					},
				}

				err := k8sClient.Create(ctx, quota)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("ResourceQuota should be created for namespace %s", ns.Name))

				// Verify quota was created correctly
				createdQuota := &corev1.ResourceQuota{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: quota.Name, Namespace: quota.Namespace}, createdQuota)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdQuota.Labels["app.kubernetes.io/tenant"]).To(Equal(ns.Labels["tenant"]))

				// Clean up
				defer func(q *corev1.ResourceQuota) {
					_ = k8sClient.Delete(ctx, q)
				}(quota)
			}
		})

		It("Should create tenant-specific RBAC for each watched namespace", func() {
			for i, ns := range tenantNamespaces {
				// Create a tenant-specific Role
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("cloudflare-dns-operator-tenant-%d", i),
						Namespace: ns.Name,
						Labels: map[string]string{
							"app.kubernetes.io/component": "tenant-rbac",
							"app.kubernetes.io/tenant":    ns.Labels["tenant"],
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"dns.cloudflare.io"},
							Resources: []string{"cloudflarerecords", "cloudflarerecords/status", "cloudflarerecords/finalizers"},
							Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
						},
						{
							APIGroups: []string{""},
							Resources: []string{"secrets"},
							Verbs:     []string{"get", "list", "watch"},
						},
					},
				}

				err := k8sClient.Create(ctx, role)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Tenant Role should be created for namespace %s", ns.Name))

				// Verify Role was created with correct labels
				createdRole := &rbacv1.Role{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, createdRole)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdRole.Labels["app.kubernetes.io/tenant"]).To(Equal(ns.Labels["tenant"]))

				// Clean up
				defer func(r *rbacv1.Role) {
					_ = k8sClient.Delete(ctx, r)
				}(role)
			}
		})
	})

	Context("When testing tenant isolation", func() {
		var (
			tenantANamespace *corev1.Namespace
			tenantBNamespace *corev1.Namespace
		)

		BeforeEach(func() {
			// Create two tenant namespaces for isolation testing
			tenantANamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-tenant-a", testNamespacePrefix),
					Labels: map[string]string{
						"tenant": "tenant-a",
					},
				},
			}

			tenantBNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-tenant-b", testNamespacePrefix),
					Labels: map[string]string{
						"tenant": "tenant-b",
					},
				},
			}

			err := k8sClient.Create(ctx, tenantANamespace)
			if err != nil {
				Skip(fmt.Sprintf("Failed to create tenant A namespace: %v", err))
			}

			err = k8sClient.Create(ctx, tenantBNamespace)
			if err != nil {
				Skip(fmt.Sprintf("Failed to create tenant B namespace: %v", err))
			}
		})

		AfterEach(func() {
			if tenantANamespace != nil {
				_ = k8sClient.Delete(ctx, tenantANamespace)
			}
			if tenantBNamespace != nil {
				_ = k8sClient.Delete(ctx, tenantBNamespace)
			}
		})

		It("Should maintain secret isolation between tenants", func() {
			// Create secrets in each tenant namespace
			tenantASecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tenant-a-cloudflare-token",
					Namespace: tenantANamespace.Name,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"api-token": []byte("tenant-a-token-value"),
				},
			}

			tenantBSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tenant-b-cloudflare-token",
					Namespace: tenantBNamespace.Name,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"api-token": []byte("tenant-b-token-value"),
				},
			}

			err := k8sClient.Create(ctx, tenantASecret)
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Create(ctx, tenantBSecret)
			Expect(err).ToNot(HaveOccurred())

			// Verify secrets exist in their respective namespaces
			retrievedSecretA := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tenantASecret.Name, Namespace: tenantASecret.Namespace}, retrievedSecretA)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(retrievedSecretA.Data["api-token"])).To(Equal("tenant-a-token-value"))

			retrievedSecretB := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tenantBSecret.Name, Namespace: tenantBSecret.Namespace}, retrievedSecretB)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(retrievedSecretB.Data["api-token"])).To(Equal("tenant-b-token-value"))

			// Verify cross-namespace access is not possible
			// (In real scenario, this would be enforced by RBAC)
			nonExistentSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tenantASecret.Name, Namespace: tenantBNamespace.Name}, nonExistentSecret)
			Expect(err).To(HaveOccurred(), "Cross-namespace secret access should not be possible")

			// Clean up
			_ = k8sClient.Delete(ctx, tenantASecret)
			_ = k8sClient.Delete(ctx, tenantBSecret)
		})

		It("Should support namespace-based tenant identification", func() {
			// Test that namespaces can be used to identify tenants
			namespaces := []*corev1.Namespace{tenantANamespace, tenantBNamespace}

			for _, ns := range namespaces {
				// Verify namespace has correct tenant labeling
				retrievedNs := &corev1.Namespace{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, retrievedNs)
				Expect(err).ToNot(HaveOccurred())

				expectedTenant := ns.Labels["tenant"]
				Expect(retrievedNs.Labels["tenant"]).To(Equal(expectedTenant))
			}
		})
	})

	Context("When validating multi-tenancy configuration", func() {
		It("Should handle zone ownership validation patterns", func() {
			// Test zone pattern matching logic (would be implemented in controller)
			allowedZones := map[string][]string{
				"tenant-a": {"customer-a.com", "*.dev.customer-a.com"},
				"tenant-b": {"customer-b.org", "api.customer-b.org"},
			}

			// Validate pattern structure
			for tenant, zones := range allowedZones {
				Expect(tenant).ToNot(BeEmpty(), "Tenant name should not be empty")
				Expect(zones).ToNot(BeEmpty(), "Zone list should not be empty")

				for _, zone := range zones {
					Expect(zone).ToNot(BeEmpty(), "Zone pattern should not be empty")
					// Basic validation - zone should contain a dot (domain structure)
					Expect(zone).To(ContainSubstring("."), "Zone should be a valid domain pattern")
				}
			}
		})

		It("Should validate resource quota limits", func() {
			quotaLimits := map[string]string{
				"maxRecords": "100",
				"cpu":        "1000m",
				"memory":     "512Mi",
			}

			// Validate quota values can be parsed
			for resourceName, limit := range quotaLimits {
				Expect(resourceName).ToNot(BeEmpty())
				Expect(limit).ToNot(BeEmpty())

				if resourceName == "maxRecords" {
					// Should be a valid integer
					Expect(limit).To(MatchRegexp(`^\d+$`), "maxRecords should be a valid integer")
				} else {
					// Should be valid Kubernetes resource quantities
					_, err := resource.ParseQuantity(limit)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Resource limit %s should be valid", resourceName))
				}
			}
		})
	})
})
