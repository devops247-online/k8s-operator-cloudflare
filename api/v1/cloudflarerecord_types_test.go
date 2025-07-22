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

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCloudflareRecordSpec_Validation(t *testing.T) {
	tests := []struct {
		name     string
		spec     CloudflareRecordSpec
		expected bool // whether the spec should be considered valid structurally
	}{
		{
			name: "valid A record",
			spec: CloudflareRecordSpec{
				Zone:    "example.com",
				Type:    "A",
				Name:    "test.example.com",
				Content: "192.168.1.100",
				TTL:     ptr.To(3600),
				CloudflareCredentialsSecretRef: SecretReference{
					Name: "test-secret",
				},
			},
			expected: true,
		},
		{
			name: "valid CNAME record with proxied",
			spec: CloudflareRecordSpec{
				Zone:    "example.com",
				Type:    "CNAME",
				Name:    "www.example.com",
				Content: "example.com",
				Proxied: ptr.To(true),
				CloudflareCredentialsSecretRef: SecretReference{
					Name: "test-secret",
				},
			},
			expected: true,
		},
		{
			name: "valid MX record with priority",
			spec: CloudflareRecordSpec{
				Zone:     "example.com",
				Type:     "MX",
				Name:     "example.com",
				Content:  "mail.example.com",
				Priority: ptr.To(10),
				CloudflareCredentialsSecretRef: SecretReference{
					Name: "test-secret",
				},
			},
			expected: true,
		},
		{
			name: "valid TXT record with comment and tags",
			spec: CloudflareRecordSpec{
				Zone:    "example.com",
				Type:    "TXT",
				Name:    "_verification.example.com",
				Content: "verification-token-12345",
				Comment: ptr.To("Domain verification record"),
				Tags:    []string{"verification", "security"},
				CloudflareCredentialsSecretRef: SecretReference{
					Name: "test-secret",
				},
			},
			expected: true,
		},
		{
			name: "minimal valid record",
			spec: CloudflareRecordSpec{
				Zone:    "example.com",
				Type:    "A",
				Name:    "test.example.com",
				Content: "1.2.3.4",
				CloudflareCredentialsSecretRef: SecretReference{
					Name: "test-secret",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the spec can be created without panicking
			record := &CloudflareRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-record",
					Namespace: "default",
				},
				Spec: tt.spec,
			}

			// Basic structural validation
			assert.NotEmpty(t, record.Spec.Zone, "Zone should not be empty")
			assert.NotEmpty(t, record.Spec.Type, "Type should not be empty")
			assert.NotEmpty(t, record.Spec.Name, "Name should not be empty")
			assert.NotEmpty(t, record.Spec.Content, "Content should not be empty")
			assert.NotEmpty(t, record.Spec.CloudflareCredentialsSecretRef.Name, "Secret ref name should not be empty")

			if tt.expected {
				// Verify optional fields work as expected
				if tt.spec.TTL != nil {
					assert.NotNil(t, record.Spec.TTL)
					assert.Greater(t, *record.Spec.TTL, 0)
				}

				if tt.spec.Priority != nil {
					assert.NotNil(t, record.Spec.Priority)
					assert.GreaterOrEqual(t, *record.Spec.Priority, 0)
				}

				if tt.spec.Proxied != nil {
					assert.NotNil(t, record.Spec.Proxied)
				}

				if tt.spec.Comment != nil {
					assert.NotNil(t, record.Spec.Comment)
					assert.NotEmpty(t, *record.Spec.Comment)
				}

				if len(tt.spec.Tags) > 0 {
					assert.NotEmpty(t, record.Spec.Tags)
				}
			}
		})
	}
}

func TestSecretReference_Validation(t *testing.T) {
	tests := []struct {
		name      string
		secretRef SecretReference
		expected  bool
	}{
		{
			name: "valid secret reference with name only",
			secretRef: SecretReference{
				Name: "test-secret",
			},
			expected: true,
		},
		{
			name: "valid secret reference with name and namespace",
			secretRef: SecretReference{
				Name:      "test-secret",
				Namespace: ptr.To("custom-namespace"),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.secretRef.Name, "Secret name should not be empty")

			if tt.secretRef.Namespace != nil {
				assert.NotEmpty(t, *tt.secretRef.Namespace, "If namespace is set, it should not be empty")
			}

			if tt.expected {
				// Additional validations for expected valid cases
				assert.True(t, len(tt.secretRef.Name) > 0)
			}
		})
	}
}

func TestCloudflareRecordStatus_Initialization(t *testing.T) {
	status := CloudflareRecordStatus{}

	// Test default values
	assert.False(t, status.Ready, "Status should default to not ready")
	assert.Nil(t, status.RecordID, "RecordID should be nil initially")
	assert.Nil(t, status.ZoneID, "ZoneID should be nil initially")
	assert.Nil(t, status.LastUpdated, "LastUpdated should be nil initially")
	assert.Empty(t, status.Conditions, "Conditions should be empty initially")
	assert.Equal(t, int64(0), status.ObservedGeneration, "ObservedGeneration should default to 0")
}

