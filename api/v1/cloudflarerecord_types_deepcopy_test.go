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
)

func TestCloudflareRecordDeepCopy(t *testing.T) {
	t.Run("DeepCopy creates independent copy", func(t *testing.T) {
		original := &CloudflareRecord{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dns.cloudflare.io/v1",
				Kind:       "CloudflareRecord",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-record",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				Annotations: map[string]string{
					"description": "test record",
				},
			},
			Spec: CloudflareRecordSpec{
				Type:    "A",
				Name:    "test.example.com",
				Content: "192.168.1.1",
				TTL:     intPtr(300),
				Comment: stringPtr("Test record"),
			},
			Status: CloudflareRecordStatus{
				RecordID: stringPtr("test-id-123"),
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "RecordCreated",
					},
				},
			},
		}

		// Create deep copy
		copied := original.DeepCopy()

		// Verify it's a different object
		assert.NotSame(t, original, copied, "DeepCopy should create a new object")

		// Verify all fields are equal
		assert.Equal(t, original.TypeMeta, copied.TypeMeta)
		assert.Equal(t, original.Name, copied.Name)
		assert.Equal(t, original.Namespace, copied.Namespace)
		assert.Equal(t, original.Spec, copied.Spec)
		assert.Equal(t, original.Status, copied.Status)

		// Verify maps and slices are deep copied (not shared) - skip pointer checks as they may be the same
		// The important thing is that modifications don't affect each other

		// Modify original and ensure copy is not affected
		original.Spec.Content = "10.0.0.1"
		original.Labels["modified"] = "true"
		original.Status.Conditions[0].Reason = "Modified"

		assert.NotEqual(t, original.Spec.Content, copied.Spec.Content)
		assert.NotContains(t, copied.Labels, "modified")
		assert.NotEqual(t, original.Status.Conditions[0].Reason, copied.Status.Conditions[0].Reason)
	})

	t.Run("DeepCopyObject returns correct type", func(t *testing.T) {
		original := &CloudflareRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-record",
			},
			Spec: CloudflareRecordSpec{
				Type: "CNAME",
				Name: "alias.example.com",
			},
		}

		// Test DeepCopyObject
		copiedObj := original.DeepCopyObject()
		assert.NotNil(t, copiedObj)

		// Should be able to cast back to CloudflareRecord
		copiedRecord, ok := copiedObj.(*CloudflareRecord)
		assert.True(t, ok, "DeepCopyObject should return CloudflareRecord type")
		assert.Equal(t, original.Name, copiedRecord.Name)
		assert.Equal(t, original.Spec.Type, copiedRecord.Spec.Type)
		assert.Equal(t, original.Spec.Name, copiedRecord.Spec.Name)

		// Should satisfy runtime.Object interface
		runtimeObj := copiedObj
		assert.NotNil(t, runtimeObj)
	})

	t.Run("DeepCopy with nil record", func(t *testing.T) {
		var original *CloudflareRecord = nil
		copied := original.DeepCopy()
		assert.Nil(t, copied, "DeepCopy of nil should return nil")
	})
}

func TestCloudflareRecordSpecDeepCopy(t *testing.T) {
	t.Run("DeepCopy creates independent spec copy", func(t *testing.T) {
		original := &CloudflareRecordSpec{
			Type:     "MX",
			Name:     "mail.example.com",
			Content:  "mail.server.com",
			TTL:      intPtr(3600),
			Priority: intPtr(10),
			Comment:  stringPtr("Mail server record"),
		}

		copied := original.DeepCopy()

		// Verify it's a different object
		assert.NotSame(t, original, copied)

		// Verify all fields are equal
		assert.Equal(t, original.Type, copied.Type)
		assert.Equal(t, original.Name, copied.Name)
		assert.Equal(t, original.Content, copied.Content)
		assert.Equal(t, original.TTL, copied.TTL)
		assert.Equal(t, original.Comment, copied.Comment)

		// Verify pointer fields are deep copied
		if original.Priority != nil && copied.Priority != nil {
			assert.NotSame(t, original.Priority, copied.Priority)
			assert.Equal(t, *original.Priority, *copied.Priority)
		}

		// Modify original and ensure copy is not affected
		original.Content = "new.mail.server.com"
		if original.Priority != nil {
			*original.Priority = 20
		}

		assert.NotEqual(t, original.Content, copied.Content)
		if copied.Priority != nil {
			assert.Equal(t, 10, *copied.Priority)
		}
	})

	t.Run("DeepCopy with nil spec", func(t *testing.T) {
		var original *CloudflareRecordSpec = nil
		copied := original.DeepCopy()
		assert.Nil(t, copied, "DeepCopy of nil spec should return nil")
	})
}

