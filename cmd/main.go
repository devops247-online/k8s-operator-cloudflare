/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	dnsv1 "github.com/devops247-online/k8s-operator-cloudflare/api/v1"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/config"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/controller"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/health"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/logging"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/tracing"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dnsv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)

	// Leader election configuration
	var leaderElectLeaseDuration time.Duration
	var leaderElectRenewDeadline time.Duration
	var leaderElectRetryPeriod time.Duration
	var leaderElectResourceName string
	var leaderElectResourceNamespace string
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	// High Availability leader election flags
	flag.DurationVar(&leaderElectLeaseDuration, "leader-elect-lease-duration", 15*time.Second,
		"Duration that leader election lease is held (default 15s)")
	flag.DurationVar(&leaderElectRenewDeadline, "leader-elect-renew-deadline", 10*time.Second,
		"Duration that leader must renew the lease before losing it (default 10s)")
	flag.DurationVar(&leaderElectRetryPeriod, "leader-elect-retry-period", 2*time.Second,
		"Duration between retry attempts for leader election (default 2s)")
	flag.StringVar(&leaderElectResourceName, "leader-elect-resource-name", "cloudflare-dns-operator-leader",
		"Name of the resource used for leader election")
	flag.StringVar(&leaderElectResourceNamespace, "leader-elect-resource-namespace", "",
		"Namespace for leader election resource (empty means same as deployment namespace)")

	// Add structured logging flags
	var logLevel string
	var logFormat string
	var enableTracing bool
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&logFormat, "log-format", "json", "Log format (json, console)")
	flag.BoolVar(&enableTracing, "enable-tracing", false, "Enable distributed tracing")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Determine environment
	environment := os.Getenv("OPERATOR_ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	// Setup structured logging
	loggingConfig := logging.NewEnvironmentConfig(environment)
	if logLevel != "" {
		loggingConfig.Level = logLevel
	}
	if logFormat != "" {
		loggingConfig.Format = logFormat
	}

	// Override with environment variables
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		loggingConfig.Level = envLevel
	}
	if envFormat := os.Getenv("LOG_FORMAT"); envFormat != "" {
		loggingConfig.Format = envFormat
	}

	// Setup global logger
	err := logging.SetupGlobalLogger(loggingConfig)
	if err != nil {
		setupLog.Error(err, "Failed to setup structured logging, falling back to default")
		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	} else {
		// Create logr logger from our structured logger
		logrLogger, err := logging.LogrFromConfig(environment)
		if err != nil {
			setupLog.Error(err, "Failed to create logr logger, falling back to default")
			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
		} else {
			ctrl.SetLogger(logrLogger)
		}
	}

	// Setup distributed tracing
	var tracingProvider *tracing.Provider
	tracingConfig := tracing.NewEnvironmentConfig(environment)

	// Override tracing enabled from flags or env
	if enableTracing {
		tracingConfig.Enabled = true
	}
	if envTracing := os.Getenv("TRACING_ENABLED"); envTracing != "" {
		tracingConfig.Enabled = envTracing == "true"
	}

	if tracingConfig.Enabled {
		tracingProvider, err = tracing.SetupGlobalTracer(tracingConfig)
		if err != nil {
			setupLog.Error(err, "Failed to setup distributed tracing")
		} else {
			setupLog.Info("Distributed tracing initialized successfully",
				"service", tracingConfig.GetServiceName(),
				"exporter", tracingConfig.Exporter.Type)
		}
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	// Configure leader election resource name and namespace
	leaderElectionID := leaderElectResourceName
	if leaderElectionID == "" {
		leaderElectionID = "085fc383.cloudflare.io"
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsServerOptions,
		WebhookServer:           webhookServer,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectResourceNamespace,
		LeaseDuration:           &leaderElectLeaseDuration,
		RenewDeadline:           &leaderElectRenewDeadline,
		RetryPeriod:             &leaderElectRetryPeriod,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize configuration manager
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = DefaultNamespace
	}
	configManager := config.NewConfigManager(mgr.GetClient(), namespace)

	// Load configuration with comprehensive options
	ctx := context.Background()
	configMapName := os.Getenv("CONFIG_CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = DefaultConfigMapName
	}

	configEnv := os.Getenv("OPERATOR_ENVIRONMENT")
	if configEnv == "" {
		configEnv = ProductionEnvironment
	}

	loadOptions := config.LoadOptions{
		LoadFromEnv:    true,
		ValidateConfig: true,
		ConfigMapName:  configMapName,
		SecretName:     os.Getenv("CONFIG_SECRET_NAME"),
	}

	operatorConfig, err := configManager.LoadConfig(ctx, loadOptions)
	if err != nil {
		setupLog.Error(err, "failed to load operator configuration")
		// Fall back to default configuration
		setupLog.Info("Using default configuration", "environment", configEnv)
		operatorConfig = config.GetEnvironmentDefaults(configEnv) // nolint:staticcheck // False positive SA4006
	} else {
		setupLog.Info("Configuration loaded successfully",
			"environment", operatorConfig.Environment,
			"logLevel", operatorConfig.Operator.LogLevel,
			"reconcileInterval", operatorConfig.Operator.ReconcileInterval,
			"source", "configmap/env")
	}

	// Create controller with performance configuration
	cloudflareReconciler := controller.NewCloudflareRecordReconciler(mgr.GetClient(), mgr.GetScheme(), configManager)
	if err := cloudflareReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CloudflareRecord")
		os.Exit(1)
	}

	// Start hot-reload configuration watcher if ConfigMap is specified
	if configMapName != "" {
		go func() {
			setupLog.Info("Starting configuration hot-reload watcher", "configMapName", configMapName)
			watchCtx, watchCancel := context.WithCancel(ctx)
			defer watchCancel()

			// Create a config loader for watching
			configLoader := config.NewConfigLoader(mgr.GetClient(), namespace)

			watchOptions := config.WatchOptions{
				ConfigMapName: configMapName,
				SecretName:    loadOptions.SecretName,
				Interval:      30 * time.Second, // Check every 30 seconds
			}

			configCh, errCh := configLoader.WatchConfig(watchCtx, watchOptions)

			for {
				select {
				case newConfig := <-configCh:
					if newConfig != nil {
						setupLog.Info("Configuration reloaded",
							"environment", newConfig.Environment,
							"logLevel", newConfig.Operator.LogLevel,
							"reconcileInterval", newConfig.Operator.ReconcileInterval)

						// Trigger reload of config manager to update internal state
						_, err := configManager.ReloadConfig(watchCtx)
						if err != nil {
							setupLog.Error(err, "Failed to reload config manager")
						} else {
							setupLog.Info("Controller performance configuration updated")
						}
					}
				case err := <-errCh:
					if err != nil {
						setupLog.Error(err, "Configuration hot-reload error")
					}
				case <-watchCtx.Done():
					setupLog.Info("Configuration hot-reload watcher stopped")
					return
				}
			}
		}()
	}
	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	// Initialize comprehensive health checker
	healthChecker := health.NewChecker(mgr, nil) // TODO: Add CloudflareAPI when available

	if err := mgr.AddHealthzCheck("healthz", healthChecker.GetHealthzHandler()); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthChecker.GetReadyzHandler()); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Setup profiling endpoints (enabled only in development)
	health.SetupProfiling(http.DefaultServeMux)

	setupLog.Info("starting manager")

	// Setup graceful shutdown for tracing
	if tracingProvider != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tracingProvider.Shutdown(shutdownCtx); err != nil {
				setupLog.Error(err, "Failed to shutdown tracing provider")
			} else {
				setupLog.Info("Tracing provider shutdown successfully")
			}
		}()
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
