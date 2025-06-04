/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AlertTrackingFinalizer = "aegis.io/alert-tracking"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisAlert describe a alert event
type AegisAlert struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AegisAlertSpec defines the alert details.
	// +optional
	Spec AegisAlertSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// AegisAlertStatus defines the alert status.
	// +optional
	Status AegisAlertStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type AlertStatusType string

const (
	AlertStatusFiring   AlertStatusType = "Firing"
	AlertStatusResolved AlertStatusType = "Resovled"
)

// AegisAlertSpec defines the alert content.
type AegisAlertSpec struct {
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,1,rep,name=selector"`
	// Type is the category of alert
	Source         string            `json:"source,omitempty" protobuf:"bytes,2,rep,name=source"`
	Type           string            `json:"type,omitempty" protobuf:"bytes,3,rep,name=type"`
	Severity       string            `json:"severity,omitempty" protobuf:"bytes,4,rep,name=severity"`
	Status         AlertStatusType   `json:"status,omitempty" protobuf:"bytes,5,rep,name=status"`
	InvolvedObject AegisAlertObject  `json:"involvedObject,omitempty" protobuf:"bytes,6,rep,name=involvedObject"`
	Details        map[string]string `json:"detials,omitempty" protobuf:"bytes,7,rep,name=details"`

	// TTLStrategy limits the lifetime of a alert
	TTLStrategy *TTLStrategy `json:"ttlStrategy,omitempty" protobuf:"bytes,8,opt,name=ttlStrategy"`
}

type AlertObjectKind string

const (
	NodeKind AlertObjectKind = "Node"
	PodKind  AlertObjectKind = "Pod"
)

type AegisAlertObject struct {
	Kind      AlertObjectKind `json:"kind,omitempty" protobuf:"bytes,1,rep,name=kind"`
	Name      string          `json:"name,omitempty" protobuf:"bytes,2,rep,name=name"`
	Namespace string          `json:"namespace,omitempty" protobuf:"bytes,3,rep,name=namespace"`
	Node      string          `json:"node,omitempty" protobuf:"bytes,4,rep,name=node"`
}

type AlertOpsStatusType string
type AlertOpsTriggerStatusType string

const (
	OpsTriggerStatusRuleNotFound     AlertOpsTriggerStatusType = "RuleNotFound"
	OpsTriggerStatusRuleError        AlertOpsTriggerStatusType = "RuleErrorFound"
	OpsTriggerStatusRuleTooManyFound AlertOpsTriggerStatusType = "TooManyRuletFound"
	OpsTriggerStatusTemplateNotFound AlertOpsTriggerStatusType = "TemplateNotFound"
	OpsTriggerStatusTemplateInvalid  AlertOpsTriggerStatusType = "TemplateInvalid"
	OpsTriggerStatusTriggerFailed    AlertOpsTriggerStatusType = "TriggerFailed"
	OpsTriggerStatusTriggered        AlertOpsTriggerStatusType = "Triggered"
)

const (
	OpsStatusPending   AlertOpsStatusType = "Pending"
	OpsStatusRunning   AlertOpsStatusType = "Running"
	OpsStatusFailed    AlertOpsStatusType = "Failed"
	OpsStatusSucceeded AlertOpsStatusType = "Succeeded"
)

// TTLStrategy is the strategy for the time to live depending on if the workflow succeeded or failed
type TTLStrategy struct {
	// SecondsAfterCompletion is the number of seconds to live after completion
	SecondsAfterCompletion *int32 `json:"secondsAfterCompletion,omitempty" protobuf:"bytes,1,opt,name=secondsAfterCompletion"`
	// SecondsAfterSuccess is the number of seconds to live after success
	SecondsAfterSuccess *int32 `json:"secondsAfterSuccess,omitempty" protobuf:"bytes,2,opt,name=secondsAfterSuccess"`
	// SecondsAfterFailure is the number of seconds to live after failure
	SecondsAfterFailure *int32 `json:"secondsAfterFailure,omitempty" protobuf:"bytes,3,opt,name=secondsAfterFailure"`
	// SecondsAfterNoOps is the number of seconds to live with no ops
	SecondsAfterNoOps *int32 `json:"secondsAfterNoOps,omitempty" protobuf:"bytes,4,opt,name=secondsAfterNoOps""`
}

// AegisAlertStatus defines the alert/ops status.
type AegisAlertStatus struct {
	// The latest available observations of an object's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=type
	// +listType=atomic
	Conditions []AlertOpsCondition `json:"conditions,omitempty" protobuf:"bytes,1,rep,name=conditions"`

	// OpsStatus is the alert ops status.
	// +optional
	OpsStatus AegisAlertOpsStatus `json:"opsStatus,omitempty" protobuf:"bytes,2,rep,name=opsStatus"`

	// +optional
	Status string `json:"status,omitempty" protobuf:"bytes,3,rep,name=status"`

	// +optional
	Count int32 `json:"count,omitempty" protobuf:"bytes,4,rep,name=count"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,5,rep,name=startTime"`
}

// AegisAlertOpsStatus defines the corresponding ops status
type AegisAlertOpsStatus struct {

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,1,rep,name=startTime"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,2,rep,name=completionTime"`

	// +optional
	Status AlertOpsStatusType `json:"alertOpsStatus,omitempty" protobuf:"bytes,3,rep,name=alertOpsStatus"`

	// +optional
	TriggerStatus AlertOpsTriggerStatusType `json:"triggerStatus,omitempty" protobuf:"bytes,4,rep,name=triggerStatus"`

	// +optional
	Total *int32 `json:"total,omitempty" protobuf:"bytes,5,rep,name=total"`

	// +optional
	Active int32 `json:"active,omitempty" protobuf:"bytes,6,rep,name=active"`

	// +optional
	Succeeded int32 `json:"succeeded,omitempty" protobuf:"bytes,7,rep,name=succeeded"`

	// +optional
	Failed int32 `json:"failed,omitempty" protobuf:"bytes,8,rep,name=failed"`
}

type AlertOpsConditionType string

const (
	AlertSucceededCreateOpsWorkflow AlertOpsConditionType = "SucceededCreateWorkflow"
	AlertFailedCreateOpsWorkflow    AlertOpsConditionType = "FailedCreateWorkflow"
	AlertCompleteOpsWrofklow        AlertOpsConditionType = "Complete"
	AlertFailedOpsWrofklow          AlertOpsConditionType = "Failed"
)

type AlertOpsCondition struct {
	Type               AlertOpsConditionType  `json:"type,omitempty" protobuf:"bytes,1,rep,name=type"`
	Status             corev1.ConditionStatus `json:"status,omitempty" protobuf:"bytes,2,rep,name=status"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty" protobuf:"bytes,3,rep,name=lastProbeTime"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,rep,name=lastTransitionTime"`
	Reason             string                 `json:"reason,omitempty" protobuf:"bytes,5,rep,name=reason"`
	Message            string                 `json:"message,omitempty" protobuf:"bytes,6,rep,name=message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisAlertList is a list of AegisAlert items.
type AegisAlertList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AegisAlert objects.
	Items []AegisAlert `json:"items" protobuf:"bytes,2,rep,name=items"`
}
