package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"
)

// Simple mock for CloudflareAPI
type SimpleMockCloudflareAPI struct {
	mock.Mock
}

func (m *SimpleMockCloudflareAPI) VerifyAPIToken(ctx context.Context) (cloudflare.APITokenVerifyBody, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return cloudflare.APITokenVerifyBody{}, args.Error(1)
	}
	return args.Get(0).(cloudflare.APITokenVerifyBody), args.Error(1)
}

func TestChecker_checkKubernetesAPI_Simple(t *testing.T) {
	tests := []struct {
		name        string
		nilClient   bool
		expectError bool
	}{
		{
			name:        "nil client should error",
			nilClient:   true,
			expectError: true,
		},
		{
			name:        "valid client should succeed",
			nilClient:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &Checker{}
			if !tt.nilClient {
				checker.k8sClient = fake.NewSimpleClientset()
			}

			err := checker.checkKubernetesAPI(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "kubernetes client not initialized")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChecker_checkCloudflareAPI_Simple(t *testing.T) {
	tests := []struct {
		name           string
		setupAPI       func() CloudflareAPIInterface
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "nil API client",
			setupAPI: func() CloudflareAPIInterface {
				return nil
			},
			expectError:    true,
			expectedErrMsg: "cloudflare API client not initialized",
		},
		{
			name: "API returns error",
			setupAPI: func() CloudflareAPIInterface {
				mockAPI := &SimpleMockCloudflareAPI{}
				mockAPI.On("VerifyAPIToken", mock.Anything).Return(cloudflare.APITokenVerifyBody{}, errors.New("network error"))
				return mockAPI
			},
			expectError:    true,
			expectedErrMsg: "failed to verify API token",
		},
		{
			name: "API token inactive",
			setupAPI: func() CloudflareAPIInterface {
				mockAPI := &SimpleMockCloudflareAPI{}
				mockAPI.On("VerifyAPIToken", mock.Anything).Return(cloudflare.APITokenVerifyBody{
					Status: "inactive",
				}, nil)
				return mockAPI
			},
			expectError:    true,
			expectedErrMsg: "API token is not active",
		},
		{
			name: "API token active",
			setupAPI: func() CloudflareAPIInterface {
				mockAPI := &SimpleMockCloudflareAPI{}
				mockAPI.On("VerifyAPIToken", mock.Anything).Return(cloudflare.APITokenVerifyBody{
					Status: "active",
				}, nil)
				return mockAPI
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &Checker{
				cloudflareAPI: tt.setupAPI(),
			}

			err := checker.checkCloudflareAPI(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChecker_GetHandlers_Simple(t *testing.T) {
	checker := &Checker{}

	t.Run("GetHealthzHandler", func(t *testing.T) {
		handler := checker.GetHealthzHandler()
		assert.NotNil(t, handler)

		// Test the returned handler
		req := httptest.NewRequest("GET", "/healthz", nil)
		err := handler(req)
		// Should error because no manager is set
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "manager is not initialized")
	})

	t.Run("GetReadyzHandler", func(t *testing.T) {
		handler := checker.GetReadyzHandler()
		assert.NotNil(t, handler)

		// Test the returned handler
		req := httptest.NewRequest("GET", "/readyz", nil)
		err := handler(req)
		// Should error because no k8s client is set
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubernetes API not ready")
	})
}
