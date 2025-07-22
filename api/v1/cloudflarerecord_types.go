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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for CloudflareRecord
const (
	// ConditionTypeReady indicates that the CloudflareRecord is ready
	ConditionTypeReady = "Ready"
	// ConditionTypeSynced indicates that the CloudflareRecord is synced with Cloudflare
	ConditionTypeSynced = "Synced"
)

// Condition reasons for CloudflareRecord
const (
	// ConditionReasonRecordCreated indicates that the DNS record was created
	ConditionReasonRecordCreated = "RecordCreated"
	// ConditionReasonRecordUpdated indicates that the DNS record was updated
	ConditionReasonRecordUpdated = "RecordUpdated"
	// ConditionReasonRecordDeleted indicates that the DNS record was deleted
	ConditionReasonRecordDeleted = "RecordDeleted"
	// ConditionReasonRecordError indicates an error occurred while managing the DNS record
	ConditionReasonRecordError = "RecordError"
	// ConditionReasonCredentialsError indicates an error with Cloudflare credentials
	ConditionReasonCredentialsError = "CredentialsError"
	// ConditionReasonZoneNotFound indicates the specified zone was not found
	ConditionReasonZoneNotFound = "ZoneNotFound"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CloudflareRecordSpec defines the desired state of CloudflareRecord
type CloudflareRecordSpec struct {
	// Zone is the Cloudflare zone name (e.g., example.com)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone"`

	// Type is the DNS record type (A, AAAA, CNAME, MX, TXT, etc.)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=A;AAAA;CNAME;MX;TXT;SRV;NS;PTR;CAA;CERT;DNSKEY;DS;NAPTR;SMIMEA;SSHFP;TLSA;URI
	Type string `json:"type"`

	// Name is the DNS record name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Content is the content of the DNS record
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// TTL is the time to live for the DNS record in seconds
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2147483647
	// +kubebuilder:default=3600
	TTL *int `json:"ttl,omitempty"`

	// Priority is used for MX, SRV, and other records that support priority
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	Priority *int `json:"priority,omitempty"`

	// Proxied indicates whether the record should be proxied through Cloudflare
	// Only applicable to A, AAAA, and CNAME records
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Proxied *bool `json:"proxied,omitempty"`

	// Comment is an optional comment for the DNS record
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=100
	Comment *string `json:"comment,omitempty"`

	// Tags are optional tags for the DNS record
	// +kubebuilder:validation:Optional
	Tags []string `json:"tags,omitempty"`

	// CloudflareCredentialsSecretRef references a Secret containing Cloudflare credentials
	// +kubebuilder:validation:Required
	CloudflareCredentialsSecretRef SecretReference `json:"cloudflareCredentialsSecretRef"`
}

// SecretReference represents a reference to a Secret
type SecretReference struct {
	// Name is the name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the secret
	// +kubebuilder:validation:Optional
	Namespace *string `json:"namespace,omitempty"`
}

// CloudflareRecordStatus defines the observed state of CloudflareRecord.
type CloudflareRecordStatus struct {
	// RecordID is the Cloudflare record ID
	// +kubebuilder:validation:Optional
	RecordID *string `json:"recordId,omitempty"`

	// ZoneID is the Cloudflare zone ID
	// +kubebuilder:validation:Optional
	ZoneID *string `json:"zoneId,omitempty"`

	// Ready indicates whether the DNS record is ready and synchronized
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// LastUpdated is the timestamp of the last update
	// +kubebuilder:validation:Optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Conditions represent the latest available observations of the CloudflareRecord's current state
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed CloudflareRecord
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=cloudflare,shortName=cfr
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Content",type=string,JSONPath=`.spec.content`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CloudflareRecord is the Schema for the cloudflarerecords API
type CloudflareRecord struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of CloudflareRecord
	// +required
	Spec CloudflareRecordSpec `json:"spec"`

	// status defines the observed state of CloudflareRecord
	// +optional
	Status CloudflareRecordStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// CloudflareRecordList contains a list of CloudflareRecord
type CloudflareRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudflareRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudflareRecord{}, &CloudflareRecordList{})
}
