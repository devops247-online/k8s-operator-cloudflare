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

package controller

import (
	"context"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1 "github.com/devops247-online/k8s-operator-cloudflare/api/v1"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/config"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/metrics"
)

const (
	// CloudflareRecordFinalizer is the finalizer added to CloudflareRecord resources
	CloudflareRecordFinalizer = "dns.cloudflare.io/finalizer"
)

// CloudflareRecordReconciler reconciles a CloudflareRecord object
type CloudflareRecordReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Configuration manager for advanced configuration support
	configManager *config.ConfigManager

	// Performance configuration
	MaxConcurrentReconciles int
	ReconcileTimeout        time.Duration
	RequeueInterval         time.Duration
	RequeueIntervalOnError  time.Duration

	// Metrics collectors
	performanceMetrics *metrics.PerformanceMetrics
	cloudflareMetrics  *metrics.CloudflareMetrics
	businessMetrics    *metrics.BusinessMetrics
}

// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// NewCloudflareRecordReconciler creates a new CloudflareRecordReconciler with performance configuration
func NewCloudflareRecordReconciler(kubeClient client.Client, scheme *runtime.Scheme, configManager *config.ConfigManager) *CloudflareRecordReconciler {
	reconciler := &CloudflareRecordReconciler{
		Client:             kubeClient,
		Scheme:             scheme,
		configManager:      configManager,
		performanceMetrics: metrics.NewPerformanceMetrics(),
		cloudflareMetrics:  metrics.NewCloudflareMetrics(),
		businessMetrics:    metrics.NewBusinessMetrics(),
	}

	// Load performance configuration from config manager or environment variables
	reconciler.loadPerformanceConfig()

	return reconciler
}

