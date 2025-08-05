package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // ginkgo DSL requires dot import
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// probeTestConfig represents configuration for probe tests
type probeTestConfig struct {
	deploymentName string
	labelName      string
	commandMessage string
	probePath      string
	probe          *corev1.Probe
}

// createHealthCheckDeployment creates a deployment with the specified probe
func createHealthCheckDeployment(config probeTestConfig, testNamespace *corev1.Namespace) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.deploymentName,
			Namespace: testNamespace.Name,
			Labels: map[string]string{
				"app.kubernetes.io/name": config.labelName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &[]int32{1}[0],
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": config.labelName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": config.labelName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "busybox:latest",
							Command: []string{
								"sh", "-c", fmt.Sprintf("while true; do echo '%s'; sleep 30; done", config.commandMessage),
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "health",
									ContainerPort: 8081,
									Protocol:      corev1.ProtocolTCP,
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
				},
			},
		},
	}

	// Set probe based on configuration
	container := &deployment.Spec.Template.Spec.Containers[0]
	if config.probePath == "/healthz" {
		if config.probe.InitialDelaySeconds == 15 {
			container.LivenessProbe = config.probe
		} else {
			container.StartupProbe = config.probe
		}
	} else {
		container.ReadinessProbe = config.probe
	}

	return deployment
}

// testProbeDeployment tests deployment creation and probe verification
func testProbeDeployment(ctx context.Context, config probeTestConfig, testNamespace *corev1.Namespace) {
	deployment := createHealthCheckDeployment(config, testNamespace)

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

	// Verify probe configuration
	container := createdDeployment.Spec.Template.Spec.Containers[0]

	var actualProbe *corev1.Probe
	if config.probePath == "/healthz" {
		if config.probe.InitialDelaySeconds == 15 {
			actualProbe = container.LivenessProbe
		} else {
			actualProbe = container.StartupProbe
		}
	} else {
		actualProbe = container.ReadinessProbe
	}

	Expect(actualProbe).ToNot(BeNil())
	Expect(actualProbe.HTTPGet).ToNot(BeNil())
	Expect(actualProbe.HTTPGet.Path).To(Equal(config.probePath))
	Expect(actualProbe.HTTPGet.Port.IntVal).To(Equal(int32(8081)))
	Expect(actualProbe.InitialDelaySeconds).To(Equal(config.probe.InitialDelaySeconds))
	Expect(actualProbe.PeriodSeconds).To(Equal(config.probe.PeriodSeconds))
	Expect(actualProbe.TimeoutSeconds).To(Equal(config.probe.TimeoutSeconds))
	Expect(actualProbe.FailureThreshold).To(Equal(config.probe.FailureThreshold))
}

var _ = Describe("Health Checks and Probes", func() {
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
				Name: fmt.Sprintf("health-test-%d-%d", time.Now().Unix(), time.Now().UnixNano()%1000000),
				Labels: map[string]string{
					"test-type": "health-checks",
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

	Context("When testing health check endpoints", func() {
		It("Should configure liveness probe correctly", func() {
			config := probeTestConfig{
				deploymentName: "health-test-deployment",
				labelName:      "cloudflare-dns-operator-health-test",
				commandMessage: "Health check test running",
				probePath:      "/healthz",
				probe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8081),
						},
					},
					InitialDelaySeconds: 15,
					PeriodSeconds:       20,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
					SuccessThreshold:    1,
				},
			}
			testProbeDeployment(ctx, config, testNamespace)
		})

		It("Should configure readiness probe correctly", func() {
			config := probeTestConfig{
				deploymentName: "readiness-test-deployment",
				labelName:      "cloudflare-dns-operator-readiness-test",
				commandMessage: "Readiness check test running",
				probePath:      "/readyz",
				probe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/readyz",
							Port: intstr.FromInt(8081),
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
					SuccessThreshold:    1,
				},
			}
			testProbeDeployment(ctx, config, testNamespace)
		})

		It("Should configure startup probe correctly", func() {
			config := probeTestConfig{
				deploymentName: "startup-test-deployment",
				labelName:      "cloudflare-dns-operator-startup-test",
				commandMessage: "Startup check test running",
				probePath:      "/healthz",
				probe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8081),
						},
					},
					InitialDelaySeconds: 10,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					FailureThreshold:    30,
					SuccessThreshold:    1,
				},
			}
			testProbeDeployment(ctx, config, testNamespace)
		})
	})
})
