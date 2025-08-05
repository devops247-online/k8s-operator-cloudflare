package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // ginkgo DSL requires dot import
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// cloudflareoperatorv1alpha1 "github.com/devops247-online/k8s-operator-cloudflare/api/v1alpha1"
)

// E2ETestSuite encapsulates all the information for e2e tests
type E2ETestSuite struct {
	k8sClient         client.Client
	testEnv           *envtest.Environment
	operatorNamespace string
	ctx               context.Context
	cancel            context.CancelFunc
}

var testSuite *E2ETestSuite

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel := context.WithCancel(context.TODO())

	testSuite = &E2ETestSuite{
		ctx:               ctx,
		cancel:            cancel,
		operatorNamespace: "cloudflare-system",
	}

	By("bootstrapping test environment")
	testSuite.testEnv = &envtest.Environment{
		UseExistingCluster:       func() *bool { b := true; return &b }(),
		AttachControlPlaneOutput: true,
	}

	cfg, err := testSuite.testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// err = cloudflareoperatorv1alpha1.AddToScheme(scheme)
	// Expect(err).NotTo(HaveOccurred())

	// Use existing cluster configuration
	restConfig, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	testSuite.k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(testSuite.k8sClient).NotTo(BeNil())

	// Create namespace for operator if it doesn't exist
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testSuite.operatorNamespace,
		},
	}
	err = testSuite.k8sClient.Create(ctx, ns)
	if err != nil && client.IgnoreAlreadyExists(err) != nil {
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	if testSuite != nil {
		testSuite.cancel()
		By("tearing down the test environment")
		err := testSuite.testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

// Helper methods for test suite
func (suite *E2ETestSuite) T() GinkgoTInterface {
	return GinkgoT()
}

// Simplified port forward - in real implementation would use kubectl port-forward
func (suite *E2ETestSuite) portForward(_ string, _, _ int, stopCh <-chan struct{}, readyCh chan<- struct{}) {
	// Simulate port forward being ready immediately for testing
	close(readyCh)
	<-stopCh
}