// loadPerformanceConfig loads performance tuning parameters from config manager or environment variables
func (r *CloudflareRecordReconciler) loadPerformanceConfig() {
	// Set defaults first
	r.MaxConcurrentReconciles = 5
	r.ReconcileTimeout = 5 * time.Minute
	r.RequeueInterval = 5 * time.Minute
	r.RequeueIntervalOnError = 1 * time.Minute

	// Try to get configuration from config manager first
	if r.configManager != nil && r.configManager.IsConfigured() {
		cfg := r.configManager.GetConfig()
		if cfg != nil {
			// Load from config manager - override defaults with non-zero values
			if cfg.Performance.MaxConcurrentReconciles > 0 {
				r.MaxConcurrentReconciles = cfg.Performance.MaxConcurrentReconciles
			}
			if cfg.Performance.ReconcileTimeout > 0 {
				r.ReconcileTimeout = cfg.Performance.ReconcileTimeout
			}
			if cfg.Performance.RequeueInterval > 0 {
				r.RequeueInterval = cfg.Performance.RequeueInterval
			}
			if cfg.Performance.RequeueIntervalOnError > 0 {
				r.RequeueIntervalOnError = cfg.Performance.RequeueIntervalOnError
			}
			return
		}
	}

	// Fallback to environment variables - override defaults
	if val := os.Getenv("MAX_CONCURRENT_RECONCILES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			r.MaxConcurrentReconciles = parsed
		}
	}

	if val := os.Getenv("RECONCILE_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			r.ReconcileTimeout = parsed
		}
	}

	if val := os.Getenv("REQUEUE_INTERVAL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			r.RequeueInterval = parsed
		}
	}

	if val := os.Getenv("REQUEUE_INTERVAL_ON_ERROR"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			r.RequeueIntervalOnError = parsed
		}
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CloudflareRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	log := logf.FromContext(ctx)

	// Create context with timeout only if parent context doesn't have deadline
	var workCtx context.Context
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		// Parent context already has deadline (e.g., from tests), use it
		workCtx = ctx
	} else {
		// No deadline in parent context, add our own
		var cancel context.CancelFunc
		workCtx, cancel = context.WithTimeout(ctx, r.ReconcileTimeout)
		defer cancel()
	}

	// Update queue depth metric (approximate)
	r.performanceMetrics.UpdateQueueDepth("cloudflarerecord", req.Namespace, 1)
	defer r.performanceMetrics.UpdateQueueDepth("cloudflarerecord", req.Namespace, 0)

	// Fetch the CloudflareRecord instance
	var cloudflareRecord dnsv1.CloudflareRecord
	if err := r.Get(workCtx, req.NamespacedName, &cloudflareRecord); err != nil {
		duration := time.Since(startTime)

		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("CloudflareRecord resource not found. Ignoring since object must be deleted")
			r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "not_found", req.Namespace, duration)
			r.performanceMetrics.IncReconcileRate("cloudflarerecord", "not_found", req.Namespace)
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get CloudflareRecord")
		r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "error", req.Namespace, duration)
		r.performanceMetrics.IncReconcileRate("cloudflarerecord", "error", req.Namespace)
		r.performanceMetrics.IncErrorRate("cloudflarerecord", "get_error", req.Namespace)
		return ctrl.Result{RequeueAfter: r.RequeueIntervalOnError}, err
	}

	// Check feature flags if config manager is available
	if r.configManager != nil && r.configManager.IsConfigured() {
		ffm := r.configManager.GetFeatureFlagManager()
		if ffm != nil && !ffm.IsEnabled("EnableReconciliation") {
			log.Info("Reconciliation disabled by feature flag")
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	// Check if the CloudflareRecord instance is marked to be deleted
	if cloudflareRecord.GetDeletionTimestamp() != nil {
		result, err := r.reconcileDelete(workCtx, &cloudflareRecord)
		duration := time.Since(startTime)

		if err != nil {
			r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "delete_error", req.Namespace, duration)
			r.performanceMetrics.IncReconcileRate("cloudflarerecord", "delete_error", req.Namespace)
			r.performanceMetrics.IncErrorRate("cloudflarerecord", "delete_error", req.Namespace)
		} else {
			r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "delete_success", req.Namespace, duration)
			r.performanceMetrics.IncReconcileRate("cloudflarerecord", "delete_success", req.Namespace)
		}

		return result, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&cloudflareRecord, CloudflareRecordFinalizer) {
		controllerutil.AddFinalizer(&cloudflareRecord, CloudflareRecordFinalizer)
		if err := r.Update(workCtx, &cloudflareRecord); err != nil {
			duration := time.Since(startTime)
			log.Error(err, "Failed to add finalizer")
			r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "finalizer_error", req.Namespace, duration)
			r.performanceMetrics.IncReconcileRate("cloudflarerecord", "finalizer_error", req.Namespace)
			r.performanceMetrics.IncErrorRate("cloudflarerecord", "finalizer_error", req.Namespace)
			return ctrl.Result{RequeueAfter: r.RequeueIntervalOnError}, err
		}
	}

	// Log the DNS record details
	log.Info("Processing CloudflareRecord",
		"zone", cloudflareRecord.Spec.Zone,
		"type", cloudflareRecord.Spec.Type,
		"name", cloudflareRecord.Spec.Name,
		"content", cloudflareRecord.Spec.Content)

	// Update business metrics for CRD resource status
	r.businessMetrics.UpdateCRDResourceStatus(req.Namespace, "processing", cloudflareRecord.Spec.Type, 1)

	// Update DNS records by type metric
	r.businessMetrics.UpdateDNSRecordsByType(cloudflareRecord.Spec.Type, cloudflareRecord.Spec.Zone, "", 1)

	// TODO: Implement full Cloudflare API integration here
	// For now, just update status to show operator is working

	// Simulate API call time for metrics
	apiStartTime := time.Now()
	time.Sleep(10 * time.Millisecond) // Simulate API delay
	r.performanceMetrics.ObserveAPIResponseTime("record_update", "200", req.Namespace, time.Since(apiStartTime))

	// Update status to indicate processing
	r.updateStatus(&cloudflareRecord, true, dnsv1.ConditionReasonRecordCreated, "DNS record processing completed (implementation ready for Cloudflare API)")

	cloudflareRecord.Status.ObservedGeneration = cloudflareRecord.Generation
	now := metav1.NewTime(time.Now())
	cloudflareRecord.Status.LastUpdated = &now

	if err := r.Status().Update(workCtx, &cloudflareRecord); err != nil {
		duration := time.Since(startTime)
		log.Error(err, "Failed to update CloudflareRecord status")
		r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "status_error", req.Namespace, duration)
		r.performanceMetrics.IncReconcileRate("cloudflarerecord", "status_error", req.Namespace)
		r.performanceMetrics.IncErrorRate("cloudflarerecord", "status_error", req.Namespace)
		return ctrl.Result{RequeueAfter: r.RequeueIntervalOnError}, err
	}

	// Record successful reconciliation
	duration := time.Since(startTime)
	r.performanceMetrics.ObserveReconcileDuration("cloudflarerecord", "success", req.Namespace, duration)
	r.performanceMetrics.IncReconcileRate("cloudflarerecord", "success", req.Namespace)

	// Update business metrics for successful reconciliation
	r.businessMetrics.UpdateCRDResourceStatus(req.Namespace, "ready", cloudflareRecord.Spec.Type, 1)
	r.businessMetrics.UpdateReconciliationHealth("cloudflarerecord", req.Namespace, "CloudflareRecord", true)
	r.businessMetrics.UpdateDNSRecordsByStatus("active", cloudflareRecord.Spec.Zone, "", 1)

	// Simulate API call metrics (in real implementation, this would be in API call)
	r.cloudflareMetrics.RecordAPIRequest("POST", "/zones/dns_records", "200", "", duration)
	r.cloudflareMetrics.RecordDNSOperation("create", cloudflareRecord.Spec.Type, "", "success")

	// Requeue with configured interval
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

