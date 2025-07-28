package health

import (
	"net/http/httptest"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestChecker_LivenessCheck_WithNilManager(t *testing.T) {
	checker := &Checker{
		manager: nil,
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	err := checker.LivenessCheck(req)

	if err == nil {
		t.Error("expected error when manager is nil, got nil")
	}

	if err != nil && err.Error() != "manager is not initialized" {
		t.Errorf("expected 'manager is not initialized', got %s", err.Error())
	}
}

func TestChecker_LivenessCheck_WithNilK8sClient(t *testing.T) {
	checker := &Checker{
		k8sClient: nil,
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	err := checker.LivenessCheck(req)

	if err == nil {
		t.Error("expected error when k8s client is nil, got nil")
	}
}

func TestChecker_ReadinessCheck_WithNilDiscoveryClient(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()

	checker := &Checker{
		k8sClient:       k8sClient,
		discoveryClient: nil,
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	err := checker.ReadinessCheck(req)

	if err == nil {
		t.Error("expected error when discovery client is nil, got nil")
	}
}

func TestChecker_GetHealthzHandler(t *testing.T) {
	checker := &Checker{}

	handler := checker.GetHealthzHandler()

	if handler == nil {
		t.Error("expected non-nil handler, got nil")
	}
}

func TestChecker_GetReadyzHandler(t *testing.T) {
	checker := &Checker{}

	handler := checker.GetReadyzHandler()

	if handler == nil {
		t.Error("expected non-nil handler, got nil")
	}
}
