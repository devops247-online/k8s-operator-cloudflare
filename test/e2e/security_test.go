package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Pod Security Standards", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	var (
		namespace *corev1.Namespace
		ctx       context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-pss-%d", time.Now().Unix()),
			},
		}
		Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
	})

	AfterEach(func() {
		// Clean up namespace
		Expect(k8sClient.Delete(ctx, namespace)).Should(Succeed())
	})

	Context("When deploying the operator", func() {
		It("Should have Pod Security Standards labels on namespace", func() {
			// Get the operator namespace
			operatorNamespace := &corev1.Namespace{}
			nsName := types.NamespacedName{Name: "cloudflare-operator-system"}

			Eventually(func() error {
				return k8sClient.Get(ctx, nsName, operatorNamespace)
			}, timeout, interval).Should(Succeed())

			// Check Pod Security Standards labels
			labels := operatorNamespace.Labels
			Expect(labels).To(HaveKeyWithValue("pod-security.kubernetes.io/enforce", "restricted"))
			Expect(labels).To(HaveKeyWithValue("pod-security.kubernetes.io/audit", "restricted"))
			Expect(labels).To(HaveKeyWithValue("pod-security.kubernetes.io/warn", "restricted"))
		})

		It("Should have proper security context on operator pods", func() {
			// List operator pods
			podList := &corev1.PodList{}
			listOpts := []client.ListOption{
				client.InNamespace("cloudflare-operator-system"),
				client.MatchingLabels{"app.kubernetes.io/name": "cloudflare-dns-operator"},
			}

			Eventually(func() int {
				err := k8sClient.List(ctx, podList, listOpts...)
				if err != nil {
					return 0
				}
				return len(podList.Items)
			}, timeout, interval).Should(BeNumerically(">", 0))

			// Check each pod's security context
			for _, pod := range podList.Items {
				// Check pod security context
				Expect(pod.Spec.SecurityContext).ToNot(BeNil())
				Expect(pod.Spec.SecurityContext.RunAsNonRoot).ToNot(BeNil())
				Expect(*pod.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())
				Expect(pod.Spec.SecurityContext.RunAsUser).ToNot(BeNil())
				Expect(*pod.Spec.SecurityContext.RunAsUser).To(Equal(int64(65532)))
				Expect(pod.Spec.SecurityContext.SeccompProfile).ToNot(BeNil())
				Expect(pod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))

				// Check container security context
				for _, container := range pod.Spec.Containers {
					Expect(container.SecurityContext).ToNot(BeNil())
					Expect(container.SecurityContext.AllowPrivilegeEscalation).ToNot(BeNil())
					Expect(*container.SecurityContext.AllowPrivilegeEscalation).To(BeFalse())
					Expect(container.SecurityContext.ReadOnlyRootFilesystem).ToNot(BeNil())
					Expect(*container.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
					Expect(container.SecurityContext.Capabilities).ToNot(BeNil())
					Expect(container.SecurityContext.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
				}
			}
		})

		It("Should create NetworkPolicy when enabled", func() {
			// Check if NetworkPolicy exists
			networkPolicy := &corev1.NetworkPolicy{}
			nsName := types.NamespacedName{
				Name:      "cloudflare-dns-operator",
				Namespace: "cloudflare-operator-system",
			}

			err := k8sClient.Get(ctx, nsName, networkPolicy)
			if err != nil && !errors.IsNotFound(err) {
				Fail(fmt.Sprintf("Failed to get NetworkPolicy: %v", err))
			}

			// If NetworkPolicy is enabled in values, it should exist
			if err == nil {
				// Verify NetworkPolicy spec
				Expect(networkPolicy.Spec.PodSelector.MatchLabels).To(HaveKey("app.kubernetes.io/name"))
				Expect(networkPolicy.Spec.PolicyTypes).To(ContainElements(
					corev1.PolicyTypeIngress,
					corev1.PolicyTypeEgress,
				))

				// Check egress rules for DNS and Cloudflare API
				Expect(networkPolicy.Spec.Egress).ToNot(BeEmpty())
				foundDNS := false
				foundCloudflare := false

				for _, egress := range networkPolicy.Spec.Egress {
					for _, port := range egress.Ports {
						if port.Port != nil && port.Port.IntVal == 53 {
							foundDNS = true
						}
						if port.Port != nil && port.Port.IntVal == 443 {
							foundCloudflare = true
						}
					}
				}

				Expect(foundDNS).To(BeTrue(), "NetworkPolicy should allow DNS traffic")
				Expect(foundCloudflare).To(BeTrue(), "NetworkPolicy should allow HTTPS traffic to Cloudflare")
			}
		})
	})

	Context("When testing security compliance", func() {
		It("Should reject pods that don't meet security standards", func() {
			if namespace.Labels == nil {
				namespace.Labels = make(map[string]string)
			}
			namespace.Labels["pod-security.kubernetes.io/enforce"] = "restricted"

			// Update namespace with security labels
			Expect(k8sClient.Update(ctx, namespace)).Should(Succeed())

			// Create a non-compliant pod (runs as root)
			nonCompliantPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-compliant-pod",
					Namespace: namespace.Name,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "busybox:latest",
							Command: []string{"sleep", "3600"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &[]int64{0}[0], // Running as root
							},
						},
					},
				},
			}

			// This should fail due to Pod Security Standards
			err := k8sClient.Create(ctx, nonCompliantPod)
			Expect(err).To(HaveOccurred())
		})

		It("Should accept pods that meet security standards", func() {
			if namespace.Labels == nil {
				namespace.Labels = make(map[string]string)
			}
			namespace.Labels["pod-security.kubernetes.io/enforce"] = "restricted"

			// Update namespace with security labels
			Expect(k8sClient.Update(ctx, namespace)).Should(Succeed())

			// Create a compliant pod
			compliantPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "compliant-pod",
					Namespace: namespace.Name,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{65532}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "busybox:latest",
							Command: []string{"sleep", "3600"},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &[]bool{false}[0],
								ReadOnlyRootFilesystem:   &[]bool{true}[0],
								RunAsNonRoot:            &[]bool{true}[0],
								RunAsUser:               &[]int64{65532}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
				},
			}

			// This should succeed
			Expect(k8sClient.Create(ctx, compliantPod)).Should(Succeed())

			// Clean up
			Expect(k8sClient.Delete(ctx, compliantPod)).Should(Succeed())
		})
	})
})
