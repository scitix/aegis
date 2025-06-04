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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisNodeHealthCheck describe a check
type AegisNodeHealthCheck struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AegisNodeHealthCheckSpec defines the check spec.
	// +optional
	Spec AegisNodeHealthCheckSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// AegisNodeHealthCheckStatus defines the template status.
	// +optional
	Status AegisNodeHealthCheckStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// AegisNodeHealthCheckSpec defines the check spec.
type AegisNodeHealthCheckSpec struct {
	// +optional
	//Schedule string `json:"schedule,omitempty" protobuf:"bytes,1,rep,name=schedule"`

	// +optional
	// Timeout time.Duration `json:"timeout,omitempty" protobuf:"bytes,1,rep,name=timeout"`

	// +optional
	Node string `json:"node,omitempty" protobuf:"bytes,1,rep,name=node"`

	// +optional
	RuleConfigmapSelector metav1.LabelSelector `json:"ruleConfigmapSelector,omitempty" protobuf:"bytes,2,opt,name=ruleConfigmapSelector"`

	// +optional
	Template corev1.PodTemplateSpec `json:"template,omitempty" protobuf:"bytes,3,opt,name=template"`
}

// type EventType string

// const (
// 	Permanent EventType = "permanent"
// 	Temporary EventType = "temporary"
// )

// type AegisNodeEventRule struct {
// 	Type      EventType `json:"type,omitempty" protobuf:"bytes,1,rep,name=type"`
// 	Reason    string    `json:"reason,omitempty" protobuf:"bytes,2,rep,name=reason"`
// 	Condition string    `json:"condition,omitempty" protobuf:"bytes,3,rep,name=condition"`
// }

type ResourceInfo struct {
	// +optional
	Item string `json:"item,omitempty" protobuf:"bytes,1,rep,name=item"`

	// +optional
	Condition string `json:"condition,omitempty" protobuf:"bytes,2,rep,name=condition"`

	// +optional
	Level string `json:"level,omitempty" protobuf:"bytes,3,rep,name=level"`

	// +optional
	Status bool `json:"status,omitempty" protobuf:"bytes,4,rep,name=status"`

	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,5,rep,name=message"`
}

type ResourceInfos []ResourceInfo

type ResultInfos map[string]ResourceInfos

// AegisNodeHealthCheckStatus defines the check status.
type AegisNodeHealthCheckStatus struct {
	// The latest available observations of an object's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=type
	// +listType=atomic
	Conditions []CheckCondition `json:"conditions,omitempty" protobuf:"bytes,1,rep,name=conditions"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,2,rep,name=startTime"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,3,rep,name=completionTime"`

	// Status is the check status.
	// +optional
	Status CheckStatusType `json:"status,omitempty" protobuf:"bytes,4,rep,name=status"`

	// +optional
	Results ResultInfos `json:"results,omitempty" protobuf:"bytes,5,rep,name=results"`
}

type CheckConditionType string

const (
	CheckSucceededCreatePod CheckConditionType = "SucceededCreatePod"
	CheckFailedCreatePod    CheckConditionType = "FailedCreatePod"
	Checking                CheckConditionType = "Checking"
	CheckSucceed            CheckConditionType = "Succeed"
	CheckFailed             CheckConditionType = "Failed"
	CheckFailedFetchResult  CheckConditionType = "FailedFetchResult"
)

type CheckCondition struct {
	Type               CheckConditionType     `json:"type,omitempty" protobuf:"bytes,1,rep,name=type"`
	Status             corev1.ConditionStatus `json:"status,omitempty" protobuf:"bytes,2,rep,name=status"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty" protobuf:"bytes,3,rep,name=lastProbeTime"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,rep,name=lastTransitionTime"`
	Reason             string                 `json:"reason,omitempty" protobuf:"bytes,5,rep,name=reason"`
	Message            string                 `json:"message,omitempty" protobuf:"bytes,6,rep,name=message"`
}

type CheckStatusType string

const (
	CheckStatusUnknown   CheckStatusType = "Unknown"
	CheckStatusPending   CheckStatusType = "Pending"
	CheckStatusRunning   CheckStatusType = "Checking"
	CheckStatusFailed    CheckStatusType = "Failed"
	CheckStatusSucceeded CheckStatusType = "Succeeded"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisNodeHealthCheckList is a list of AegisNodeHealthCheck items.
type AegisNodeHealthCheckList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AegisNodeHealthCheck objects.
	Items []AegisNodeHealthCheck `json:"items" protobuf:"bytes,2,rep,name=items"`
}
