package health

import (
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
)

// SetupProfiling configures pprof endpoints for debugging
func SetupProfiling(mux *http.ServeMux) {
	// Only enable pprof in development or when explicitly enabled
	if !isPprofEnabled() {
		return
	}

	// Register pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Register additional handlers
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
}

// isPprofEnabled checks if pprof profiling should be enabled
func isPprofEnabled() bool {
	// Check environment variable
	if enabled := os.Getenv("ENABLE_PPROF"); enabled != "" {
		if val, err := strconv.ParseBool(enabled); err == nil {
			return val
		}
	}

	// Check if running in development mode
	if env := os.Getenv("GO_ENV"); env == "development" {
		return true
	}

	// Default to disabled for security
	return false
}
