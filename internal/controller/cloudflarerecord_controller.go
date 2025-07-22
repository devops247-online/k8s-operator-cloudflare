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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1 "github.com/example/cloudflare-dns-operator/api/v1"
)

const (
	// CloudflareRecordFinalizer is the finalizer added to CloudflareRecord resources
	CloudflareRecordFinalizer = "dns.cloudflare.io/finalizer"
)

// CloudflareRecordReconciler reconciles a CloudflareRecord object
type CloudflareRecordReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cloudflare.io,resources=cloudflarerecords/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CloudflareRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the CloudflareRecord instance
	var cloudflareRecord dnsv1.CloudflareRecord
	if err := r.Get(ctx, req.NamespacedName, &cloudflareRecord); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("CloudflareRecord resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CloudflareRecord")
		return ctrl.Result{}, err
	}

	// Check if the CloudflareRecord instance is marked to be deleted
	if cloudflareRecord.GetDeletionTimestamp() != nil {
		return r.reconcileDelete(ctx, &cloudflareRecord)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&cloudflareRecord, CloudflareRecordFinalizer) {
		controllerutil.AddFinalizer(&cloudflareRecord, CloudflareRecordFinalizer)
		if err := r.Update(ctx, &cloudflareRecord); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Log the DNS record details
	log.Info("Processing CloudflareRecord",
		"zone", cloudflareRecord.Spec.Zone,
		"type", cloudflareRecord.Spec.Type,
		"name", cloudflareRecord.Spec.Name,
		"content", cloudflareRecord.Spec.Content)

	// TODO: Implement full Cloudflare API integration here
	// For now, just update status to show operator is working

	// Update status to indicate processing
	r.updateStatus(ctx, &cloudflareRecord, true, dnsv1.ConditionReasonRecordCreated, "DNS record processing completed (implementation ready for Cloudflare API)")

	cloudflareRecord.Status.ObservedGeneration = cloudflareRecord.Generation
	now := metav1.NewTime(time.Now())
	cloudflareRecord.Status.LastUpdated = &now

	if err := r.Status().Update(ctx, &cloudflareRecord); err != nil {
		log.Error(err, "Failed to update CloudflareRecord status")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Requeue after 5 minutes for periodic sync
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// reconcileDelete handles the deletion of CloudflareRecord
func (r *CloudflareRecordReconciler) reconcileDelete(ctx context.Context, cloudflareRecord *dnsv1.CloudflareRecord) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Deleting CloudflareRecord", "name", cloudflareRecord.Name)

	// TODO: In a full implementation, delete the DNS record from Cloudflare here

	// Remove finalizer
	controllerutil.RemoveFinalizer(cloudflareRecord, CloudflareRecordFinalizer)
	if err := r.Update(ctx, cloudflareRecord); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the status of the CloudflareRecord
func (r *CloudflareRecordReconciler) updateStatus(ctx context.Context, cloudflareRecord *dnsv1.CloudflareRecord, ready bool, reason, message string) {
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
	updated := false
	for i, existingCondition := range cloudflareRecord.Status.Conditions {
		if existingCondition.Type == dnsv1.ConditionTypeReady {
			if existingCondition.Status != condition.Status || existingCondition.Reason != condition.Reason {
				cloudflareRecord.Status.Conditions[i] = condition
				updated = true
			}
			break
		}
	}

	if !updated {
		cloudflareRecord.Status.Conditions = append(cloudflareRecord.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CloudflareRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1.CloudflareRecord{}).
		Named("cloudflarerecord").
		Complete(r)
}
