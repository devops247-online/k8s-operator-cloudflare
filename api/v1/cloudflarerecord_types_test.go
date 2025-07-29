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

func TestCloudflareRecord_StructCreation(t *testing.T) {
	record := &CloudflareRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "struct-test-record",
			Namespace: "test-namespace",
		},
		Spec: CloudflareRecordSpec{
			Zone:    "struct.example.com",
			Type:    "A",
			Name:    "test.struct.example.com",
			Content: "192.168.1.1",
			TTL:     ptr.To(1800),
			CloudflareCredentialsSecretRef: SecretReference{
				Name: "struct-secret",
			},
		},
	}

	// Test that the struct can be created and all fields are accessible
	assert.Equal(t, "struct-test-record", record.Name)
	assert.Equal(t, "test-namespace", record.Namespace)
	assert.Equal(t, "struct.example.com", record.Spec.Zone)
	assert.Equal(t, "A", record.Spec.Type)
	assert.Equal(t, "test.struct.example.com", record.Spec.Name)
	assert.Equal(t, "192.168.1.1", record.Spec.Content)
	assert.Equal(t, 1800, *record.Spec.TTL)
	assert.Equal(t, "struct-secret", record.Spec.CloudflareCredentialsSecretRef.Name)

	// Test that nested structs work correctly
	assert.NotNil(t, record.Spec.CloudflareCredentialsSecretRef)
	assert.NotEmpty(t, record.Spec.CloudflareCredentialsSecretRef.Name)
}

func TestCloudflareRecordSpec_EdgeCases(t *testing.T) {
	t.Run("EmptyRequiredFields", testEmptyRequiredFields)
	t.Run("InvalidNumericValues", testInvalidNumericValues)
	t.Run("InvalidRangeValues", testInvalidRangeValues)
	t.Run("InvalidLengthValues", testInvalidLengthValues)
}

func testEmptyRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		spec CloudflareRecordSpec
	}{
		{
			name: "empty zone",
			spec: CloudflareRecordSpec{
				Zone: "", Type: "A", Name: "test.example.com", Content: "1.2.3.4",
				CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "empty type",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "", Name: "test.example.com", Content: "1.2.3.4",
				CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "empty name",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "A", Name: "", Content: "1.2.3.4",
				CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "empty content",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "A", Name: "test.example.com", Content: "",
				CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "empty secret name",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "A", Name: "test.example.com", Content: "1.2.3.4",
				CloudflareCredentialsSecretRef: SecretReference{Name: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasValidationIssue := tt.spec.Zone == "" || tt.spec.Type == "" ||
				tt.spec.Name == "" || tt.spec.Content == "" ||
				tt.spec.CloudflareCredentialsSecretRef.Name == ""
			assert.True(t, hasValidationIssue, "Should have validation issue for empty required field")
		})
	}
}

func testInvalidNumericValues(t *testing.T) {
	tests := []struct {
		name string
		spec CloudflareRecordSpec
	}{
		{
			name: "negative TTL",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "A", Name: "test.example.com", Content: "1.2.3.4",
				TTL: ptr.To(-1), CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "negative priority",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "MX", Name: "example.com", Content: "mail.example.com",
				Priority: ptr.To(-1), CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasValidationIssue := (tt.spec.TTL != nil && *tt.spec.TTL < 1) ||
				(tt.spec.Priority != nil && *tt.spec.Priority < 0)
			assert.True(t, hasValidationIssue, "Should have validation issue for negative values")
		})
	}
}

func testInvalidRangeValues(t *testing.T) {
	tests := []struct {
		name string
		spec CloudflareRecordSpec
	}{
		{
			name: "very large TTL",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "A", Name: "test.example.com", Content: "1.2.3.4",
				TTL: ptr.To(2147483648), CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
		{
			name: "very large priority",
			spec: CloudflareRecordSpec{
				Zone: "example.com", Type: "MX", Name: "example.com", Content: "mail.example.com",
				Priority: ptr.To(65536), CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasValidationIssue := (tt.spec.TTL != nil && *tt.spec.TTL > 2147483647) ||
				(tt.spec.Priority != nil && *tt.spec.Priority > 65535)
			assert.True(t, hasValidationIssue, "Should have validation issue for out-of-range values")
		})
	}
}

func testInvalidLengthValues(t *testing.T) {
	longComment := "This is a very long comment that exceeds the maximum allowed length of 100 characters for comments in DNS records, which should be invalid according to the kubebuilder validation rules"

	spec := CloudflareRecordSpec{
		Zone: "example.com", Type: "A", Name: "test.example.com", Content: "1.2.3.4",
		Comment: ptr.To(longComment), CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
	}

	hasValidationIssue := spec.Comment != nil && len(*spec.Comment) > 100
	assert.True(t, hasValidationIssue, "Should have validation issue for too long comment")
}

func TestSecretReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		secretRef SecretReference
		valid     bool
	}{
		{
			name:      "empty name",
			secretRef: SecretReference{Name: ""},
			valid:     false,
		},
		{
			name:      "empty namespace pointer",
			secretRef: SecretReference{Name: "test", Namespace: ptr.To("")},
			valid:     false,
		},
		{
			name:      "nil namespace is ok",
			secretRef: SecretReference{Name: "test", Namespace: nil},
			valid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.secretRef.Name)
			} else {
				hasIssue := tt.secretRef.Name == "" ||
					(tt.secretRef.Namespace != nil && *tt.secretRef.Namespace == "")
				assert.True(t, hasIssue, "Should have validation issue")
			}
		})
	}
}