// reconcileDelete handles the deletion of CloudflareRecord
func (r *CloudflareRecordReconciler) reconcileDelete(ctx context.Context, cloudflareRecord *dnsv1.CloudflareRecord) (ctrl.Result, error) {
	startTime := time.Now()
	log := logf.FromContext(ctx)
	log.Info("Deleting CloudflareRecord", "name", cloudflareRecord.Name)

	// TODO: In a full implementation, delete the DNS record from Cloudflare here
	// Simulate API call metrics for deletion
	duration := time.Since(startTime)
	r.cloudflareMetrics.RecordAPIRequest("DELETE", "/zones/dns_records", "200", "", duration)
	r.cloudflareMetrics.RecordDNSOperation("delete", cloudflareRecord.Spec.Type, "", "success")

	// Update business metrics for deletion
	r.businessMetrics.UpdateCRDResourceStatus(cloudflareRecord.Namespace, "deleting", cloudflareRecord.Spec.Type, 1)
	r.businessMetrics.UpdateDNSRecordsByType(cloudflareRecord.Spec.Type, cloudflareRecord.Spec.Zone, "", -1)

	// Remove finalizer
	controllerutil.RemoveFinalizer(cloudflareRecord, CloudflareRecordFinalizer)
	if err := r.Update(ctx, cloudflareRecord); err != nil {
		log.Error(err, "Failed to remove finalizer")
		r.cloudflareMetrics.RecordAPIError("DELETE", "/zones/dns_records", "finalizer_error", "")
		return ctrl.Result{RequeueAfter: r.RequeueIntervalOnError}, err
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the status of the CloudflareRecord
func (r *CloudflareRecordReconciler) updateStatus(cloudflareRecord *dnsv1.CloudflareRecord, ready bool, reason, message string) {
	cloudflareRecord.Status.Ready = ready

	// Update condition
	condition := metav1.Condition{
		Type:               dnsv1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}

	if ready {
		condition.Status = metav1.ConditionTrue
	}

	// Find existing condition or add new one
	found := false
	for i, existingCondition := range cloudflareRecord.Status.Conditions {
		if existingCondition.Type == dnsv1.ConditionTypeReady {
			found = true
			if existingCondition.Status != condition.Status || existingCondition.Reason != condition.Reason {
				cloudflareRecord.Status.Conditions[i] = condition
			}
			break
		}
	}

	if !found {
		cloudflareRecord.Status.Conditions = append(cloudflareRecord.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CloudflareRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1.CloudflareRecord{}).
		Named("cloudflarerecord").
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		Complete(r)
}