func TestCloudflareRecordStatusDeepCopy(t *testing.T) {
	t.Run("DeepCopy creates independent status copy", func(t *testing.T) {
		original := &CloudflareRecordStatus{
			RecordID: stringPtr("record-123"),
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "RecordSynced",
					Message:            "Record successfully synced",
				},
				{
					Type:               "Error",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
					Reason:             "NoError",
					Message:            "No errors detected",
				},
			},
		}

		copied := original.DeepCopy()

		// Verify it's a different object
		assert.NotSame(t, original, copied)

		// Verify all fields are equal
		if original.RecordID != nil && copied.RecordID != nil {
			assert.Equal(t, *original.RecordID, *copied.RecordID)
		}
		assert.Equal(t, len(original.Conditions), len(copied.Conditions))

		// Verify conditions slice is deep copied - skip pointer check
		for i := range original.Conditions {
			assert.Equal(t, original.Conditions[i].Type, copied.Conditions[i].Type)
			assert.Equal(t, original.Conditions[i].Status, copied.Conditions[i].Status)
			assert.Equal(t, original.Conditions[i].Reason, copied.Conditions[i].Reason)
			assert.Equal(t, original.Conditions[i].Message, copied.Conditions[i].Message)
		}

		// Modify original and ensure copy is not affected
		original.RecordID = stringPtr("modified-123")
		original.Conditions[0].Reason = "ModifiedReason"
		original.Conditions = append(original.Conditions, metav1.Condition{
			Type:   "NewCondition",
			Status: metav1.ConditionTrue,
		})

		if original.RecordID != nil && copied.RecordID != nil {
			assert.NotEqual(t, *original.RecordID, *copied.RecordID)
		}
		assert.NotEqual(t, original.Conditions[0].Reason, copied.Conditions[0].Reason)
		assert.NotEqual(t, len(original.Conditions), len(copied.Conditions))
	})

	t.Run("DeepCopy with nil status", func(t *testing.T) {
		var original *CloudflareRecordStatus = nil
		copied := original.DeepCopy()
		assert.Nil(t, copied, "DeepCopy of nil status should return nil")
	})
}

func TestCloudflareRecordListDeepCopy(t *testing.T) {
	t.Run("DeepCopy creates independent list copy", func(t *testing.T) {
		original := &CloudflareRecordList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dns.cloudflare.io/v1",
				Kind:       "CloudflareRecordList",
			},
			ListMeta: metav1.ListMeta{
				ResourceVersion: "12345",
			},
			Items: []CloudflareRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "record1",
					},
					Spec: CloudflareRecordSpec{
						Type: "A",
						Name: "test1.example.com",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "record2",
					},
					Spec: CloudflareRecordSpec{
						Type: "CNAME",
						Name: "test2.example.com",
					},
				},
			},
		}

		copied := original.DeepCopy()

		// Verify it's a different object
		assert.NotSame(t, original, copied)

		// Verify metadata is equal
		assert.Equal(t, original.TypeMeta, copied.TypeMeta)
		assert.Equal(t, original.ListMeta, copied.ListMeta)

		// Verify items slice is deep copied - skip pointer check
		assert.Equal(t, len(original.Items), len(copied.Items))

		for i := range original.Items {
			assert.Equal(t, original.Items[i].Name, copied.Items[i].Name)
			assert.Equal(t, original.Items[i].Spec.Type, copied.Items[i].Spec.Type)
			assert.Equal(t, original.Items[i].Spec.Name, copied.Items[i].Spec.Name)
		}

		// Modify original and ensure copy is not affected
		original.Items[0].Spec.Content = "modified"
		original.Items = append(original.Items, CloudflareRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "record3"},
		})

		assert.NotEqual(t, original.Items[0].Spec.Content, copied.Items[0].Spec.Content)
		assert.NotEqual(t, len(original.Items), len(copied.Items))
	})

	t.Run("DeepCopyObject returns correct type", func(t *testing.T) {
		original := &CloudflareRecordList{
			Items: []CloudflareRecord{
				{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			},
		}

		copiedObj := original.DeepCopyObject()
		assert.NotNil(t, copiedObj)

		// Should be able to cast back to CloudflareRecordList
		copiedList, ok := copiedObj.(*CloudflareRecordList)
		assert.True(t, ok, "DeepCopyObject should return CloudflareRecordList type")
		assert.Equal(t, len(original.Items), len(copiedList.Items))
	})
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
