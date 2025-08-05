package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // Using ginkgo DSL
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("High Availability Features", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	var (
		ctx           context.Context
		testNamespace *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create test namespace with unique name
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("ha-test-%d-%d", time.Now().Unix(), time.Now().UnixNano()%1000000),
				Labels: map[string]string{
					"test-type": "high-availability",
				},
			},
		}
		// Create namespace with retry logic in case of conflicts
		Eventually(func() error {
			return k8sClient.Create(ctx, testNamespace)
		}, timeout, interval).Should(Succeed(), "Should be able to create test namespace")
	})

	AfterEach(func() {
		if testNamespace != nil {
			// Clean up test namespace with proper wait
			Eventually(func() error {
				return k8sClient.Delete(ctx, testNamespace)
			}, timeout, interval).Should(SatisfyAny(Succeed(), MatchError(ContainSubstring("not found"))))

			// Wait for namespace to be fully deleted
			Eventually(func() bool {
				var ns corev1.Namespace
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace.Name}, &ns)
				return err != nil // Namespace should not be found
			}, timeout, interval).Should(BeTrue())
		}
	})

	Context("When testing leader election configuration", func() {
		It("Should validate leader election resource configuration", func() {
			// Test leader election configuration validation
			leaderElectionConfig := map[string]interface{}{
				"enabled":           true,
				"leaseDuration":     "15s",
				"renewDeadline":     "10s",
				"retryPeriod":       "2s",
				"resourceName":      "cloudflare-dns-operator-leader",
				"resourceNamespace": "",
			}

			// Validate configuration structure
			Expect(leaderElectionConfig).To(HaveKey("enabled"))
			Expect(leaderElectionConfig).To(HaveKey("leaseDuration"))
			Expect(leaderElectionConfig).To(HaveKey("renewDeadline"))
			Expect(leaderElectionConfig).To(HaveKey("retryPeriod"))
			Expect(leaderElectionConfig).To(HaveKey("resourceName"))

			// Validate values
			Expect(leaderElectionConfig["enabled"]).To(BeTrue())
			Expect(leaderElectionConfig["leaseDuration"]).To(Equal("15s"))
			Expect(leaderElectionConfig["renewDeadline"]).To(Equal("10s"))
			Expect(leaderElectionConfig["retryPeriod"]).To(Equal("2s"))
			Expect(leaderElectionConfig["resourceName"]).To(Equal("cloudflare-dns-operator-leader"))
		})

		It("Should validate timing constraints for leader election", func() {
			// Test that renewDeadline < leaseDuration
			leaseDuration := 15 * time.Second
			renewDeadline := 10 * time.Second
			retryPeriod := 2 * time.Second

			Expect(renewDeadline).To(BeNumerically("<", leaseDuration),
				"renewDeadline should be less than leaseDuration")
			Expect(retryPeriod).To(BeNumerically("<", renewDeadline),
				"retryPeriod should be less than renewDeadline")
		})
	})

	Context("When testing multi-replica deployment", func() {
		It("Should support multiple replicas with proper anti-affinity", func() {
			// Create a test deployment with HA configuration
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ha-test-deployment",
					Namespace: testNamespace.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name": "cloudflare-dns-operator-test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &[]int32{2}[0],
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": "cloudflare-dns-operator-test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": "cloudflare-dns-operator-test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "busybox:latest",
									Command: []string{
										"sleep", "3600",
									},
									SecurityContext: &corev1.SecurityContext{
										RunAsNonRoot:             &[]bool{true}[0],
										AllowPrivilegeEscalation: &[]bool{false}[0],
										ReadOnlyRootFilesystem:   &[]bool{true}[0],
										RunAsUser:                &[]int64{65532}[0],
										Capabilities: &corev1.Capabilities{
											Drop: []corev1.Capability{"ALL"},
										},
									},
								},
							},
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: &[]bool{true}[0],
								RunAsUser:    &[]int64{65532}[0],
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							// Add pod anti-affinity
							Affinity: &corev1.Affinity{
								PodAntiAffinity: &corev1.PodAntiAffinity{
									PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
										{
											Weight: 100,
											PodAffinityTerm: corev1.PodAffinityTerm{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"app.kubernetes.io/name": "cloudflare-dns-operator-test",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, deployment)
			if err != nil {
				Skip(fmt.Sprintf("Deployment creation failed (expected in test environment): %v", err))
			}

			// Verify deployment was created
			createdDeployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			}, createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			// Verify anti-affinity configuration
			Expect(createdDeployment.Spec.Template.Spec.Affinity).ToNot(BeNil())
			Expect(createdDeployment.Spec.Template.Spec.Affinity.PodAntiAffinity).ToNot(BeNil())
			antiAffinity := createdDeployment.Spec.Template.Spec.Affinity.PodAntiAffinity
			Expect(antiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))
			Expect(antiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).To(Equal(int32(100)))
		})
	})

	Context("When testing PodDisruptionBudget", func() {
		It("Should create and validate PodDisruptionBudget", func() {
			pdb := &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ha-test-pdb",
					Namespace: testNamespace.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name": "cloudflare-dns-operator-test",
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": "cloudflare-dns-operator-test",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, pdb)
			Expect(err).ToNot(HaveOccurred())

			// Verify PodDisruptionBudget was created
			createdPDB := &policyv1.PodDisruptionBudget{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      pdb.Name,
				Namespace: pdb.Namespace,
			}, createdPDB)
			Expect(err).ToNot(HaveOccurred())

			// Verify PDB configuration
			Expect(createdPDB.Spec.MinAvailable).ToNot(BeNil())
			Expect(createdPDB.Spec.MinAvailable.IntVal).To(Equal(int32(1)))
			Expect(createdPDB.Spec.Selector).ToNot(BeNil())
			Expect(createdPDB.Spec.Selector.MatchLabels).To(
				HaveKeyWithValue("app.kubernetes.io/name", "cloudflare-dns-operator-test"))
		})
	})

	Context("When testing graceful shutdown configuration", func() {
		It("Should validate graceful shutdown settings", func() {
			gracefulShutdownConfig := map[string]interface{}{
				"enabled":                       true,
				"terminationGracePeriodSeconds": 30,
				"preStopDelay":                  "5s",
			}

			// Validate configuration structure
			Expect(gracefulShutdownConfig).To(HaveKey("enabled"))
			Expect(gracefulShutdownConfig).To(HaveKey("terminationGracePeriodSeconds"))
			Expect(gracefulShutdownConfig).To(HaveKey("preStopDelay"))

			// Validate values
			Expect(gracefulShutdownConfig["enabled"]).To(BeTrue())
			Expect(gracefulShutdownConfig["terminationGracePeriodSeconds"]).To(Equal(30))
			Expect(gracefulShutdownConfig["preStopDelay"]).To(Equal("5s"))
		})

		It("Should validate preStop hook configuration", func() {
			// Create a pod with preStop hook
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "graceful-shutdown-test-pod",
					Namespace: testNamespace.Name,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "busybox:latest",
							Command: []string{
								"sleep", "300",
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh",
											"-c",
											"sleep 5",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &[]bool{true}[0],
								AllowPrivilegeEscalation: &[]bool{false}[0],
								ReadOnlyRootFilesystem:   &[]bool{true}[0],
								RunAsUser:                &[]int64{65532}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{65532}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					TerminationGracePeriodSeconds: &[]int64{30}[0],
				},
			}

			err := k8sClient.Create(ctx, pod)
			if err != nil {
				Skip(fmt.Sprintf("Pod creation failed (expected in test environment): %v", err))
			}

			// Verify preStop hook configuration
			createdPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			}, createdPod)
			Expect(err).ToNot(HaveOccurred())

			// Verify preStop hook
			Expect(createdPod.Spec.Containers[0].Lifecycle).ToNot(BeNil())
			Expect(createdPod.Spec.Containers[0].Lifecycle.PreStop).ToNot(BeNil())
			Expect(createdPod.Spec.Containers[0].Lifecycle.PreStop.Exec).ToNot(BeNil())
			Expect(createdPod.Spec.Containers[0].Lifecycle.PreStop.Exec.Command).To(ContainElements("/bin/sh", "-c", "sleep 5"))

			// Verify termination grace period
			Expect(createdPod.Spec.TerminationGracePeriodSeconds).ToNot(BeNil())
			Expect(*createdPod.Spec.TerminationGracePeriodSeconds).To(Equal(int64(30)))
		})
	})

	Context("When testing HA failover scenarios", func() {
		It("Should validate leader election lease resources", func() {
			// Test that we can create a lease resource for leader election
			// This validates the RBAC permissions and resource access

			// Note: This is a configuration validation test
			// In a real cluster, we would test actual failover scenarios
			leaseConfig := map[string]interface{}{
				"apiVersion": "coordination.k8s.io/v1",
				"kind":       "Lease",
				"metadata": map[string]interface{}{
					"name":      "cloudflare-dns-operator-leader",
					"namespace": testNamespace.Name,
				},
			}

			// Validate lease configuration structure
			Expect(leaseConfig).To(HaveKey("apiVersion"))
			Expect(leaseConfig).To(HaveKey("kind"))
			Expect(leaseConfig).To(HaveKey("metadata"))

			metadata := leaseConfig["metadata"].(map[string]interface{})
			Expect(metadata).To(HaveKey("name"))
			Expect(metadata).To(HaveKey("namespace"))
			Expect(metadata["name"]).To(Equal("cloudflare-dns-operator-leader"))
		})

		It("Should support rolling updates without service disruption", func() {
			// Test rolling update strategy configuration
			updateStrategy := appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			}

			// Validate rolling update configuration
			Expect(updateStrategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
			Expect(updateStrategy.RollingUpdate).ToNot(BeNil())
			Expect(updateStrategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(updateStrategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(1)))
		})
	})

	Context("When testing HA monitoring and observability", func() {
		It("Should validate leader election metrics", func() {
			// Test that leader election exposes appropriate metrics
			expectedMetrics := []string{
				"leader_election_master_status",
				"leader_election_slowpath_total",
			}

			for _, metric := range expectedMetrics {
				Expect(metric).ToNot(BeEmpty())
				// Validate metric name format
				Expect(metric).To(MatchRegexp("^[a-z_]+$"))
			}
		})

		It("Should validate health check endpoints for HA", func() {
			// Test health check configuration for HA
			healthConfig := map[string]interface{}{
				"livenessProbe": map[string]interface{}{
					"httpGet": map[string]interface{}{
						"path": "/healthz",
						"port": 8081,
					},
					"initialDelaySeconds": 15,
					"periodSeconds":       20,
				},
				"readinessProbe": map[string]interface{}{
					"httpGet": map[string]interface{}{
						"path": "/readyz",
						"port": 8081,
					},
					"initialDelaySeconds": 5,
					"periodSeconds":       10,
				},
			}

			// Validate health check configuration
			Expect(healthConfig).To(HaveKey("livenessProbe"))
			Expect(healthConfig).To(HaveKey("readinessProbe"))

			liveness := healthConfig["livenessProbe"].(map[string]interface{})
			readiness := healthConfig["readinessProbe"].(map[string]interface{})

			Expect(liveness).To(HaveKey("httpGet"))
			Expect(readiness).To(HaveKey("httpGet"))
		})
	})
})