func TestCloudflareRecordStatus_WithValues(t *testing.T) {
	recordID := "test-record-id"
	zoneID := "test-zone-id"
	now := metav1.Now()

	status := CloudflareRecordStatus{
		RecordID:           &recordID,
		ZoneID:             &zoneID,
		Ready:              true,
		LastUpdated:        &now,
		ObservedGeneration: 1,
		Conditions: []metav1.Condition{
			{
				Type:               ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				Reason:             ConditionReasonRecordCreated,
				Message:            "DNS record created successfully",
				LastTransitionTime: now,
			},
		},
	}

	assert.True(t, status.Ready, "Status should be ready")
	assert.Equal(t, recordID, *status.RecordID, "RecordID should match")
	assert.Equal(t, zoneID, *status.ZoneID, "ZoneID should match")
	assert.NotNil(t, status.LastUpdated, "LastUpdated should not be nil")
	assert.Equal(t, int64(1), status.ObservedGeneration, "ObservedGeneration should be 1")
	assert.Len(t, status.Conditions, 1, "Should have one condition")
	assert.Equal(t, ConditionTypeReady, status.Conditions[0].Type, "Condition type should match")
	assert.Equal(t, metav1.ConditionTrue, status.Conditions[0].Status, "Condition status should be True")
}

func TestConditionConstants(t *testing.T) {
	// Test condition type constants
	assert.Equal(t, "Ready", ConditionTypeReady, "ConditionTypeReady should equal 'Ready'")
	assert.Equal(t, "Synced", ConditionTypeSynced, "ConditionTypeSynced should equal 'Synced'")

	// Test condition reason constants
	assert.Equal(t, "RecordCreated", ConditionReasonRecordCreated, "ConditionReasonRecordCreated should match")
	assert.Equal(t, "RecordUpdated", ConditionReasonRecordUpdated, "ConditionReasonRecordUpdated should match")
	assert.Equal(t, "RecordDeleted", ConditionReasonRecordDeleted, "ConditionReasonRecordDeleted should match")
	assert.Equal(t, "RecordError", ConditionReasonRecordError, "ConditionReasonRecordError should match")
	assert.Equal(t, "CredentialsError", ConditionReasonCredentialsError, "ConditionReasonCredentialsError should match")
	assert.Equal(t, "ZoneNotFound", ConditionReasonZoneNotFound, "ConditionReasonZoneNotFound should match")
}

func TestCloudflareRecord_Complete(t *testing.T) {
	record := &CloudflareRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complete-test-record",
			Namespace: "test-namespace",
		},
		Spec: CloudflareRecordSpec{
			Zone:     "example.com",
			Type:     "A",
			Name:     "complete.example.com",
			Content:  "10.0.0.1",
			TTL:      ptr.To(7200),
			Priority: ptr.To(5),
			Proxied:  ptr.To(false),
			Comment:  ptr.To("Test record for completeness"),
			Tags:     []string{"test", "complete", "example"},
			CloudflareCredentialsSecretRef: SecretReference{
				Name:      "complete-secret",
				Namespace: ptr.To("secret-namespace"),
			},
		},
		Status: CloudflareRecordStatus{
			RecordID:           ptr.To("complete-record-id"),
			ZoneID:             ptr.To("complete-zone-id"),
			Ready:              true,
			LastUpdated:        &metav1.Time{Time: metav1.Now().Time},
			ObservedGeneration: 2,
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             ConditionReasonRecordCreated,
					Message:            "Complete test record created",
					LastTransitionTime: metav1.Now(),
				},
				{
					Type:               ConditionTypeSynced,
					Status:             metav1.ConditionTrue,
					Reason:             ConditionReasonRecordUpdated,
					Message:            "Record synchronized with Cloudflare",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	// Verify all fields are accessible and correct
	assert.Equal(t, "complete-test-record", record.Name)
	assert.Equal(t, "test-namespace", record.Namespace)

	// Spec fields
	assert.Equal(t, "example.com", record.Spec.Zone)
	assert.Equal(t, "A", record.Spec.Type)
	assert.Equal(t, "complete.example.com", record.Spec.Name)
	assert.Equal(t, "10.0.0.1", record.Spec.Content)
	assert.Equal(t, 7200, *record.Spec.TTL)
	assert.Equal(t, 5, *record.Spec.Priority)
	assert.False(t, *record.Spec.Proxied)
	assert.Equal(t, "Test record for completeness", *record.Spec.Comment)
	assert.Contains(t, record.Spec.Tags, "test")
	assert.Contains(t, record.Spec.Tags, "complete")
	assert.Contains(t, record.Spec.Tags, "example")
	assert.Equal(t, "complete-secret", record.Spec.CloudflareCredentialsSecretRef.Name)
	assert.Equal(t, "secret-namespace", *record.Spec.CloudflareCredentialsSecretRef.Namespace)

	// Status fields
	assert.Equal(t, "complete-record-id", *record.Status.RecordID)
	assert.Equal(t, "complete-zone-id", *record.Status.ZoneID)
	assert.True(t, record.Status.Ready)
	assert.NotNil(t, record.Status.LastUpdated)
	assert.Equal(t, int64(2), record.Status.ObservedGeneration)
	assert.Len(t, record.Status.Conditions, 2)

	// Verify both conditions
	readyCondition := record.Status.Conditions[0]
	syncedCondition := record.Status.Conditions[1]

	assert.Equal(t, ConditionTypeReady, readyCondition.Type)
	assert.Equal(t, metav1.ConditionTrue, readyCondition.Status)
	assert.Equal(t, ConditionReasonRecordCreated, readyCondition.Reason)

	assert.Equal(t, ConditionTypeSynced, syncedCondition.Type)
	assert.Equal(t, metav1.ConditionTrue, syncedCondition.Status)
	assert.Equal(t, ConditionReasonRecordUpdated, syncedCondition.Reason)
}
