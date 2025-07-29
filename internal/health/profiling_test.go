package health

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupProfiling(t *testing.T) {
	t.Run("profiling disabled by default", func(t *testing.T) {
		// Clean environment
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Unsetenv("GO_ENV")

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Test that pprof endpoints are not registered
		req := httptest.NewRequest("GET", "/debug/pprof/", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code, "pprof should be disabled by default")
	})

	t.Run("profiling enabled via ENABLE_PPROF=true", func(t *testing.T) {
		// Set environment variable
		_ = os.Setenv("ENABLE_PPROF", "true")
		defer func() { _ = os.Unsetenv("ENABLE_PPROF") }()

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Test that pprof index endpoint is registered
		req := httptest.NewRequest("GET", "/debug/pprof/", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code, "pprof index should be available when enabled")
		assert.Contains(t, recorder.Body.String(), "pprof", "Response should contain pprof content")
	})

	t.Run("profiling enabled via GO_ENV=development", func(t *testing.T) {
		// Clean ENABLE_PPROF and set GO_ENV
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Setenv("GO_ENV", "development")
		defer func() { _ = os.Unsetenv("GO_ENV") }()

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Test multiple pprof endpoints
		endpoints := []string{
			"/debug/pprof/",
			"/debug/pprof/cmdline",
			"/debug/pprof/goroutine",
			"/debug/pprof/heap",
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code, "pprof endpoint %s should be available in development", endpoint)
		}
	})

	t.Run("profiling disabled via ENABLE_PPROF=false", func(t *testing.T) {
		// Explicitly disable
		_ = os.Setenv("ENABLE_PPROF", "false")
		defer func() { _ = os.Unsetenv("ENABLE_PPROF") }()

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Test that pprof endpoints are not registered
		req := httptest.NewRequest("GET", "/debug/pprof/", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code, "pprof should be disabled when ENABLE_PPROF=false")
	})

	t.Run("all pprof endpoints registered when enabled", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "true")
		defer func() { _ = os.Unsetenv("ENABLE_PPROF") }()

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Test all expected pprof endpoints
		endpoints := []struct {
			path        string
			description string
		}{
			{"/debug/pprof/", "index"},
			{"/debug/pprof/cmdline", "command line"},
			{"/debug/pprof/goroutine", "goroutines"},
			{"/debug/pprof/heap", "heap"},
			{"/debug/pprof/threadcreate", "thread creation"},
			{"/debug/pprof/block", "blocking"},
			{"/debug/pprof/mutex", "mutex"},
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint.path, nil)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code, "pprof %s endpoint should be available", endpoint.description)
		}
	})
}

func TestIsPprofEnabled(t *testing.T) {
	// Save original environment
	originalEnablePprof := os.Getenv("ENABLE_PPROF")
	originalGoEnv := os.Getenv("GO_ENV")
	defer func() {
		if originalEnablePprof != "" {
			_ = os.Setenv("ENABLE_PPROF", originalEnablePprof)
		} else {
			_ = os.Unsetenv("ENABLE_PPROF")
		}
		if originalGoEnv != "" {
			_ = os.Setenv("GO_ENV", originalGoEnv)
		} else {
			_ = os.Unsetenv("GO_ENV")
		}
	}()

	t.Run("default disabled", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Unsetenv("GO_ENV")

		assert.False(t, isPprofEnabled(), "Should be disabled by default")
	})

	t.Run("enabled via ENABLE_PPROF=true", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "true")
		_ = os.Unsetenv("GO_ENV")

		assert.True(t, isPprofEnabled(), "Should be enabled when ENABLE_PPROF=true")
	})

	t.Run("enabled via ENABLE_PPROF=1", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "1")
		_ = os.Unsetenv("GO_ENV")

		assert.True(t, isPprofEnabled(), "Should be enabled when ENABLE_PPROF=1")
	})

	t.Run("disabled via ENABLE_PPROF=false", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "false")
		_ = os.Unsetenv("GO_ENV")

		assert.False(t, isPprofEnabled(), "Should be disabled when ENABLE_PPROF=false")
	})

	t.Run("disabled via ENABLE_PPROF=0", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "0")
		_ = os.Unsetenv("GO_ENV")

		assert.False(t, isPprofEnabled(), "Should be disabled when ENABLE_PPROF=0")
	})

	t.Run("enabled via GO_ENV=development", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Setenv("GO_ENV", "development")

		assert.True(t, isPprofEnabled(), "Should be enabled when GO_ENV=development")
	})

	t.Run("disabled via GO_ENV=production", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Setenv("GO_ENV", "production")

		assert.False(t, isPprofEnabled(), "Should be disabled when GO_ENV=production")
	})

	t.Run("ENABLE_PPROF takes precedence over GO_ENV", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "false")
		_ = os.Setenv("GO_ENV", "development")

		assert.False(t, isPprofEnabled(), "ENABLE_PPROF=false should override GO_ENV=development")
	})

	t.Run("invalid ENABLE_PPROF value falls back to GO_ENV", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "invalid")
		_ = os.Setenv("GO_ENV", "development")

		assert.True(t, isPprofEnabled(), "Invalid ENABLE_PPROF should fall back to GO_ENV check")
	})

	t.Run("invalid ENABLE_PPROF value with no GO_ENV", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "invalid")
		_ = os.Unsetenv("GO_ENV")

		assert.False(t, isPprofEnabled(), "Invalid ENABLE_PPROF with no GO_ENV should default to false")
	})
}

func TestProfilingIntegration(t *testing.T) {
	t.Run("profiling setup with real mux", func(t *testing.T) {
		_ = os.Setenv("ENABLE_PPROF", "true")
		defer func() { _ = os.Unsetenv("ENABLE_PPROF") }()

		// Use the default HTTP mux
		originalMux := http.DefaultServeMux
		http.DefaultServeMux = http.NewServeMux()
		defer func() { http.DefaultServeMux = originalMux }()

		SetupProfiling(http.DefaultServeMux)

		// Create test server
		server := httptest.NewServer(http.DefaultServeMux)
		defer server.Close()

		// Test that we can access pprof endpoints
		resp, err := http.Get(server.URL + "/debug/pprof/")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	})

	t.Run("profiling security - disabled in production-like env", func(t *testing.T) {
		// Simulate production environment
		_ = os.Unsetenv("ENABLE_PPROF")
		_ = os.Setenv("GO_ENV", "production")
		defer func() { _ = os.Unsetenv("GO_ENV") }()

		mux := http.NewServeMux()
		SetupProfiling(mux)

		// Verify pprof is not accessible
		req := httptest.NewRequest("GET", "/debug/pprof/", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code, "pprof should not be accessible in production")
	})
}