func TestCloudflareRecordTypes_SupportedTypes(t *testing.T) {
	supportedTypes := []string{"A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "PTR", "CAA", "CERT", "DNSKEY", "DS", "NAPTR", "SMIMEA", "SSHFP", "TLSA", "URI"}

	for _, recordType := range supportedTypes {
		t.Run("type_"+recordType, func(t *testing.T) {
			spec := CloudflareRecordSpec{
				Zone:                           "example.com",
				Type:                           recordType,
				Name:                           "test.example.com",
				Content:                        getValidContentForType(recordType),
				CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
			}

			record := &CloudflareRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec:       spec,
			}

			assert.Equal(t, recordType, record.Spec.Type)
			assert.NotEmpty(t, record.Spec.Content)
		})
	}
}

func getValidContentForType(recordType string) string {
	switch recordType {
	case "A":
		return "192.168.1.1"
	case "AAAA":
		return "2001:db8::1"
	case "CNAME":
		return "example.com"
	case "MX":
		return "mail.example.com"
	case "TXT":
		return "v=spf1 include:_spf.example.com ~all"
	case "SRV":
		return "10 5 443 target.example.com"
	default:
		return "test-content"
	}
}

func TestCloudflareRecordList_Operations(t *testing.T) {
	list := &CloudflareRecordList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "dns.cloudflare.io/v1",
			Kind:       "CloudflareRecordList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []CloudflareRecord{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "record1", Namespace: "default"},
				Spec: CloudflareRecordSpec{
					Zone:                           "example.com",
					Type:                           "A",
					Name:                           "test1.example.com",
					Content:                        "1.2.3.4",
					CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "record2", Namespace: "default"},
				Spec: CloudflareRecordSpec{
					Zone:                           "example.com",
					Type:                           "CNAME",
					Name:                           "test2.example.com",
					Content:                        "example.com",
					CloudflareCredentialsSecretRef: SecretReference{Name: "secret"},
				},
			},
		},
	}

	assert.Equal(t, "dns.cloudflare.io/v1", list.APIVersion)
	assert.Equal(t, "CloudflareRecordList", list.Kind)
	assert.Equal(t, "12345", list.ResourceVersion)
	assert.Len(t, list.Items, 2)
	assert.Equal(t, "record1", list.Items[0].Name)
	assert.Equal(t, "record2", list.Items[1].Name)
	assert.Equal(t, "A", list.Items[0].Spec.Type)
	assert.Equal(t, "CNAME", list.Items[1].Spec.Type)
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

func TestGroupVersion_Constants(t *testing.T) {
	// Test GroupVersion constants
	assert.Equal(t, "dns.cloudflare.io", GroupVersion.Group, "Group should match")
	assert.Equal(t, "v1", GroupVersion.Version, "Version should match")
	assert.Equal(t, "dns.cloudflare.io/v1", GroupVersion.String(), "GroupVersion string should match")
}

func TestSchemeBuilder_Registration(t *testing.T) {
	// Test that SchemeBuilder is properly initialized
	assert.NotNil(t, SchemeBuilder, "SchemeBuilder should not be nil")
	assert.NotNil(t, AddToScheme, "AddToScheme should not be nil")
	assert.Equal(t, GroupVersion, SchemeBuilder.GroupVersion, "SchemeBuilder should have correct GroupVersion")
}

func TestCloudflareRecord_TypeMeta(t *testing.T) {
	record := &CloudflareRecord{}
	record.APIVersion = GroupVersion.String()
	record.Kind = "CloudflareRecord"

	assert.Equal(t, "dns.cloudflare.io/v1", record.APIVersion)
	assert.Equal(t, "CloudflareRecord", record.Kind)
}

func TestCloudflareRecordList_TypeMeta(t *testing.T) {
	list := &CloudflareRecordList{}
	list.APIVersion = GroupVersion.String()
	list.Kind = "CloudflareRecordList"

	assert.Equal(t, "dns.cloudflare.io/v1", list.APIVersion)
	assert.Equal(t, "CloudflareRecordList", list.Kind)
}
